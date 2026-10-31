package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/guard"
	"github.com/ens-Emilio/odotoken/internal/otel"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

// doctorCheck is one diagnostic line. Status is "ok", "warn" or "fail".
type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// doctorReport is the machine-readable form of `odotoken doctor`.
type doctorReport struct {
	Checks []doctorCheck `json:"checks"`
	Failed int           `json:"failed"`
	Warned int           `json:"warned"`
}

func (r *doctorReport) add(name, status, detail string) {
	r.Checks = append(r.Checks, doctorCheck{Name: name, Status: status, Detail: detail})
	switch status {
	case "fail":
		r.Failed++
	case "warn":
		r.Warned++
	}
}

func newDoctorCmd() *cobra.Command {
	var (
		configPath  string
		dbPath      string
		jsonOut     bool
		flushAlerts bool
		vacuum      bool
	)
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose configuration, database and alert delivery",
		Long: `Run the checks an operator needs before trusting the numbers: config
parsing, budget settings, price catalog, database schema, retention, provider
fallbacks and the alert queue.

  odotoken doctor                 # human-readable report
  odotoken doctor --json          # same report for scripts
  odotoken doctor --flush-alerts  # deliver queued alerts now
  odotoken doctor --vacuum        # apply retention, then reclaim disk space`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := &doctorReport{}
			cfg, cfgErr := config.Load(configPath)
			if cfgErr != nil {
				report.add("config", "fail", cfgErr.Error())
			} else {
				report.add("config", "ok", fmt.Sprintf("loaded %s", configPath))
				checkGuardConfig(report, cfg)
				checkProviders(report, cfg)
				checkScheduledImport(report, cfg)
			}

			database, err := db.Open(dbPath)
			if err != nil {
				report.add("database", "fail", err.Error())
				return finishDoctor(report, jsonOut)
			}
			defer database.Close()

			checkDatabase(report, database, dbPath)
			checkPrices(report)
			checkAlerts(report, cfg, database)
			if cfgErr == nil {
				checkOtel(report, cfg, database)
				checkDaemon(report)
			}

			if flushAlerts {
				g := guard.New(database, cfg.Guard, cfg.Alerts, newCLILogger())
				delivered := g.DeliverPending(context.Background())
				report.add("alerts.flush", "ok", fmt.Sprintf("delivered %d queued alert(s)", delivered))
			}
			if vacuum {
				result, err := database.ApplyRetention(cfg.Storage.TokenEventsRetentionDays, true)
				if err != nil {
					report.add("retention", "fail", err.Error())
				} else {
					report.add("retention", "ok", fmt.Sprintf(
						"deleted %d event(s), vacuum=%t, oldest remaining %s",
						result.Deleted, result.Vacuum, orNone(result.Oldest)))
				}
			}
			return finishDoctor(report, jsonOut)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "config.json", "config file path")
	cmd.Flags().StringVar(&dbPath, "db", "odotoken.db", "database file path")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	cmd.Flags().BoolVar(&flushAlerts, "flush-alerts", false, "Deliver queued alerts now")
	cmd.Flags().BoolVar(&vacuum, "vacuum", false, "Apply retention and VACUUM the database")
	return cmd
}

func finishDoctor(report *doctorReport, jsonOut bool) error {
	if jsonOut {
		if err := writeJSON(report); err != nil {
			return err
		}
	} else {
		for _, c := range report.Checks {
			mark := "ok  "
			switch c.Status {
			case "warn":
				mark = "warn"
			case "fail":
				mark = "FAIL"
			}
			fmt.Printf("%s  %-22s %s\n", mark, c.Name, c.Detail)
		}
		fmt.Printf("\n%d failed, %d warning(s)\n", report.Failed, report.Warned)
	}
	if report.Failed > 0 {
		return fmt.Errorf("%d check(s) failed", report.Failed)
	}
	return nil
}

func checkGuardConfig(report *doctorReport, cfg config.Config) {
	g := cfg.Guard
	if g.DailyCostLimitUSD == 0 && g.MonthlyCostLimitUSD == 0 && g.DailyTokenLimit == 0 && g.MonthlyTokenLimit == 0 {
		report.add("guard", "warn", "no budget limits configured; the guard records spend but never blocks")
	} else {
		report.add("guard", "ok", fmt.Sprintf(
			"daily $%.2f / %d tokens, monthly $%.2f / %d tokens, action %s",
			g.DailyCostLimitUSD, g.DailyTokenLimit, g.MonthlyCostLimitUSD, g.MonthlyTokenLimit, g.ActionOnBreach))
	}
	if g.Timezone != "" {
		if _, err := time.LoadLocation(g.Timezone); err != nil {
			report.add("guard.timezone", "fail", fmt.Sprintf("invalid timezone %q: %v", g.Timezone, err))
		}
	}
	if g.ActionOnBreach != "block" && g.ActionOnBreach != "warn" && g.ActionOnBreach != "" {
		report.add("guard.action", "warn", fmt.Sprintf("unknown action_on_breach %q, treated as warn", g.ActionOnBreach))
	}
	for project, override := range g.Projects {
		if override.Timezone != nil {
			if _, err := time.LoadLocation(*override.Timezone); err != nil {
				report.add("guard.projects", "fail", fmt.Sprintf("project %q timezone %q is invalid", project, *override.Timezone))
			}
		}
	}
}

