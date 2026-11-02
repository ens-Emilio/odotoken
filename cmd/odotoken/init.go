package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ens-Emilio/odotoken/internal/cli"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.RunWizard(os.Stdout, os.Stdin)
		},
	}
}