package importers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

// The v3 JSONL session format is shared by Pi and its OMO sibling. Both write one
// `message` record per assistant turn with a `usage` block, so one parser serves
// both CLIs.

// PiImporter imports from Pi agent session files (~/.pi/agent/sessions).
type PiImporter struct{ paths Paths }

func (p *PiImporter) Name() string { return "pi" }
func (p *PiImporter) root() string {
	return p.paths.Resolve("pi", filepath.Join(homeDir(), ".pi", "agent", "sessions"))
}
func (p *PiImporter) Detect() bool { return detectSessionDir(p.root()) }
func (p *PiImporter) Scan(since time.Time) domain.ScanResult {
	return scanSessionDir("pi", p.root(), since)
}
func (p *PiImporter) IterUsages(since time.Time, out chan<- domain.RawUsage) {
	defer close(out)
	walkAgentSessions("pi", p.root(), since, out)
}

// OmoImporter imports from OMO sessions (~/.omo/agent/sessions). OMO is the
// successor of the OMP harness and writes the same v3 JSONL records: usage lives
// on `message` entries with role=assistant, not in `model_usage` records.
type OmoImporter struct{ paths Paths }

func (o *OmoImporter) Name() string { return "omo" }
func (o *OmoImporter) root() string {
	return o.paths.Resolve("omo", filepath.Join(homeDir(), ".omo", "agent", "sessions"))
}
func (o *OmoImporter) Detect() bool { return detectSessionDir(o.root()) }
func (o *OmoImporter) Scan(since time.Time) domain.ScanResult {
	return scanSessionDir("omo", o.root(), since)
}
func (o *OmoImporter) IterUsages(since time.Time, out chan<- domain.RawUsage) {
	defer close(out)
	walkAgentSessions("omo", o.root(), since, out)
}

// detectSessionDir reports whether dir holds at least one JSONL session file.
// Importers used to accept any directory, so an empty data root looked ready.
func detectSessionDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	found := false
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".jsonl") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// scanSessionDir is the discovery half shared by every agent-session CLI.
func scanSessionDir(cli, dir string, since time.Time) domain.ScanResult {
	r := domain.ScanResult{CLI: cli, Path: dir}
	if !detectSessionDir(dir) {
		return r
	}
	count, first, last := scanFiles(dir, ".jsonl", since)
	r.Available = true
	r.SessionFiles = count
	r.FirstModified, r.LastModified = first, last
	return r
}

func walkAgentSessions(cli, dir string, since time.Time, out chan<- domain.RawUsage) {
	if _, err := os.Stat(dir); err != nil {
		return
	}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		readAgentSession(cli, path, since, out)
		return nil
	})
}

func readAgentSession(cli, path string, since time.Time, out chan<- domain.RawUsage) {
	base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	sessionID := base
	if idx := strings.LastIndex(base, "_"); idx >= 0 {
		sessionID = base[idx+1:]
	}
	project := piProjectFromDir(filepath.Dir(path))

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	idx := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if json.Unmarshal([]byte(line), &obj) != nil || obj["type"] != "message" {
			continue
		}
		msg, _ := obj["message"].(map[string]interface{})
		if msg == nil || msg["role"] != "assistant" {
			continue
		}
		usage, _ := msg["usage"].(map[string]interface{})
		if usage == nil {
			continue
		}
		ts, _ := obj["timestamp"].(string)
		if beforeSince(ts, since) {
			continue
		}

		// OMO omits responseId on some providers; the record id is still stable
		// across reimports, which is what the dedupe index needs.
		eid := strOr(msg, "responseId", "")
		if eid == "" {
			eid = strOr(obj, "id", "")
		}
		if eid == "" {
			eid = fmt.Sprintf("%s:%d", path, idx)
		}
		rawCost := 0.0
		if costMap, ok := usage["cost"].(map[string]interface{}); ok {
			if total, ok := costMap["total"].(float64); ok {
				rawCost = total
			}
		}

		out <- domain.RawUsage{
			CLI:              cli,
			SessionID:        sessionID,
			SessionFile:      path,
			EntryID:          eid,
			Timestamp:        ts,
			Provider:         strOr(msg, "provider", "unknown"),
			Model:            strOr(msg, "model", "unknown"),
			PromptTokens:     toInt(usage, "input"),
			CompletionTokens: toInt(usage, "output"),
			CacheReadTokens:  toInt(usage, "cacheRead"),
			CacheWriteTokens: toInt(usage, "cacheWrite"),
			Project:          project,
			RawCost:          rawCost,
		}
		idx++
	}
}
