package importers

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

// ClaudeImporter imports from Claude CLI session files (~/.claude/projects/<hash>/<session>.jsonl).
type ClaudeImporter struct{ paths Paths }

func (c *ClaudeImporter) Name() string { return "claude" }

// root is the CLI data root, honoring sessions.paths.
func (c *ClaudeImporter) root() string {
	return c.paths.Resolve("claude", filepath.Join(homeDir(), ".claude", "projects"))
}

func (c *ClaudeImporter) Detect() bool {
	p := c.root()
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return false
	}
	entries, _ := os.ReadDir(p)
	return len(entries) > 0
}

func (c *ClaudeImporter) Scan(since time.Time) domain.ScanResult {
	dir := c.root()
	r := domain.ScanResult{CLI: "claude", Path: dir}
	if !c.Detect() {
		return r
	}
	count, first, last := scanFiles(dir, ".jsonl", since)
	r.Available = true
	r.SessionFiles = count
	r.FirstModified, r.LastModified = first, last
	return r
}

func (c *ClaudeImporter) IterUsages(since time.Time, out chan<- domain.RawUsage) {
	defer close(out)
	dir := c.root()
	if !c.Detect() {
		return
	}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		projectHash := filepath.Base(filepath.Dir(path))
		project := projectHash
		if strings.HasPrefix(projectHash, "-") {
			project = strings.ReplaceAll(projectHash, "-", "/")
		}
		sessionID := strings.TrimSuffix(info.Name(), ".jsonl")
		f, ferr := os.Open(path)
		if ferr != nil {
			return nil
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
			if json.Unmarshal([]byte(line), &obj) != nil {
				continue
			}
			if obj["type"] != "assistant" {
				continue
			}
			msg, _ := obj["message"].(map[string]interface{})
			if msg == nil {
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
			model := strOr(msg, "model", strOr(obj, "model", "unknown"))
			entryID := strOr(obj, "uuid", "")
			if entryID == "" {
				entryID = fmt.Sprintf("%s:%d", sessionID, idx)
			}
			cacheWrite := toInt(usage, "cache_creation_input_tokens")
			if cacheWrite == 0 {
				if cc, ok := usage["cache_creation"].(map[string]interface{}); ok {
					cacheWrite = toInt(cc, "ephemeral_5m_input_tokens") + toInt(cc, "ephemeral_1h_input_tokens")
				}
			}
			out <- domain.RawUsage{
				CLI:              "claude",
				SessionID:        sessionID,
				SessionFile:      path,
				EntryID:          entryID,
				Timestamp:        ts,
				Provider:         "anthropic",
				Model:            model,
				PromptTokens:     toInt(usage, "input_tokens"),
				CompletionTokens: toInt(usage, "output_tokens"),
				CacheReadTokens:  toInt(usage, "cache_read_input_tokens"),
				CacheWriteTokens: cacheWrite,
				Project:          project,
			}
			idx++
		}
		return nil
	})
}

// CodexImporter imports from Codex CLI session files (~/.codex/sessions/**/*.jsonl).
type CodexImporter struct{ paths Paths }

func (c *CodexImporter) Name() string { return "codex" }

// root is the CLI data root, honoring sessions.paths.
func (c *CodexImporter) root() string {
	return c.paths.Resolve("codex", filepath.Join(homeDir(), ".codex", "sessions"))
}

func (c *CodexImporter) Detect() bool {
	p := c.root()
	_, err := os.Stat(p)
	return err == nil
}

func (c *CodexImporter) Scan(since time.Time) domain.ScanResult {
	dir := c.root()
	r := domain.ScanResult{CLI: "codex", Path: dir}
	if _, err := os.Stat(dir); err != nil {
		return r
	}
	count, first, last := scanFiles(dir, ".jsonl", since)
	r.Available = true
	r.SessionFiles = count
	r.FirstModified, r.LastModified = first, last
	return r
}

