package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/importers"
)

// importJob is one in-flight import. The HTTP surface streams progress from a
// goroutine, so it needs somewhere to keep the latest state between polls.
type importJob struct {
	mu       sync.Mutex
	report   importers.Report
	progress importers.Progress
	done     bool
	err      error
	cancel   context.CancelFunc
}

func (j *importJob) snapshot() (importers.Report, importers.Progress, bool, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.report, j.progress, j.done, j.err
}

// importJobs is process-global because the daemon runs a single import at a
// time; concurrent imports would contend for the same SQLite write lock.
var (
	importJobsMu sync.Mutex
	importJobs   = map[string]*importJob{}
)

func registerImportRoutes(r chi.Router, database *db.Database, engine *importers.Engine, configPath string) {
	// GET /api/import/sources — what exists on this machine, with the last
	// import recorded per CLI so the UI can say "already imported".
	r.Get("/import/sources", func(w http.ResponseWriter, req *http.Request) {
		since, err := parseSince(req.URL.Query().Get("since"))
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.generic", err.Error())
			return
		}
		sources := engine.Discover(since)
		importers.SortSources(sources)

		last, _ := database.LastImportByCLI()
		for i := range sources {
			if run, ok := last[sources[i].CLI]; ok {
				sources[i].LastImported = run.FinishedAt
				sources[i].LastInserted = run.Inserted
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"sources": sources})
	})

	// POST /api/import/run — start an import. Returns a job id to poll.
	//
	// A dry run is the default: importing months of history by accident is not
	// something the user can undo from the dashboard, so it must be explicit.
	r.Post("/import/run", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			CLIs   []string `json:"clis"`
			Since  string   `json:"since"`
			DryRun *bool    `json:"dry_run"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<16)).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.request.invalidJson", "invalid JSON")
			return
		}

		since, err := parseSince(body.Since)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.generic", err.Error())
			return
		}

		dryRun := true
		if body.DryRun != nil {
			dryRun = *body.DryRun
		}
		opts := importers.Options{CLIs: body.CLIs, Since: since, DryRun: dryRun}
		if err := importers.ValidateOptions(opts, engine.Registry); err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.generic", err.Error())
			return
		}

		importJobsMu.Lock()
		for _, job := range importJobs {
			if _, _, done, _ := job.snapshot(); !done {
				importJobsMu.Unlock()
				writeAPIError(w, http.StatusConflict, "error.import.alreadyRunning", "an import is already running")
				return
			}
		}
		// Finished jobs are kept only long enough for the poller that started
		// them to read the result; without this the map grows for the life of
		// the process, each entry holding a full report.
		var newestFinished string
		for id, job := range importJobs {
			if _, _, done, _ := job.snapshot(); !done {
				continue
			}
			if id > newestFinished {
				newestFinished = id
			}
		}
		for id := range importJobs {
			if id != newestFinished {
				delete(importJobs, id)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		id := db.NewImportRunID(time.Now())
		job := &importJob{cancel: cancel}
		importJobs[id] = job
		importJobsMu.Unlock()

		go func() {
			report, runErr := engine.Run(ctx, opts, func(p importers.Progress) {
				job.mu.Lock()
				job.progress = p
				job.mu.Unlock()
			})
			if !dryRun {
				_ = database.RecordImport(opts.CLIs, report)
			}
			job.mu.Lock()
			job.report = report
			job.done = true
			job.err = runErr
			job.mu.Unlock()
			cancel()
		}()

		writeJSON(w, http.StatusAccepted, map[string]interface{}{"job_id": id, "dry_run": dryRun})
	})

	// GET /api/import/status — poll one job.
	r.Get("/import/status", func(w http.ResponseWriter, req *http.Request) {
		id := req.URL.Query().Get("job_id")
		importJobsMu.Lock()
		job := importJobs[id]
		importJobsMu.Unlock()
		if job == nil {
			writeAPIError(w, http.StatusNotFound, "error.import.unknownJob", "unknown job")
			return
		}
		report, progress, done, err := job.snapshot()
		resp := map[string]interface{}{
			"job_id":   id,
			"done":     done,
			"progress": progress,
			"report":   report,
		}
		if err != nil {
			resp["error"] = err.Error()
		}
		writeJSON(w, http.StatusOK, resp)
	})

	// POST /api/import/cancel — stop a running job.
	r.Post("/import/cancel", func(w http.ResponseWriter, req *http.Request) {
		id := req.URL.Query().Get("job_id")
		importJobsMu.Lock()
		job := importJobs[id]
		importJobsMu.Unlock()
		if job == nil {
			writeAPIError(w, http.StatusNotFound, "error.import.unknownJob", "unknown job")
			return
		}
		job.cancel()
		writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
	})

	// GET /api/import/history — recent runs.
	r.Get("/import/history", func(w http.ResponseWriter, req *http.Request) {
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		runs, err := database.RecentImportRuns(limit)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.generic", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"runs": runs})
	})

	// POST /api/import/path — point one CLI at a different data root.
	//
	// The saved value is written to sessions.paths and applied to the running
	// engine immediately, so Rescan shows the new root without a restart.
	r.Post("/import/path", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			CLI  string `json:"cli"`
			Path string `json:"path"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<14)).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.request.invalidJson", "invalid JSON")
			return
		}

		cli := strings.TrimSpace(body.CLI)
		if engine.Registry.Find(cli) == nil {
			writeAPIError(w, http.StatusBadRequest, "error.import.unknownCli", fmt.Sprintf("unknown CLI %q", cli))
			return
		}
		path := strings.TrimSpace(body.Path)
		if path != "" && !filepath.IsAbs(path) && path != "~" && !strings.HasPrefix(path, "~/") {
			writeAPIError(w, http.StatusBadRequest, "error.import.pathAbsolute", "use an absolute path or one starting with ~/")
			return
		}

		if configPath != "" {
			if err := persistSessionPath(configPath, cli, path); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": fmt.Sprintf("persist path: %v", err),
				})
				return
			}
		}
		engine.SetPath(cli, path)

		resolved := sourcePath(engine, cli)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cli":         cli,
			"override":    engine.Registry.PathFor(cli),
			"path":        resolved,
			"path_exists": pathExistsOnDisk(resolved),
		})
	})
}

// sourcePath is the root a CLI would read right now.
func sourcePath(engine *importers.Engine, cli string) string {
	if imp := engine.Registry.Find(cli); imp != nil {
		return imp.Scan(time.Time{}).Path
	}
	return ""
}

func pathExistsOnDisk(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func parseSince(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
