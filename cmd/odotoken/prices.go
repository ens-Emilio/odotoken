package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/ens-Emilio/odotoken/internal/pricing"
)

func newPricesCmd() *cobra.Command {
	var pricesPath string
	cmd := &cobra.Command{
		Use:   "prices",
		Short: "Inspect and edit model prices",
		Long: `Prices come from prices.json when it exists; the built-in table covers the
rest. Only prices.json entries are editable, and a built-in rate cannot be
deleted: the model would fall back to $0 and stop being metered.`,
	}
	cmd.PersistentFlags().StringVar(&pricesPath, "prices", "prices.json", "price catalog file")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List model prices",
		RunE: func(cmd *cobra.Command, args []string) error {
			query, _ := cmd.Flags().GetString("query")
			limit, _ := cmd.Flags().GetInt("limit")
			asJSON, _ := cmd.Flags().GetBool("json")

			store, err := openPriceStore(pricesPath)
			if err != nil {
				return err
			}
			rows := store.List(query, limit)
			out := cmd.OutOrStdout()
			if asJSON {
				return writeJSONTo(out, rows)
			}
			fmt.Fprintf(out, "%s %s %s %s %s %s\n",
				pad("MODEL", 34), padLeft("INPUT", 10), padLeft("OUTPUT", 10),
				padLeft("CACHE_READ", 12), padLeft("CACHE_WRITE", 12), "SOURCE")
			for _, row := range rows {
				source := "builtin"
				if row.Explicit {
					source = "override"
				}
				fmt.Fprintf(out, "%s %s %s %s %s %s\n",
					pad(row.Model, 34), padLeft(formatRate(row.Input), 10),
					padLeft(formatRate(row.Output), 10), padLeft(formatRate(row.CacheRead), 12),
					padLeft(formatRate(row.CacheWrite), 12), source)
			}
			return nil
		},
	}
	listCmd.Flags().String("query", "", "Filter models by substring")
	listCmd.Flags().Int("limit", 0, "Maximum rows (0 = all)")
	listCmd.Flags().Bool("json", false, "Emit JSON")

	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Write an override price for one model",
		RunE: func(cmd *cobra.Command, args []string) error {
			model, _ := cmd.Flags().GetString("model")
			input, _ := cmd.Flags().GetFloat64("input")
			output, _ := cmd.Flags().GetFloat64("output")
			cacheRead, _ := cmd.Flags().GetFloat64("cache-read")
			cacheWrite, _ := cmd.Flags().GetFloat64("cache-write")

			store, err := openPriceStore(pricesPath)
			if err != nil {
				return err
			}
			row, err := store.Set(model, pricing.ModelRates{
				Input: input, Output: output,
				CacheRead: cacheRead, CacheWrite: cacheWrite,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: input %s, output %s, cache_read %s, cache_write %s (%s)\n",
				row.Model, formatRate(row.Input), formatRate(row.Output),
				formatRate(row.CacheRead), formatRate(row.CacheWrite), store.Path())
			return nil
		},
	}
	setCmd.Flags().String("model", "", "Model name (required)")
	setCmd.Flags().Float64("input", 0, "USD per 1M input tokens (required)")
	setCmd.Flags().Float64("output", 0, "USD per 1M output tokens (required)")
	setCmd.Flags().Float64("cache-read", 0, "USD per 1M cache-read tokens")
	setCmd.Flags().Float64("cache-write", 0, "USD per 1M cache-write tokens")
	_ = setCmd.MarkFlagRequired("model")
	_ = setCmd.MarkFlagRequired("input")
	_ = setCmd.MarkFlagRequired("output")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Remove an override price, falling back to the built-in rate",
		RunE: func(cmd *cobra.Command, args []string) error {
			model, _ := cmd.Flags().GetString("model")
			store, err := openPriceStore(pricesPath)
			if err != nil {
				return err
			}
			row, ok := store.Get(model)
			if !ok {
				return fmt.Errorf("model %q has no price; nothing to delete", model)
			}
			// Deleting a built-in would not restore anything: the model falls
			// back to the $0 default and stops being metered.
			if !row.Explicit {
				return fmt.Errorf("%q is a built-in price, not an override; refusing to delete", model)
			}
			if err := store.Delete(model); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: override removed (%s)\n", model, store.Path())
			return nil
		},
	}
	deleteCmd.Flags().String("model", "", "Model name (required)")
	_ = deleteCmd.MarkFlagRequired("model")

	cmd.AddCommand(listCmd, setCmd, deleteCmd)
	return cmd
}

// openPriceStore loads the catalog exactly as the daemon does, so a CLI edit
// and a running proxy price from the same file.
func openPriceStore(path string) (*pricing.Store, error) {
	calc := pricing.NewCalculator(path)
	store := pricing.NewStore(path, calc)
	if err := store.LoadError(); err != nil {
		return nil, fmt.Errorf("price catalog %s is unreadable: %w", store.Path(), err)
	}
	return store, nil
}

// formatRate keeps 0.075 readable; rounding to two decimals would print a
// different rate than the one stored.
func formatRate(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
