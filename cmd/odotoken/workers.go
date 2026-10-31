package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/importers"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

// runRetentionWorker applies storage.token_events_retention_days once at start
// and then daily. A retention of 0 disables it; the worker exits instead of
// waking up to do nothing.
func runRetentionWorker(ctx context.Context, database *db.Database, cfg config.Config, logger *slog.Logger) {
	days := cfg.Storage.TokenEventsRetentionDays
	if days <= 0 {
		return
	}
	apply := func() {
		result, err := database.ApplyRetention(days, cfg.Storage.VacuumAfterRetention)
		if err != nil {
			logger.Warn("retention pass failed", "error", err)
			return
		}
		if result.Deleted > 0 {
			logger.Info("retention removed old events",
				"deleted", result.Deleted, "vacuum", result.Vacuum, "oldest", result.Oldest)
		}
	}
	apply()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			apply()
		}
	}
}

// runScheduledImport imports every detected CLI on scheduled_import.interval.
// It uses the same engine as `odotoken import`, so the two cannot disagree
// about what a duplicate is.
func runScheduledImport(ctx context.Context, database *db.Database, cfg config.Config, logger *slog.Logger) {
	if !cfg.ScheduledImport.Enabled {
		return
	}
	interval, err := time.ParseDuration(cfg.ScheduledImport.Interval)
	if err != nil || interval <= 0 {
		logger.Warn("scheduled import disabled: invalid interval",
			"interval", cfg.ScheduledImport.Interval, "error", err)
		return
	}
	engine := importers.NewEngine(
		importers.Paths(cfg.Sessions.Paths),
		pricing.NewCalculator("prices.json"),
		database,
	)

	run := func() {
		opts := importers.Options{CLIs: cfg.ScheduledImport.CLIs}
		if err := importers.ValidateOptions(opts, engine.Registry); err != nil {
			logger.Warn("scheduled import skipped", "error", err)
			return
		}
		report, err := engine.Run(ctx, opts, nil)
		if err != nil {
			logger.Warn("scheduled import failed", "error", err)
			return
		}
		if !report.DryRun {
			_ = database.RecordImport(opts.CLIs, report)
		}
		logger.Info("scheduled import finished",
			"inserted", report.Inserted, "duplicates", report.Duplicates, "cost_usd", report.CostUSD)
	}
	run()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
