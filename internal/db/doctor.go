package db

// TableCounts is the row count per table `odotoken doctor` reports, so an
// operator can see whether the schema is populated without opening SQLite.
type TableCounts struct {
	TokenEvents  int64 `json:"token_events"`
	GuardEvents  int64 `json:"guard_events"`
	ActiveRuns   int64 `json:"active_runs"`
	ImportRuns   int64 `json:"import_runs"`
	AlertPending int64 `json:"alert_pending"`
}

// TableCounts counts rows in every table the daemon writes. It fails if the
// schema is missing, which is exactly the signal doctor needs.
func (d *Database) TableCounts() (TableCounts, error) {
	var c TableCounts
	rows := []struct {
		table string
		into  *int64
	}{
		{"token_events", &c.TokenEvents},
		{"guard_events", &c.GuardEvents},
		{"active_runs", &c.ActiveRuns},
		{"import_runs", &c.ImportRuns},
	}
	for _, row := range rows {
		if err := d.db.QueryRow(`SELECT COUNT(*) FROM ` + row.table).Scan(row.into); err != nil {
			return c, err
		}
	}
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM alert_outbox WHERE delivered_at IS NULL`).Scan(&c.AlertPending); err != nil {
		return c, err
	}
	return c, nil
}
