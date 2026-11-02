package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ens-Emilio/odotoken/internal/config"
)

// RunWizard runs the interactive onboarding wizard.
// If stdin is not a terminal, it uses sensible defaults without prompting.
func RunWizard(out io.Writer, in io.Reader) error {
	dailyBudget := 5.0
	monthlyBudget := 100.0

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	candidates := map[string]string{
		"claude": filepath.Join(home, ".claude"),
		"codex":  filepath.Join(home, ".codex"),
		"cursor": filepath.Join(home, ".cursor"),
	}

	paths := make(map[string]string)
	for name, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			paths[name] = dir
		}
	}

	if f, ok := in.(*os.File); ok && isTerminal(f) {
		fmt.Fprint(out, "Daily budget (USD) [5.0]: ")
		if _, err := fmt.Fscan(in, &dailyBudget); err != nil {
			dailyBudget = 5.0
		}
		fmt.Fprint(out, "Monthly budget (USD) [100.0]: ")
		if _, err := fmt.Fscan(in, &monthlyBudget); err != nil {
			monthlyBudget = 100.0
		}
	}

	cfg := config.Config{
		Sessions: config.SessionsConfig{
			Enabled: len(paths) > 0,
			Paths:   paths,
		},
		Guard: config.GuardConfig{
			DailyCostLimitUSD:   dailyBudget,
			MonthlyCostLimitUSD: monthlyBudget,
			ActionOnBreach:      "warn",
			WarningThresholdPct: 80,
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile("config.json", data, 0644); err != nil {
		return fmt.Errorf("write config.json: %w", err)
	}

	fmt.Fprintf(out, "Wrote config.json (daily: $%.2f, monthly: $%.2f, sessions: %d)\n",
		dailyBudget, monthlyBudget, len(paths))
	return nil
}

// isTerminal reports whether f is a terminal device.
// ponytail: uses os.File.Stat mode bits; upgrade to golang.org/x/term if needed elsewhere.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}