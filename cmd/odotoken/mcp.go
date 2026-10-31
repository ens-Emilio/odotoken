package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/guard"
	"github.com/ens-Emilio/odotoken/internal/mcp"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

// newMCPCmd serves the read-only analytics over stdio so an MCP client can ask
// odotoken what it spent without a second HTTP surface to authenticate.
func newMCPCmd() *cobra.Command {
	var (
		configPath string
		dbPath     string
	)
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve read-only analytics over MCP on stdio",
		Long: `Speak MCP (JSON-RPC 2.0, newline-delimited) on stdin/stdout.

The tools are odotoken_stats, odotoken_guard and odotoken_forecast. Nothing is
written: an MCP client can read usage but cannot change limits or prices.

Example client config:
  {"command": "odotoken", "args": ["mcp", "--db", "/var/lib/odotoken/odotoken.db"]}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			g := guard.New(database, cfg.Guard, cfg.Alerts, newCLILogger())
			srv := mcp.NewOdotokenServer(database, g.Check, pricing.NewCalculator("prices.json"))
			return srv.Serve(os.Stdin, os.Stdout)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "config.json", "config file path")
	cmd.Flags().StringVar(&dbPath, "db", "odotoken.db", "database file path")
	return cmd
}
