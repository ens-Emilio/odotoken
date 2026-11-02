package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ens-Emilio/odotoken/internal/cli"
)

func newTailCmd() *cobra.Command {
	var (
		project string
		tag     string
		server  string
	)
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream live proxy sessions in real time",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return cli.StreamTail(ctx, server, project, tag, os.Stdout)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Filter by project name")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	cmd.Flags().StringVar(&server, "server", "http://127.0.0.1:8080", "odotoken server URL")
	return cmd
}