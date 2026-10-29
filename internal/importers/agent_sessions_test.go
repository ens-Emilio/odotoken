package importers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

func TestOmoImporterReadsAssistantUsage(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "--tmp-project--")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "session_abc.jsonl")
	data := `{"type":"message","id":"msg-1","timestamp":"2026-10-01T12:00:00Z","message":{"role":"assistant","provider":"openai","model":"gpt-4o-mini","usage":{"input":12,"output":7,"cacheRead":2,"cacheWrite":1,"cost":{"total":0.004}}}}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	imp := &OmoImporter{paths: Paths{"omo": root}}
	if !imp.Detect() {
		t.Fatal("OMO source should be detected")
	}
	out := make(chan domain.RawUsage, 1)
	imp.IterUsages(time.Time{}, out)
	usage, ok := <-out
	if !ok {
		t.Fatal("expected one usage record")
	}
	if usage.CLI != "omo" || usage.EntryID != "msg-1" || usage.PromptTokens != 12 || usage.CompletionTokens != 7 || usage.RawCost != 0.004 {
		t.Fatalf("usage = %+v", usage)
	}
	if _, ok := <-out; ok {
		t.Fatal("expected exactly one usage record")
	}
}
