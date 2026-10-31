package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubDaemon points the pid file at a server that answers /health with status,
// so the CLI probes the same URL the daemon serves.
func stubDaemon(t *testing.T, status int) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(upstream.Close)

	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	addr := strings.TrimPrefix(upstream.URL, "http://")
	if err := os.WriteFile(filepath.Join(dir, "odotoken.pid"), []byte("1 "+addr+"\n"), 0o600); err != nil {
		t.Fatalf("write pid file: %v", err)
	}
}

func captureStdout(t *testing.T, run func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	runErr := run()
	os.Stdout = orig
	_ = w.Close()
	out, _ := io.ReadAll(r)
	_ = r.Close()
	if runErr != nil {
		t.Fatalf("command failed: %v", runErr)
	}
	return string(out)
}

// status must probe the local liveness endpoint: /v1/health is behind
// server.api_key, so a key-protected daemon reported itself unhealthy.
func TestStatusUsesLocalLiveness(t *testing.T) {
	stubDaemon(t, http.StatusOK)
	cmd := newStatusCmd()
	out := captureStdout(t, func() error { return cmd.RunE(cmd, nil) })
	if !strings.HasPrefix(out, "status: running") {
		t.Fatalf("status output = %q, want it to report running", out)
	}
}

// doctor treats a non-200 liveness answer as a failure, not a healthy daemon.
func TestDoctorFailsDaemonOnUnhealthyLiveness(t *testing.T) {
	stubDaemon(t, http.StatusServiceUnavailable)
	report := &doctorReport{}
	checkDaemon(report)
	if len(report.Checks) != 1 {
		t.Fatalf("checks = %+v, want one daemon check", report.Checks)
	}
	check := report.Checks[0]
	if check.Name != "daemon" || check.Status != "fail" {
		t.Fatalf("check = %+v, want daemon fail", check)
	}
	if !strings.Contains(check.Detail, "503") {
		t.Fatalf("detail = %q, want the observed status", check.Detail)
	}
}
