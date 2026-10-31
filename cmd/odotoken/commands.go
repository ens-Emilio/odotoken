package main

import (
	"context"
	"encoding/json"
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
	"unicode"

	"github.com/spf13/cobra"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/importers"
	"github.com/ens-Emilio/odotoken/internal/pricing"
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

			// Background work the daemon owns: durable alert delivery, token
			// event retention, the scheduled import, and the OTLP export. Each
			// is independent, so one failing pass cannot stop the others.
			go srv.Guard.StartAlertWorker(ctx, 5*time.Second)
			go runRetentionWorker(ctx, database, cfg, logger)
			go runScheduledImport(ctx, database, cfg, logger)
			if srv.Otel != nil {
				go srv.Otel.Start(ctx)
			}
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
			resp, err := client.Get("http://" + addr + "/health")
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
		since      string
		dbPath     string
		configPath string
		clis       []string
		dryRun     bool
		jsonOut    bool
		discover   bool
		lang       string
	)
	cmd := &cobra.Command{
		Use:   "import [cli...]",
		Short: "Import usage data from CLI session files or databases",
		Long: `Import historical usage data from supported CLIs.

Without arguments every detected CLI is imported. Costs are recomputed from
prices.json, so an import is consistent with what the proxy would have
recorded for the same tokens.

Examples:
  odotoken import                          # every detected CLI
  odotoken import claude codex             # only these two
  odotoken import --dry-run --json         # preview, machine-readable
  odotoken import --since 2024-01-01T00:00:00Z
  odotoken import --discover --json        # what is on this machine`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(configPath)

			var sinceTime time.Time
			if since != "" {
				t, err := time.Parse(time.RFC3339, since)
				if err != nil {
					return fmt.Errorf("invalid --since %q: use RFC3339, e.g. 2024-01-01T00:00:00Z", since)
				}
				sinceTime = t.UTC()
			}

			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			engine := importers.NewEngine(
				importers.Paths(cfg.Sessions.Paths),
				pricing.NewCalculator("prices.json"),
				database,
			)

			selected := append(args, clis...)
			if discover {
				sources := engine.Discover(sinceTime)
				importers.SortSources(sources)
				if jsonOut {
					return writeJSON(sources)
				}
				printSources(sources, cliLanguage(lang))
			}

			opts := importers.Options{CLIs: selected, Since: sinceTime, DryRun: dryRun}
			if err := importers.ValidateOptions(opts, engine.Registry); err != nil {
				return err
			}

			var progress importers.Progress
			report, runErr := engine.Run(context.Background(), opts, func(p importers.Progress) {
				progress = p
				if !jsonOut {
					fmt.Printf("\r[%d/%d] %s: %d scanned, %d new...", p.Index, p.Total, p.CLI, p.Scanned, p.Inserted)
				}
			})
			if !jsonOut && progress.Total > 0 {
				fmt.Println()
			}

			if !dryRun {
				// A failed run record must not fail an import whose events landed.
				_ = database.RecordImport(selected, report)
			}

			if jsonOut {
				if err := writeJSON(report); err != nil {
					return err
				}
			} else {
				printReport(report, cliLanguage(lang))
			}
			return runErr
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Import only events after this RFC3339 timestamp")
	cmd.Flags().StringVar(&dbPath, "db", "odotoken.db", "database file path")
	cmd.Flags().StringVar(&configPath, "config", "config.json", "config file path (for sessions.paths)")
	cmd.Flags().StringSliceVar(&clis, "cli", nil, "CLI to import (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Parse and price without writing")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	cmd.Flags().BoolVar(&discover, "discover", false, "List detected CLIs instead of importing")
	cmd.Flags().StringVar(&lang, "lang", "", "Human output language (default: terminal locale; --json is never translated)")
	return cmd
}

// pad fits a cell to a column width in display columns, not bytes: `%-10s`
// counts bytes, so a Chinese or Hindi label leaves the table ragged.
func pad(cell string, width int) string {
	visible := 0
	for _, r := range cell {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			continue // combining marks take no column of their own
		}
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) ||
			unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			visible += 2
			continue
		}
		visible++
	}
	if visible >= width {
		return cell
	}
	return cell + strings.Repeat(" ", width-visible)
}

func padLeft(cell string, width int) string {
	padded := pad(cell, width)
	return strings.Repeat(" ", len([]rune(padded))-len([]rune(cell))) + cell
}

func printSources(sources []importers.Source, lang string) {
	labels := cliLabelsFor(lang)
	fmt.Printf("%s %s %s %s\n", pad("CLI", 10), pad(labels.status, 12), pad(labels.files, 10), "PATH")
	for _, s := range sources {
		status := labels.missing
		if s.Available {
			status = labels.ready
		}
		fmt.Printf("%s %s %s %s\n", pad(s.CLI, 10), pad(status, 12),
			pad(strconv.Itoa(s.SessionFiles), 10), s.Path)
	}
}

func printReport(r importers.Report, lang string) {
	labels := cliLabelsFor(lang)
	fmt.Printf("%s %s %s %s %s\n", pad("CLI", 10), padLeft(labels.scanned, 10),
		padLeft(labels.new, 8), padLeft(labels.dupes, 8), padLeft(labels.cost, 12))
	for _, c := range r.CLIs {
		fmt.Printf("%s %s %s %s %s\n", pad(c.CLI, 10),
			padLeft(strconv.Itoa(c.Scanned), 10), padLeft(strconv.Itoa(c.Inserted), 8),
			padLeft(strconv.Itoa(c.Duplicates), 8), padLeft(fmt.Sprintf("%.4f", c.CostUSD), 12))
		if c.Error != "" {
			fmt.Printf("  %s\n", c.Error)
		}
	}
	fmt.Printf("\n%s: %d %s, %d %s, %d %s, $%.4f\n", labels.total, r.Inserted, labels.new, r.Duplicates, labels.dupes, r.Tokens, labels.tokens, r.CostUSD)
	if r.Unpriced > 0 {
		fmt.Printf("%d %s\n", r.Unpriced, labels.unpriced)
	}
	if r.Status != "" && r.Status != "done" {
		fmt.Printf("%s %s.\n", labels.importWord, r.Status)
	}
	if r.DryRun {
		fmt.Println(labels.dryRun)
	}
}

func writeJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