func checkProviders(report *doctorReport, cfg config.Config) {
	names := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		names = append(names, name)
	}
	report.add("providers", "ok", fmt.Sprintf("%d configured", len(cfg.Providers)))
	for name, provider := range cfg.Providers {
		for _, fallback := range provider.Fallbacks {
			if _, ok := cfg.Providers[fallback]; !ok {
				report.add("providers.fallback", "fail",
					fmt.Sprintf("provider %q falls back to %q, which is not configured", name, fallback))
			}
			if fallback == name {
				report.add("providers.fallback", "warn",
					fmt.Sprintf("provider %q lists itself as a fallback", name))
			}
		}
	}
}

func checkScheduledImport(report *doctorReport, cfg config.Config) {
	if !cfg.ScheduledImport.Enabled {
		report.add("scheduled_import", "ok", "disabled")
		return
	}
	if _, err := time.ParseDuration(cfg.ScheduledImport.Interval); err != nil {
		report.add("scheduled_import", "fail",
			fmt.Sprintf("invalid interval %q: %v", cfg.ScheduledImport.Interval, err))
		return
	}
	report.add("scheduled_import", "ok", "every "+cfg.ScheduledImport.Interval)
}

func checkDatabase(report *doctorReport, database *db.Database, dbPath string) {
	stats, err := os.Stat(dbPath)
	if err != nil {
		report.add("database", "warn", fmt.Sprintf("stat %s: %v", dbPath, err))
	} else {
		report.add("database", "ok", fmt.Sprintf("%s, %.1f MB", dbPath, float64(stats.Size())/(1<<20)))
	}
	counts, err := database.TableCounts()
	if err != nil {
		report.add("database.schema", "fail", err.Error())
		return
	}
	report.add("database.schema", "ok", fmt.Sprintf(
		"token_events=%d guard_events=%d active_runs=%d import_runs=%d",
		counts.TokenEvents, counts.GuardEvents, counts.ActiveRuns, counts.ImportRuns))
}

func checkPrices(report *doctorReport) {
	calc := pricing.NewCalculator("prices.json")
	if len(calc.Prices()) == 0 {
		report.add("prices", "warn", "prices.json is empty or unreadable; costs fall back to built-in rates")
		return
	}
	report.add("prices", "ok", fmt.Sprintf("%d models priced", len(calc.Prices())))
}

func checkAlerts(report *doctorReport, cfg config.Config, database *db.Database) {
	channels := 0
	if cfg.Alerts.SlackURL != "" {
		channels++
	}
	if cfg.Alerts.DiscordURL != "" {
		channels++
	}
	if cfg.Alerts.Telegram.BotToken != "" && cfg.Alerts.Telegram.ChatID != "" {
		channels++
	}
	if cfg.Alerts.Email.SMTPHost != "" && cfg.Alerts.Email.ToAddr != "" {
		channels++
	}
	if channels == 0 {
		report.add("alerts", "warn", "no alert channel configured; guard transitions are only logged")
	} else {
		report.add("alerts", "ok", fmt.Sprintf("%d channel(s), retry max %d, base %ds",
			channels, cfg.Alerts.Retry.MaxAttempts, cfg.Alerts.Retry.BaseDelaySeconds))
	}
	pending, err := database.PendingAlertCount()
	if err != nil {
		report.add("alerts.queue", "fail", err.Error())
		return
	}
	status := "ok"
	if pending > 0 {
		status = "warn"
	}
	report.add("alerts.queue", status, fmt.Sprintf("%d queued", pending))
}

func checkOtel(report *doctorReport, cfg config.Config, database *db.Database) {
	if !cfg.Otel.Enabled {
		report.add("otel", "ok", "disabled")
		return
	}
	exporter := otel.New(database, cfg.Otel, newCLILogger())
	if exporter == nil {
		report.add("otel", "warn", "enabled but otel.endpoint is empty; nothing is exported")
		return
	}
	endpoint, interval := exporter.Status()
	report.add("otel", "ok", fmt.Sprintf("enabled, POST %s every %s", endpoint, interval))
}

func checkDaemon(report *doctorReport) {
	_, addr, _, err := readDaemonRecord()
	if err != nil || addr == "" {
		report.add("daemon", "warn", "not running (no pid file)")
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/health")
	if err != nil {
		report.add("daemon", "warn", fmt.Sprintf("pid file points at %s but it is unreachable", addr))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		report.add("daemon", "fail", fmt.Sprintf("running at %s but /health returned HTTP %d", addr, resp.StatusCode))
		return
	}
	report.add("daemon", "ok", fmt.Sprintf("running at %s", addr))
}

func orNone(v string) string {
	if v == "" {
		return "none"
	}
	return v
}

func newCLILogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}
