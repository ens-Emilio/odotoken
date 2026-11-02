package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type liveSession struct {
	RunID       string  `json:"run_id"`
	CLI         string  `json:"cli"`
	SessionID   string  `json:"session_id"`
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	TokensIn    int64   `json:"tokens_input"`
	TokensOut   int64   `json:"tokens_output"`
	CostUSD     float64 `json:"cost_usd"`
	Project     string  `json:"project"`
	StartedAt   string  `json:"started_at"`
	LastUpdated string  `json:"last_updated"`
	ElapsedMs   int64   `json:"elapsed_ms"`
}

// StreamTail polls /api/sessions/live every 2s and writes formatted lines to out.
func StreamTail(ctx context.Context, serverURL, project, tag string, out io.Writer) error {
	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		if err := fetchAndPrint(ctx, client, serverURL, project, tag, out); err != nil {
			fmt.Fprintf(out, "[tail] error: %v\n", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func fetchAndPrint(ctx context.Context, client *http.Client, serverURL, project, tag string, out io.Writer) error {
	u, err := url.Parse(serverURL + "/api/sessions/live")
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	q := u.Query()
	if project != "" {
		q.Set("project", project)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var sessions []liveSession
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	for _, s := range sessions {
		fmt.Fprintf(out, "%s | %-10s | %-20s | in=%d out=%d | $%.4f | %s\n",
			s.RunID[:min(len(s.RunID), 8)],
			s.Provider,
			s.Model,
			s.TokensIn,
			s.TokensOut,
			s.CostUSD,
			s.Project,
		)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}