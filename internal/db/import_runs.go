package db

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
	"github.com/ens-Emilio/odotoken/internal/importers"
)

// WriteEvents satisfies importers.Sink so the import engine does not need to
// know about SQLite.
func (d *Database) WriteEvents(events []domain.TokenEvent) (int, int, error) {
	result, err := d.InsertEvents(events)
	return result.Inserted, result.Duplicates, err
}

// ImportRun is one recorded import, kept so the UI can show what happened last
// time instead of re-deriving it from the events it wrote.
type ImportRun struct {
	ID         string          `json:"id"`
	StartedAt  string          `json:"started_at"`
	FinishedAt string          `json:"finished_at,omitempty"`
	Status     string          `json:"status"`
	DryRun     bool            `json:"dry_run"`
	CLIs       string          `json:"clis"`
	Since      string          `json:"since,omitempty"`
	Inserted   int             `json:"inserted"`
	Duplicates int             `json:"duplicates"`
	Tokens     int             `json:"tokens"`
	CostUSD    float64         `json:"cost_usd"`
	Error      string          `json:"error,omitempty"`
	Report     json.RawMessage `json:"report,omitempty"`
}

// SaveImportRun inserts or replaces an import run record.
func (d *Database) SaveImportRun(run ImportRun) error {
	report := run.Report
	if len(report) == 0 {
		report = json.RawMessage("{}")
	}
	_, err := d.db.Exec(`INSERT OR REPLACE INTO import_runs
		(id, started_at, finished_at, status, dry_run, clis, since,
		 inserted, duplicates, tokens, cost_usd, error, report)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.StartedAt, run.FinishedAt, run.Status, boolInt(run.DryRun),
		run.CLIs, run.Since, run.Inserted, run.Duplicates, run.Tokens,
		run.CostUSD, run.Error, string(report))
	return err
}

// LastImportByCLI returns the most recent finished import per CLI name, so a
// discovery screen can say "claude: imported 3 days ago" without the caller
// scanning run history itself.
func (d *Database) LastImportByCLI() (map[string]ImportRun, error) {
	rows, err := d.db.Query(`SELECT id, started_at, finished_at, status, dry_run,
		clis, since, inserted, duplicates, tokens, cost_usd, error
		FROM import_runs ORDER BY started_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]ImportRun)
	for rows.Next() {
		var r ImportRun
		var finished sql.NullString
		var dryRun int
		if err := rows.Scan(&r.ID, &r.StartedAt, &finished, &r.Status, &dryRun,
			&r.CLIs, &r.Since, &r.Inserted, &r.Duplicates, &r.Tokens,
			&r.CostUSD, &r.Error); err != nil {
			return nil, err
		}
		r.FinishedAt = finished.String
		r.DryRun = dryRun != 0
		for _, cli := range splitCLIs(r.CLIs) {
			if _, seen := out[cli]; !seen {
				out[cli] = r
			}
		}
	}
	return out, rows.Err()
}

// RecentImportRuns returns the newest runs for a history list.
func (d *Database) RecentImportRuns(limit int) ([]ImportRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.Query(`SELECT id, started_at, finished_at, status, dry_run,
		clis, since, inserted, duplicates, tokens, cost_usd, error, report
		FROM import_runs ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ImportRun{}
	for rows.Next() {
		var r ImportRun
		var finished sql.NullString
		var dryRun int
		var report string
		if err := rows.Scan(&r.ID, &r.StartedAt, &finished, &r.Status, &dryRun,
			&r.CLIs, &r.Since, &r.Inserted, &r.Duplicates, &r.Tokens,
			&r.CostUSD, &r.Error, &report); err != nil {
			return nil, err
		}
		r.FinishedAt = finished.String
		r.DryRun = dryRun != 0
		r.Report = json.RawMessage(report)
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecordImport stores a finished run. Both the CLI and the HTTP surface call
// this, so a UI import and a terminal import leave the same trace.
func (d *Database) RecordImport(clis []string, report importers.Report) error {
	raw, _ := json.Marshal(report)
	// The engine already decided what happened; re-deriving it here is how a
	// cancelled run used to be filed as a success.
	status := report.Status
	if status == "" {
		status = importers.ReportStatus(report, false)
	}
	finished := ""
	if !report.FinishedAt.IsZero() {
		finished = report.FinishedAt.Format(time.RFC3339)
	}
	firstErr := ""
	for _, c := range report.CLIs {
		if c.Error != "" {
			firstErr = c.Error
			break
		}
	}
	return d.SaveImportRun(ImportRun{
		ID:         NewImportRunID(report.StartedAt),
		StartedAt:  report.StartedAt.Format(time.RFC3339),
		FinishedAt: finished,
		Status:     status,
		DryRun:     report.DryRun,
		CLIs:       strings.Join(clis, ","),
		Since:      report.Since,
		Inserted:   report.Inserted,
		Duplicates: report.Duplicates,
		Tokens:     report.Tokens,
		CostUSD:    report.CostUSD,
		Error:      firstErr,
		Report:     raw,
	})
}

// NewImportRunID is time-ordered so "latest run" needs no extra column.
func NewImportRunID(t time.Time) string {
	return t.UTC().Format("20060102T150405.000000Z")
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func splitCLIs(csv string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(csv); i++ {
		if i == len(csv) || csv[i] == ',' {
			if part := csv[start:i]; part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}