func (c *CodexImporter) IterUsages(since time.Time, out chan<- domain.RawUsage) {
	defer close(out)
	dir := c.root()
	if _, err := os.Stat(dir); err != nil {
		return
	}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		sessionID := strings.TrimSuffix(info.Name(), ".jsonl")
		providerHint := ""
		modelHint := ""
		cwdHint := ""
		f, ferr := os.Open(path)
		if ferr != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var obj map[string]interface{}
			if json.Unmarshal([]byte(line), &obj) != nil {
				continue
			}
			t, _ := obj["type"].(string)
			payload, _ := obj["payload"].(map[string]interface{})
			switch t {
			case "session_meta":
				if payload != nil {
					if v, ok := payload["model_provider"].(string); ok {
						providerHint = v
					}
					if v, ok := payload["cwd"].(string); ok {
						cwdHint = v
					}
				}
			case "event_msg":
				if payload != nil {
					if ts, ok := payload["thread_settings"].(map[string]interface{}); ok {
						if v, ok := ts["model"].(string); ok {
							modelHint = v
						}
						if v, ok := ts["model_provider_id"].(string); ok {
							providerHint = v
						}
					}
				}
			case "token_usage_record":
				if payload == nil {
					continue
				}
				usage, _ := payload["usage"].(map[string]interface{})
				if usage == nil {
					continue
				}
				ts, _ := obj["timestamp"].(string)
				if beforeSince(ts, since) {
					continue
				}
				model := strOr(payload, "model", firstNonEmpty(modelHint, providerHint, "unknown"))
				provider := firstNonEmpty(providerHint, "openai")
				project := firstNonEmpty(cwdHint, strOr(payload, "cwd", ""), "default")
				sid := strOr(payload, "session_id", sessionID)
				eid := strOr(payload, "response_id", "")
				if eid == "" {
					eid = strOr(obj, "timestamp", sessionID+":0")
				}
				out <- domain.RawUsage{
					CLI:              "codex",
					SessionID:        sid,
					SessionFile:      path,
					EntryID:          eid,
					Timestamp:        ts,
					Provider:         provider,
					Model:            model,
					PromptTokens:     toInt(usage, "input_tokens"),
					CompletionTokens: toInt(usage, "output_tokens"),
					CacheReadTokens:  toInt(usage, "cached_input_tokens"),
					CacheWriteTokens: toInt(usage, "cache_write_input_tokens"),
					Project:          project,
				}
			}
		}
		return nil
	})
}

// OmpImporter imports from OMP session JSONL files (~/.omp/agent/sessions/**/*.jsonl).
type OmpImporter struct{ paths Paths }

func (o *OmpImporter) Name() string { return "omp" }

// root is the CLI data root, honoring sessions.paths.
func (o *OmpImporter) root() string {
	return o.paths.Resolve("omp", filepath.Join(homeDir(), ".omp", "agent", "sessions"))
}

func (o *OmpImporter) Detect() bool {
	p := o.root()
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func (o *OmpImporter) Scan(since time.Time) domain.ScanResult {
	sessionsDir := o.root()
	if !o.Detect() {
		return domain.ScanResult{CLI: "omp", Available: false, Path: sessionsDir}
	}

	var totalFiles int
	var first, last time.Time
	filepath.WalkDir(sessionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if info, statErr := d.Info(); statErr == nil {
			if !since.IsZero() && info.ModTime().Before(since) {
				return nil
			}
			mt := info.ModTime()
			if first.IsZero() || mt.Before(first) {
				first = mt
			}
			if mt.After(last) {
				last = mt
			}
		}
		totalFiles++
		return nil
	})

	return domain.ScanResult{
		CLI:           "omp",
		Available:     true,
		SessionFiles:  totalFiles,
		Path:          sessionsDir,
		FirstModified: first,
		LastModified:  last,
	}
}

func (o *OmpImporter) IterUsages(since time.Time, out chan<- domain.RawUsage) {
	defer close(out)
	sessionsDir := o.root()
	if !o.Detect() {
		return
	}

	filepath.WalkDir(sessionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Extract session ID from filename
		sessionID := filepath.Base(path)
		if ext := filepath.Ext(sessionID); ext != "" {
			sessionID = sessionID[:len(sessionID)-len(ext)]
		}

		// Extract project from directory structure
		relPath, _ := filepath.Rel(sessionsDir, path)
		project := filepath.Dir(relPath)
		if project == "." {
			project = "default"
		}
		// Decode project name from directory encoding
		project = strings.ReplaceAll(project, "--", "/")
		project = strings.TrimPrefix(project, "-")
		project = strings.TrimSuffix(project, "-")

		entryIdx := 0
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, `"type":"model_usage"`) {
				continue
			}

			var event map[string]interface{}
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				continue
			}

			tsStr, _ := event["timestamp"].(string)
			if tsStr == "" {
				continue
			}

			ts, err := time.Parse(time.RFC3339Nano, tsStr)
			if err != nil {
				ts, err = time.Parse(time.RFC3339, tsStr)
				if err != nil {
					continue
				}
			}

			if !since.IsZero() && ts.Before(since) {
				continue
			}

			usage, ok := event["usage"].(map[string]interface{})
			if !ok {
				continue
			}

			inputTokens := toInt(usage, "input")
			outputTokens := toInt(usage, "output")
			cacheRead := toInt(usage, "cacheRead")
			cacheWrite := toInt(usage, "cacheWrite")

			costMap, _ := usage["cost"].(map[string]interface{})
			var costTotal float64
			if costMap != nil {
				if v, ok := costMap["total"].(float64); ok {
					costTotal = v
				}
			}

			provider, _ := event["provider"].(string)
			model, _ := event["model"].(string)
			entryID, _ := event["id"].(string)
			if entryID == "" {
				entryID = fmt.Sprintf("%s:%d", sessionID, entryIdx)
			}

			out <- domain.RawUsage{
				CLI:              "omp",
				SessionID:        sessionID,
				SessionFile:      path,
				EntryID:          entryID,
				Timestamp:        ts.UTC().Format(time.RFC3339),
				Provider:         provider,
				Model:            model,
				PromptTokens:     inputTokens,
				CompletionTokens: outputTokens,
				CacheReadTokens:  cacheRead,
				CacheWriteTokens: cacheWrite,
				Project:          project,
				RawCost:          costTotal,
			}
			entryIdx++
		}
		return nil
	})
}

