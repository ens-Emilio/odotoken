package importers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

// Options selects what an import run reads.
type Options struct {
	// CLIs limits the run to these importers. Empty means every detected CLI.
	CLIs []string
	// Since drops events older than this instant. Zero means no cutoff.
	Since time.Time
	// DryRun parses and prices without writing anything.
	DryRun bool
}

// Source is the discovery view of one CLI, shown before the user picks.
type Source struct {
	CLI       string `json:"cli"`
	Available bool   `json:"available"`
	// Path is the root actually read: the override when one is configured,
	// otherwise the CLI's default location.
	Path          string `json:"path"`
	DefaultPath   string `json:"default_path"`
	Override      string `json:"override,omitempty"`
	PathExists    bool   `json:"path_exists"`
	SessionFiles  int    `json:"session_files"`
	FirstModified string `json:"first_modified,omitempty"`
	LastModified  string `json:"last_modified,omitempty"`
	LastImported  string `json:"last_imported,omitempty"`
	LastInserted  int    `json:"last_inserted,omitempty"`
}

// CLIReport is the outcome for a single CLI.
type CLIReport struct {
	CLI        string  `json:"cli"`
	Scanned    int     `json:"scanned"`
	Inserted   int     `json:"inserted"`
	Duplicates int     `json:"duplicates"`
	Tokens     int     `json:"tokens"`
	CostUSD    float64 `json:"cost_usd"`
	// PricedByCatalog counts rows whose cost came from prices.json,
	// PricedByProvider those that carried their own cost, and Unpriced those
	// with neither. An import that silently reports $0 for unpriced models is
	// the failure this exists to prevent.
	PricedByCatalog  int `json:"priced_by_catalog"`
	PricedByProvider int `json:"priced_by_provider"`
	Unpriced         int `json:"unpriced"`
	Errors           int `json:"errors"`
	// Cancelled marks the CLI whose remaining files were abandoned when the
	// run was stopped. Its counts are what landed before the stop.
	Cancelled bool   `json:"cancelled,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Report is the complete result of a run.
type Report struct {
	StartedAt  time.Time   `json:"started_at"`
	FinishedAt time.Time   `json:"finished_at"`
	DryRun     bool        `json:"dry_run"`
	Since      string      `json:"since,omitempty"`
	CLIs       []CLIReport `json:"clis"`
	Inserted   int         `json:"inserted"`
	Duplicates int         `json:"duplicates"`
	Tokens     int         `json:"tokens"`
	CostUSD    float64     `json:"cost_usd"`
	Unpriced   int         `json:"unpriced"`
	Errors     int         `json:"errors"`
	// Status is the terminal state a caller should show: done, partial, or
	// cancelled. A cancelled run keeps the rows it already wrote, so it is
	// neither a success nor a failure.
	Status string `json:"status"`
}

// Progress is emitted while a run is in flight.
type Progress struct {
	CLI      string `json:"cli"`
	Scanned  int    `json:"scanned"`
	Inserted int    `json:"inserted"`
	Index    int    `json:"index"`
	Total    int    `json:"total"`
}

// Sink is where priced events go. The CLI and the daemon both implement it, so
// neither owns import semantics.
type Sink interface {
	WriteEvents(events []domain.TokenEvent) (inserted, duplicates int, err error)
}

// Engine runs imports. It owns parsing, timestamp normalization and pricing so
// the CLI and the HTTP surface cannot disagree about what an import produced.
type Engine struct {
	Registry *Registry
	Pricing  *pricing.Calculator
	Sink     Sink
	// BatchSize bounds the WAL write lock. An unbounded batch would hold it for
	// the whole run and stall the proxy.
	BatchSize int
}

// NewEngine builds an engine over the given paths config.
func NewEngine(paths Paths, calc *pricing.Calculator, sink Sink) *Engine {
	return &Engine{
		Registry:  NewRegistry(paths),
		Pricing:   calc,
		Sink:      sink,
		BatchSize: 500,
	}
}

// Discover reports every registered CLI, detected or not, so the UI can explain
// what is missing instead of showing an empty list.
func (e *Engine) Discover(since time.Time) []Source {
	var out []Source
	for _, imp := range e.Registry.All() {
		// Scan always reports the root it read, detected or not, so the UI can
		// show where a missing source was looked for.
		scan := imp.Scan(since)
		src := Source{
			CLI:         imp.Name(),
			Path:        scan.Path,
			DefaultPath: DefaultPath(imp.Name()),
			Override:    e.Registry.PathFor(imp.Name()),
			PathExists:  pathExists(scan.Path),
		}
		if imp.Detect() {
			src.Available = scan.Available
			src.SessionFiles = scan.SessionFiles
			if !scan.FirstModified.IsZero() {
				src.FirstModified = scan.FirstModified.UTC().Format(time.RFC3339)
			}
			if !scan.LastModified.IsZero() {
				src.LastModified = scan.LastModified.UTC().Format(time.RFC3339)
			}
		}
		out = append(out, src)
	}
	return out
}

// pathExists answers the only question the UI needs about a configured root:
// did anything answer at that path. It never opens the file.
func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// SetPath changes the source root used by future discovery and imports.
// An empty path removes the override and restores the CLI's default.
func (e *Engine) SetPath(cli, path string) {
	e.Registry.SetPath(cli, path)
}

// DefaultPath is the root a CLI uses when no override is configured.
func DefaultPath(cli string) string {
	if imp := NewRegistry(Paths{}).Find(cli); imp != nil {
		return imp.Scan(time.Time{}).Path
	}
	return ""
}

// Run imports the selected CLIs and returns a report.
//
// A failure in one CLI does not abort the others: partial success is the normal
// case when one CLI's data directory is locked by its own process.
func (e *Engine) Run(ctx context.Context, opts Options, onProgress func(Progress)) (Report, error) {
	started := time.Now().UTC()
	report := Report{StartedAt: started, DryRun: opts.DryRun}
	if !opts.Since.IsZero() {
		report.Since = opts.Since.UTC().Format(time.RFC3339)
	}

	selected := e.selected(opts.CLIs)
	report.CLIs = make([]CLIReport, 0, len(selected))
	cancelled := false
	for i, imp := range selected {
		if ctx.Err() != nil {
			cancelled = true
			break
		}
		cliReport := e.runOne(ctx, imp, opts, Progress{Index: i + 1, Total: len(selected)}, onProgress)
		report.CLIs = append(report.CLIs, cliReport)
		report.Inserted += cliReport.Inserted
		report.Duplicates += cliReport.Duplicates
		report.Tokens += cliReport.Tokens
		report.CostUSD += cliReport.CostUSD
		report.Unpriced += cliReport.Unpriced
		report.Errors += cliReport.Errors
		// runOne returns the moment the context is cancelled, so a cancelled
		// CLI is the last one in the report and stops the loop.
		if cliReport.Cancelled {
			cancelled = true
			break
		}
	}

	report.FinishedAt = time.Now().UTC()
	report.Status = ReportStatus(report, cancelled)
	return report, nil
}

func (e *Engine) selected(names []string) []Importer {
	if len(names) == 0 {
		return e.Registry.All()
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[strings.TrimSpace(n)] = true
	}
	var out []Importer
	for _, imp := range e.Registry.All() {
		if want[imp.Name()] {
			out = append(out, imp)
		}
	}
	return out
}

func (e *Engine) runOne(ctx context.Context, imp Importer, opts Options, progress Progress, onProgress func(Progress)) CLIReport {
	rep := CLIReport{CLI: imp.Name()}
	if !imp.Detect() {
		rep.Error = "not detected on this machine"
		return rep
	}

	out := make(chan domain.RawUsage, 256)
	go imp.IterUsages(opts.Since, out)

	batchSize := e.BatchSize
	if batchSize <= 0 {
		batchSize = 500
	}
	batch := make([]domain.TokenEvent, 0, batchSize)
	progress.CLI = imp.Name()
	lastReport := time.Time{}

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if opts.DryRun {
			for range batch {
				rep.Inserted++
			}
		} else if e.Sink != nil {
			inserted, duplicates, err := e.Sink.WriteEvents(batch)
			if err != nil {
				rep.Errors++
				rep.Error = err.Error()
			} else {
				rep.Inserted += inserted
				rep.Duplicates += duplicates
			}
		}
		batch = batch[:0]
	}

	for {
		// Checked before the select: a buffered channel with pending usages
		// would otherwise let cancellation lose the race and read on.
		if ctx.Err() != nil {
			flush()
			rep.Cancelled = true
			return rep
		}
		select {
		case <-ctx.Done():
			flush()
			// A cancelled CLI is not an error: whatever landed is real, and the
			// run stops here rather than pretending the remaining files failed.
			rep.Cancelled = true
			return rep
		case usage, ok := <-out:
			if !ok {
				flush()
				progress.Scanned = rep.Scanned
				progress.Inserted = rep.Inserted
				if onProgress != nil {
					onProgress(progress)
				}
				return rep
			}
			event, priceKind := e.toEvent(usage)
			rep.Scanned++
			rep.Tokens += event.TotalTokens
			rep.CostUSD += event.CostUSD
			switch priceKind {
			case priceCatalog:
				rep.PricedByCatalog++
			case priceProvider:
				rep.PricedByProvider++
			default:
				rep.Unpriced++
			}
			batch = append(batch, event)
			if len(batch) >= batchSize {
				flush()
			}
			progress.Scanned = rep.Scanned
			progress.Inserted = rep.Inserted
			// Progress is for humans: emit at most a few times a second so a
			// large import does not spend its time formatting numbers.
			if onProgress != nil && time.Since(lastReport) > 250*time.Millisecond {
				lastReport = time.Now()
				onProgress(progress)
			}
		}
	}
}

type priceKind int

const (
	priceMissing priceKind = iota
	priceProvider
	priceCatalog
)

// toEvent turns one raw usage row into a storable event.
//
// Cost precedence is deliberate: the catalog wins when it knows the model,
// because a CLI's own cost field is frequently absent or stale; the provider's
// number is the fallback; zero is the last resort and is marked, never passed
// off as a real measurement.
func (e *Engine) toEvent(usage domain.RawUsage) (domain.TokenEvent, priceKind) {
	ts, assumed := normalizeTimestamp(usage.Timestamp, usage.SessionFile)

	cost := 0.0
	kind := priceMissing
	if e.Pricing != nil && usage.Model != "" && e.Pricing.HasExplicitPrice(usage.Model) {
		cost = e.Pricing.CalculateCost(usage.Model, usage.PromptTokens, usage.CompletionTokens, usage.CacheReadTokens, usage.CacheWriteTokens)
		kind = priceCatalog
	} else if usage.RawCost > 0 {
		cost = usage.RawCost
		kind = priceProvider
	}

	metadata := map[string]interface{}{}
	switch kind {
	case priceCatalog:
		metadata["cost_source"] = "catalog"
	case priceProvider:
		metadata["cost_source"] = "provider"
	default:
		metadata["cost_source"] = "missing"
		metadata["price_missing"] = true
	}
	if assumed {
		metadata["assumed_timestamp"] = true
	}
	metadataJSON, _ := json.Marshal(metadata)

	project := usage.Project
	if project == "" {
		project = "default"
	}

	return domain.TokenEvent{
		Timestamp:        ts,
		Provider:         usage.Provider,
		Model:            usage.Model,
		ClientApp:        usage.CLI,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.PromptTokens + usage.CompletionTokens,
		CacheReadTokens:  usage.CacheReadTokens,
		CacheWriteTokens: usage.CacheWriteTokens,
		CostUSD:          cost,
		ProjectID:        project,
		Metadata:         string(metadataJSON),
		Source:           "import:" + usage.CLI,
		SessionID:        usage.SessionID,
		SessionFile:      usage.SessionFile,
		EntryID:          usage.EntryID,
	}, kind
}

// normalizeTimestamp parses an importer timestamp.
//
// A session file that records an unparseable time used to be stamped with
// time.Now(), which silently moved months of history into today's budget. The
// file's modification time is the honest fallback, and the caller marks it.
func normalizeTimestamp(raw, sessionFile string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), false
		}
	}
	if sessionFile != "" {
		if info, err := os.Stat(sessionFile); err == nil {
			return info.ModTime().UTC(), true
		}
	}
	return time.Time{}, true
}

// ReportStatus names the terminal state of a run once every CLI has reported.
// It is the single definition both the database record and the UI read, so a
// stopped import cannot be filed as a success.
func ReportStatus(report Report, cancelled bool) string {
	if !cancelled {
		for _, c := range report.CLIs {
			if c.Cancelled {
				cancelled = true
				break
			}
		}
	}
	if cancelled {
		return "cancelled"
	}
	if report.Errors > 0 {
		return "partial"
	}
	return "done"
}

// SortSources orders discovery output deterministically for the UI.
func SortSources(sources []Source) {
	sort.SliceStable(sources, func(i, j int) bool { return sources[i].CLI < sources[j].CLI })
}

// ValidateOptions rejects selections that cannot produce anything.
func ValidateOptions(opts Options, reg *Registry) error {
	if len(opts.CLIs) == 0 {
		return nil
	}
	for _, name := range opts.CLIs {
		if reg.Find(strings.TrimSpace(name)) == nil {
			return fmt.Errorf("unknown CLI %q", name)
		}
	}
	return nil
}
