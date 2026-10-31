package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ens-Emilio/odotoken/internal/analytics"
	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
)

// exportMeta is the machine-readable summary printed with --json when the rows
// themselves go to a file. A caller that redirects stdout wants to know how
// many rows landed and whether the export was cut short.
type exportMeta struct {
	Path      string `json:"path"`
	Format    string `json:"format"`
	Rows      int    `json:"rows"`
	Truncated bool   `json:"truncated"`
	Since     string `json:"since,omitempty"`
	Until     string `json:"until,omitempty"`
}

func newExportCmd() *cobra.Command {
	var (
		format     string
		since      string
		until      string
		project    string
		source     string
		tag        string
		limit      int
		out        string
		jsonMeta   bool
		dbPath     string
		configPath string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export token events as CSV or JSON without the daemon",
		Long: `Export token events straight from the database.

The rows come from the same query and the same CSV contract as
GET /api/events.csv, so an export and the dashboard cannot disagree.

Examples:
  odotoken export --out events.csv
  odotoken export --format json --project acme
  odotoken export --since 2024-01-01T00:00:00Z --out - > events.csv
  odotoken export --out events.csv --json   # write rows, print a summary`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "csv" && format != "json" {
				return fmt.Errorf("invalid --format %q: use csv or json", format)
			}
			sinceTime, err := parseExportBound("since", since)
			if err != nil {
				return err
			}
			untilTime, err := parseExportBound("until", until)
			if err != nil {
				return err
			}
			if sinceTime != "" && untilTime != "" && sinceTime > untilTime {
				return fmt.Errorf("--since %s is after --until %s", sinceTime, untilTime)
			}

			// config is only read for parity with the other commands; the
			// export needs nothing from it, but a broken config should not be
			// silently ignored either.
			if _, err := config.Load(configPath); err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			database, err := db.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			rows, truncated, err := database.EventsForExport(sinceTime, untilTime, project, source, tag, limit)
			if err != nil {
				return fmt.Errorf("export events: %w", err)
			}

			writer, closeWriter, err := openExportTarget(cmd, out)
			if err != nil {
				return err
			}
			defer closeWriter()

			switch format {
			case "csv":
				err = analytics.WriteEventsCSV(writer, rows)
			case "json":
				err = writeEventJSON(writer, rows)
			}
			if err != nil {
				return err
			}

			if jsonMeta {
				if out == "-" {
					return fmt.Errorf("--json writes the metadata to stdout, so --out must be a file")
				}
				return writeJSONTo(cmd.OutOrStdout(), exportMeta{
					Path: out, Format: format, Rows: len(rows),
					Truncated: truncated, Since: sinceTime, Until: untilTime,
				})
			}
			if truncated {
				fmt.Fprintf(cmd.ErrOrStderr(), "export truncated at %d rows; raise --limit or narrow the range\n", len(rows))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "csv", "Output format: csv or json")
	cmd.Flags().StringVar(&since, "since", "", "Only events at or after this RFC3339 timestamp")
	cmd.Flags().StringVar(&until, "until", "", "Only events at or before this RFC3339 timestamp")
	cmd.Flags().StringVar(&project, "project", "", "Only this project_id")
	cmd.Flags().StringVar(&source, "source", "", "Only this source (proxy, import:claude, ...)")
	cmd.Flags().StringVar(&tag, "tag", "", "Only events carrying this tag")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (default 10000, capped at 100000)")
	cmd.Flags().StringVar(&out, "out", "-", "Output path, or - for stdout")
	cmd.Flags().BoolVar(&jsonMeta, "json", false, "Print a JSON summary to stdout when --out is a file")
	cmd.Flags().StringVar(&dbPath, "db", "odotoken.db", "database file path")
	cmd.Flags().StringVar(&configPath, "config", "config.json", "config file path")
	return cmd
}

// parseExportBound normalizes a timestamp the same way the HTTP export does:
// SQLite stores RFC3339 text, and a value with an offset or sub-second
// precision would compare wrong as a raw string.
func parseExportBound(name, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", fmt.Errorf("invalid --%s %q: use RFC3339, e.g. 2024-01-01T00:00:00Z", name, value)
	}
	return t.UTC().Format(time.RFC3339), nil
}

func openExportTarget(cmd *cobra.Command, out string) (io.Writer, func(), error) {
	if out == "-" || out == "" {
		return cmd.OutOrStdout(), func() {}, nil
	}
	file, err := os.Create(out)
	if err != nil {
		return nil, nil, fmt.Errorf("create %s: %w", out, err)
	}
	return file, func() { _ = file.Close() }, nil
}

func writeEventJSON(w io.Writer, rows []map[string]interface{}) error {
	if rows == nil {
		rows = []map[string]interface{}{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func writeJSONTo(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
