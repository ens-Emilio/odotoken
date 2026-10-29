package importers

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

// Importer defines the contract for session file importers.
type Importer interface {
	Name() string
	Detect() bool
	Scan(since time.Time) domain.ScanResult
	IterUsages(since time.Time, out chan<- domain.RawUsage)
}

// Paths overrides a CLI's data root. A key is a CLI name, a value is a path
// that may start with "~". An empty or absent entry means "use the default".
type Paths map[string]string

// Resolve returns the configured root for cli, or def when none is set.
func (p Paths) Resolve(cli, def string) string {
	raw := strings.TrimSpace(p[cli])
	if raw == "" {
		return def
	}
	if raw == "~" {
		return homeDir()
	}
	if strings.HasPrefix(raw, "~/") {
		return filepath.Join(homeDir(), raw[2:])
	}
	return raw
}

// Registry holds all registered importers.
type Registry struct {
	importers []Importer
	// paths is the single live override map. Importers hold the same map, so a
	// change made here is visible to the next Detect/Scan without rebuilding.
	paths Paths
}

// NewRegistry creates a registry with all built-in importers.
//
// paths carries the `sessions.paths` config block. It used to be documented and
// ignored, so a user who moved a CLI's data directory still got the default.
func NewRegistry(paths Paths) *Registry {
	if paths == nil {
		paths = Paths{}
	}
	return &Registry{
		paths: paths,
		importers: []Importer{
			&ClaudeImporter{paths: paths},
			&CodexImporter{paths: paths},
			&OmpImporter{paths: paths},
			&CrushImporter{paths: paths},
			&OpencodeImporter{paths: paths},
			&PiImporter{paths: paths},
			&OmoImporter{paths: paths},
		},
	}
}

// PathFor returns the raw configured override for cli, or "" for the default.
func (r *Registry) PathFor(cli string) string {
	return strings.TrimSpace(r.paths[cli])
}

// SetPath records an override for cli. An empty path restores the default.
func (r *Registry) SetPath(cli, path string) {
	if path == "" {
		delete(r.paths, cli)
		return
	}
	r.paths[cli] = path
}

// All returns all registered importers.
func (r *Registry) All() []Importer {
	return r.importers
}

// DetectAvailable returns importers that detect their data sources.
func (r *Registry) DetectAvailable() []Importer {
	var available []Importer
	for _, imp := range r.importers {
		if imp.Detect() {
			available = append(available, imp)
		}
	}
	return available
}

// Find returns the importer with the given CLI name.
func (r *Registry) Find(name string) Importer {
	for _, imp := range r.importers {
		if imp.Name() == name {
			return imp
		}
	}
	return nil
}

// scanFiles walks root and returns the number of files with suffix plus the
// oldest and newest modification time. Session files carry no reliable global
// ordering, so mtime is the only coverage signal a scan can offer without
// parsing every file.
func scanFiles(root, suffix string, since time.Time) (files int, first, last time.Time) {
	if root == "" {
		return 0, time.Time{}, time.Time{}
	}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, suffix) {
			return nil
		}
		if !since.IsZero() && info.ModTime().Before(since) {
			return nil
		}
		files++
		mt := info.ModTime()
		if first.IsZero() || mt.Before(first) {
			first = mt
		}
		if mt.After(last) {
			last = mt
		}
		return nil
	})
	return files, first, last
}

// beforeSince reports whether an RFC3339 timestamp is strictly older than the
// cutoff. Unparseable timestamps are kept: the engine dates them from the file
// and marks them, rather than silently dropping a real event.
func beforeSince(ts string, since time.Time) bool {
	if since.IsZero() || ts == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
	}
	if err != nil {
		return false
	}
	return t.Before(since)
}