// CrushImporter imports from Crush session files resolved via projects.json.
type CrushImporter struct{ paths Paths }

func (c *CrushImporter) Name() string { return "crush" }

// root is the CLI data root, honoring sessions.paths.
func (c *CrushImporter) root() string {
	return c.paths.Resolve("crush", filepath.Join(homeDir(), ".local", "share", "crush", "projects.json"))
}

func (c *CrushImporter) Detect() bool {
	p := c.root()
	_, err := os.Stat(p)
	return err == nil
}

func (c *CrushImporter) crushDataDirs() []string {
	p := c.root()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var pj struct {
		Projects []struct {
			DataDir string `json:"data_dir"`
		} `json:"projects"`
	}
	if json.Unmarshal(data, &pj) != nil {
		return nil
	}
	var dirs []string
	for _, p := range pj.Projects {
		if p.DataDir != "" {
			dirs = append(dirs, p.DataDir)
		}
	}
	return dirs
}

func (c *CrushImporter) Scan(since time.Time) domain.ScanResult {
	pjPath := c.root()
	r := domain.ScanResult{CLI: "crush", Path: pjPath}
	if !c.Detect() {
		return r
	}
	seen := make(map[string]bool)
	for _, d := range c.crushDataDirs() {
		filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(path, ".jsonl") {
				seen[path] = true
			}
			return nil
		})
	}
	r.Available = true
	r.SessionFiles = len(seen)
	var first, last time.Time
	for path := range seen {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		mt := info.ModTime()
		if first.IsZero() || mt.Before(first) {
			first = mt
		}
		if mt.After(last) {
			last = mt
		}
	}
	r.FirstModified, r.LastModified = first, last
	return r
}

func (c *CrushImporter) IterUsages(since time.Time, out chan<- domain.RawUsage) {
	defer close(out)
	if !c.Detect() {
		return
	}
	seen := make(map[string]bool)
	for _, d := range c.crushDataDirs() {
		filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
				return nil
			}
			if seen[path] {
				return nil
			}
			seen[path] = true
			sessionID := strings.TrimSuffix(info.Name(), ".jsonl")
			f, ferr := os.Open(path)
			if ferr != nil {
				return nil
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
				if json.Unmarshal([]byte(line), &obj) != nil {
					continue
				}
				msg, _ := obj["message"].(map[string]interface{})
				if msg == nil {
					msg = obj
				}
				usage, _ := msg["usage"].(map[string]interface{})
				if usage == nil {
					usage, _ = obj["usage"].(map[string]interface{})
				}
				if usage == nil {
					continue
				}
				prompt := toIntAlt(usage, "prompt_tokens", "input_tokens")
				completion := toIntAlt(usage, "completion_tokens", "output_tokens")
				if prompt == 0 && completion == 0 {
					continue
				}
				ts := strOr(obj, "timestamp", strOr(msg, "timestamp", ""))
				if beforeSince(ts, since) {
					continue
				}
				model := strOr(msg, "model", strOr(obj, "model", "unknown"))
				provider := strOr(msg, "provider", strOr(obj, "provider", "unknown"))
				eid := strOr(obj, "id", strOr(obj, "uuid", ""))
				if eid == "" {
					eid = fmt.Sprintf("%s:%d", sessionID, idx)
				}
				out <- domain.RawUsage{
					CLI:              "crush",
					SessionID:        sessionID,
					SessionFile:      path,
					EntryID:          eid,
					Timestamp:        ts,
					Provider:         provider,
					Model:            model,
					PromptTokens:     prompt,
					CompletionTokens: completion,
					CacheReadTokens:  toInt(usage, "cache_read_tokens"),
					CacheWriteTokens: toInt(usage, "cache_write_tokens"),
					Project:          filepath.Dir(path),
				}
				idx++
			}
			return nil
		})
	}
}

// OpencodeImporter imports from opencode SQLite DB (~/.local/share/opencode/opencode.db).
type OpencodeImporter struct{ paths Paths }

