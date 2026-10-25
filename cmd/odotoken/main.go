package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.3.0-go"

func main() {
	rootCmd := &cobra.Command{
		Use:   "odotoken",
		Short: "Transparent AI token metering proxy, usage tracker and budget guard",
		Long: `odotoken is a transparent reverse proxy for AI APIs that meters token usage,
enforces budget limits, and provides detailed usage analytics.`,
		Version: version,
	}

	// Add subcommands (will be implemented in Phase 7)
	// Add subcommands
	rootCmd.AddCommand(
		newStartCmd(),
		newStopCmd(),
		newStatusCmd(),
		newStatsCmd(),
		newImportCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
