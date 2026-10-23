package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
	"github.com/ens-Emilio/odotoken/internal/importers"
	"github.com/ens-Emilio/odotoken/internal/server"
)

func newStartCmd() *cobra.Command {
	var (
		host       string
		port       int
		configPath string
		dbPath     string
	)
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the odotoken proxy and dashboard service",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			srv := server.New(database, cfg, logger)

			addr := fmt.Sprintf("%s:%d", host, port)
			httpSrv := &http.Server{Addr: addr, Handler: srv.Router}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			go func() {
				logger.Info("starting odotoken", "addr", addr)
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Error("server error", "error", err)
					os.Exit(1)
				}
			}()

			<-ctx.Done()
			logger.Info("shutting down")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return httpSrv.Shutdown(shutdownCtx)
		},
	}
	cmd.Flags().StringVar(&host, "host", "0.0.0.0", "bind host")
	cmd.Flags().IntVar(&port, "port", 8080, "bind port")
	cmd.Flags().StringVar(&configPath, "config", "config.json", "config file path")
	cmd.Flags().StringVar(&dbPath, "db", "odotoken.db", "database file path")
	return cmd
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pidFile := filepath.Join(os.TempDir(), "odotoken.pid")
			data, err := os.ReadFile(pidFile)
			if err != nil {
				return fmt.Errorf("no running daemon found: %w", err)
			}
			var pid int
			fmt.Sscanf(string(data), "%d", &pid)
			proc, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("process %d not found: %w", pid, err)
			}
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("failed to stop process: %w", err)
			}
			os.Remove(pidFile)
			fmt.Println("daemon stopped")
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get("http://localhost:8080/v1/health")
			if err != nil {
				fmt.Println("status: not running")
				return nil
			}
			defer resp.Body.Close()
			fmt.Printf("status: running (HTTP %d)\n", resp.StatusCode)
			return nil
		},
	}
}

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show usage statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load("config.json")
			if err != nil {
				return err
			}
			database, err := db.Open("odotoken.db")
			if err != nil {
				return err
			}
			defer database.Close()

			now := time.Now().UTC()
			from := now.AddDate(0, 0, -30)
			stats, err := database.GetDailyStats(from, now, "")
			if err != nil {
				return err
			}
			if len(stats) == 0 {
				fmt.Println("No events recorded yet.")
				return nil
			}
			fmt.Printf("%-12s %8s %12s %10s\n", "DATE", "REQUESTS", "TOKENS", "COST USD")
			for _, s := range stats {
				fmt.Printf("%-12s %8d %12d %10.4f\n", s.Date, s.TotalRequests, s.TotalTokens, s.TotalCostUSD)
			}
			_ = cfg
			return nil
		},
	}
}

func newImportCmd() *cobra.Command {
	var (
		since  string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "import [cli]",
		Short: "Import usage data from CLI session files or databases",
		Long: `Import historical usage data from supported CLIs:
  omp      - OMP/Ghostcli stats.db (SQLite)
  claude   - Claude CLI session files
  codex    - Codex CLI session files
  crush    - Crush session files
  opencode - Opencode session files
  pi       - Pi agent session files

Examples:
  odotoken import omp              # Import all OMP data
  odotoken import omp --since 2024-01-01T00:00:00Z  # Import since date
  odotoken import                  # Import from all available sources`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			registry := importers.NewRegistry()
			cliFilter := ""
			if len(args) > 0 {
				cliFilter = args[0]
			}

			imported := 0
			for _, imp := range registry.All() {
				if cliFilter != "" && imp.Name() != cliFilter {
					continue
				}
				if !imp.Detect() {
					fmt.Printf("[skip] %s: not detected\n", imp.Name())
					continue
				}

				scan := imp.Scan(since)
				if !scan.Available {
					fmt.Printf("[skip] %s: not available at %s\n", imp.Name(), scan.Path)
					continue
				}
				fmt.Printf("[scan] %s: %d entries at %s\n", imp.Name(), scan.TotalEntries, scan.Path)

				out := make(chan domain.RawUsage, 100)
				go imp.IterUsages(since, out)

				count := 0
				for usage := range out {
					event := domain.TokenEvent{
						Timestamp:        parseTimestamp(usage.Timestamp),
						Provider:         usage.Provider,
						Model:            usage.Model,
						ClientApp:        usage.CLI,
						PromptTokens:     usage.PromptTokens,
						CompletionTokens: usage.CompletionTokens,
						TotalTokens:      usage.PromptTokens + usage.CompletionTokens,
						CacheReadTokens:  usage.CacheReadTokens,
						CacheWriteTokens: usage.CacheWriteTokens,
						CostUSD:          usage.RawCost,
						ProjectID:        usage.Project,
						Source:           "import:" + usage.CLI,
						SessionID:        usage.SessionID,
						SessionFile:      usage.SessionFile,
						EntryID:          usage.EntryID,
					}
					if _, err := database.InsertEvent(event); err != nil {
						fmt.Printf("[error] %s: failed to insert event: %v\n", imp.Name(), err)
						continue
					}
					count++
				}
				fmt.Printf("[done] %s: imported %d events\n", imp.Name(), count)
				imported += count
			}
			fmt.Printf("\nTotal: %d events imported\n", imported)
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Import only events after this ISO8601 timestamp")
	cmd.Flags().StringVar(&dbPath, "db", "odotoken.db", "database file path")
	return cmd
}

func parseTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Now().UTC()
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}