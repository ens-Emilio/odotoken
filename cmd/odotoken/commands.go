package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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

			srv := server.NewWithConfigPath(database, cfg, logger, configPath)

			bindHost := host
			if bindHost == "" {
				bindHost = cfg.Server.BindHost
			}
			if bindHost == "" {
				bindHost = "127.0.0.1"
			}
			addr := net.JoinHostPort(bindHost, strconv.Itoa(port))
			httpSrv := &http.Server{
				Addr:              addr,
				Handler:           srv.Router,
				ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout:       60 * time.Second,
				WriteTimeout:      0, // streaming proxy responses are open-ended
				IdleTimeout:       120 * time.Second,
			}

			pidFile, err := daemonPIDFile()
			if err == nil {
				err = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d %s\n", os.Getpid(), addr)), 0o600)
			}
			if err != nil {
				logger.Warn("could not write pid file", "error", err)
			}
			defer os.Remove(pidFile)

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
	cmd.Flags().StringVar(&host, "host", "", "bind host (default: server.bind_host or 127.0.0.1)")
	cmd.Flags().IntVar(&port, "port", 8080, "bind port")
	cmd.Flags().StringVar(&configPath, "config", "config.json", "config file path")
	cmd.Flags().StringVar(&dbPath, "db", "odotoken.db", "database file path")
	return cmd
}

func daemonPIDFile() (string, error) {
	stateDir := os.Getenv("XDG_RUNTIME_DIR")
	if stateDir == "" {
		stateDir = os.TempDir()
	}
	return filepath.Join(stateDir, "odotoken.pid"), nil
}

func readDaemonRecord() (pid int, addr string, path string, err error) {
	path, err = daemonPIDFile()
	if err != nil {
		return 0, "", "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", path, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, "", path, fmt.Errorf("pid file is empty")
	}
	pid, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", path, fmt.Errorf("invalid pid file: %w", err)
	}
	if len(fields) > 1 {
		addr = fields[1]
	}
	return pid, addr, path, nil
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, _, pidFile, err := readDaemonRecord()
			if err != nil {
				return fmt.Errorf("no running daemon found: %w", err)
			}
			proc, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("process %d not found: %w", pid, err)
			}
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("failed to stop process: %w", err)
			}
			_ = os.Remove(pidFile)
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
			_, addr, _, err := readDaemonRecord()
			if err != nil || addr == "" {
				fmt.Println("status: not running")
				return nil
			}
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get("http://" + addr + "/v1/health")
			if err != nil {
				fmt.Println("status: not running")
				return nil
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("status: unhealthy (HTTP %d)\n", resp.StatusCode)
				return nil
			}
			fmt.Printf("status: running (%s)\n", addr)
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

				// Batch in chunks: one transaction per commit amortizes fsync,
				// but an unbounded batch would hold the WAL write lock for the
				// entire import and stall the proxy.
				const batchSize = 500
				batch := make([]domain.TokenEvent, 0, batchSize)
				count := 0
				flush := func() {
					if len(batch) == 0 {
						return
					}
					if err := database.InsertEvents(batch); err != nil {
						fmt.Printf("[error] %s: failed to insert batch: %v\n", imp.Name(), err)
					} else {
						count += len(batch)
					}
					batch = batch[:0]
				}

				for usage := range out {
					batch = append(batch, domain.TokenEvent{
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
					})
					if len(batch) >= batchSize {
						flush()
					}
				}
				flush()
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
