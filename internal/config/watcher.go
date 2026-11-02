package config

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// WatchConfig listens for SIGHUP and reloads the config file. On success it
// calls reload with the new config; on parse error it logs and keeps the old
// config. The goroutine exits when ctx is cancelled.
func WatchConfig(ctx context.Context, configPath string, reload func(Config) error, logger *slog.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	defer signal.Stop(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			cfg, err := Load(configPath)
			if err != nil {
				logger.Error("config reload failed", "error", err)
				continue
			}
			if err := reload(cfg); err != nil {
				logger.Error("config reload callback failed", "error", err)
				continue
			}
			logger.Info("config reloaded via SIGHUP")
		}
	}
}