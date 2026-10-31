package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "odotoken",
		Short: "Transparent AI token metering proxy, usage tracker and budget guard",
		Long: `odotoken is a transparent reverse proxy for AI APIs that meters token usage,
enforces budget limits, and provides detailed usage analytics.`,
		Version: version,
	}

	rootCmd.AddCommand(
		newStartCmd(),
		newStopCmd(),
		newStatusCmd(),
		newStatsCmd(),
		newExportCmd(),
		newPricesCmd(),
		newImportCmd(),
		newDoctorCmd(),
		newMCPCmd(),
	)
	return rootCmd
}