func (o *OpencodeImporter) Name() string { return "opencode" }

// root is the CLI data root, honoring sessions.paths.
func (o *OpencodeImporter) root() string {
	return o.paths.Resolve("opencode", filepath.Join(homeDir(), ".local", "share", "opencode", "opencode.db"))
}

func (o *OpencodeImporter) Detect() bool {
	p := o.root()
	_, err := os.Stat(p)
	return err == nil
}

func (o *OpencodeImporter) Scan(since time.Time) domain.ScanResult {
	dbPath := o.root()
	r := domain.ScanResult{CLI: "opencode", Path: dbPath}
	if !o.Detect() {
		return r
	}
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		r.Available = true
		return r
	}
	defer conn.Close()
	var total int
	if err := conn.QueryRow("SELECT COUNT(*) FROM message").Scan(&total); err == nil {
		r.TotalEntries = total
	}
	var firstMs, lastMs sql.NullInt64
	_ = conn.QueryRow("SELECT MIN(time_created), MAX(time_created) FROM message").Scan(&firstMs, &lastMs)
	if firstMs.Valid {
		r.FirstModified = time.UnixMilli(firstMs.Int64).UTC()
	}
	if lastMs.Valid {
		r.LastModified = time.UnixMilli(lastMs.Int64).UTC()
	}
	r.Available = true
	r.SessionFiles = 1
	return r
}

func (o *OpencodeImporter) IterUsages(since time.Time, out chan<- domain.RawUsage) {
	defer close(out)
	dbPath := o.root()
	if !o.Detect() {
		return
	}
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return
	}
	defer conn.Close()

	query := "SELECT id, session_id, time_created, data FROM message"
	var args []interface{}
	if !since.IsZero() {
		query += " WHERE time_created >= ?"
		args = append(args, since.UnixMilli())
	}
	query += " ORDER BY time_created ASC"

	rows, err := conn.Query(query, args...)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, sessionID sql.NullString
		var timeCreated sql.NullInt64
		var dataStr sql.NullString
		if err := rows.Scan(&id, &sessionID, &timeCreated, &dataStr); err != nil {
			continue
		}
		if !dataStr.Valid {
			continue
		}
		var data map[string]interface{}
		if json.Unmarshal([]byte(dataStr.String), &data) != nil {
			continue
		}
		if data["role"] != "assistant" {
			continue
		}
		tokens, _ := data["tokens"].(map[string]interface{})
		if tokens == nil || toInt(tokens, "total") == 0 {
			continue
		}
		ts := ""
		if timeCreated.Valid {
			ts = time.UnixMilli(timeCreated.Int64).UTC().Format(time.RFC3339)
		}
		if beforeSince(ts, since) {
			continue
		}
		cache, _ := tokens["cache"].(map[string]interface{})
		sid := sessionID.String
		if sid == "" {
			sid = "unknown"
		}
		eid := id.String
		if eid == "" {
			eid = fmt.Sprintf("%s:%d", sid, timeCreated.Int64)
		}
		project := "default"
		if p, ok := data["path"].(map[string]interface{}); ok {
			if cwd, ok := p["cwd"].(string); ok && cwd != "" {
				project = cwd
			}
		}
		rawCost := 0.0
		if c, ok := data["cost"]; ok {
			if cf, ok := c.(float64); ok {
				rawCost = cf
			}
		}
		out <- domain.RawUsage{
			CLI:              "opencode",
			SessionID:        sid,
			SessionFile:      "",
			EntryID:          eid,
			Timestamp:        ts,
			Provider:         strOr(data, "providerID", "unknown"),
			Model:            strOr(data, "modelID", "unknown"),
			PromptTokens:     toInt(tokens, "input"),
			CompletionTokens: toInt(tokens, "output"),
			CacheReadTokens:  toInt(cache, "read"),
			CacheWriteTokens: toInt(cache, "write"),
			Project:          project,
			RawCost:          rawCost,
		}
	}
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// strOr returns m[key] as string or fallback if missing/not-string.
func strOr(m map[string]interface{}, key, fallback string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

// toInt returns m[key] as int, handling float64 from JSON.
func toInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

// toIntAlt tries key1 first, then key2.
func toIntAlt(m map[string]interface{}, key1, key2 string) int {
	if v := toInt(m, key1); v != 0 {
		return v
	}
	return toInt(m, key2)
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// piProjectFromDir decodes pi's encoded directory name (--home-user-...-- -> /home/user/...).
func piProjectFromDir(dir string) string {
	name := filepath.Base(dir)
	name = strings.Trim(name, "-")
	if strings.HasPrefix(name, "-") {
		return "/" + strings.ReplaceAll(strings.TrimLeft(name, "-"), "-", "/")
	}
	if name == "" {
		return "default"
	}
	return name
}
