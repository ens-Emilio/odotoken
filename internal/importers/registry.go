package importers

import (
	"github.com/user/odotoken/internal/domain"
)

// Importer defines the contract for session file importers.
type Importer interface {
	Name() string
	Detect() bool
	Scan(since string) domain.ScanResult
	IterUsages(since string, out chan<- domain.RawUsage)
}

// Registry holds all registered importers.
type Registry struct {
	importers []Importer
}

// NewRegistry creates a registry with all built-in importers.
func NewRegistry() *Registry {
	return &Registry{
		importers: []Importer{
			&ClaudeImporter{},
			&CodexImporter{},
			&OmpImporter{},
			&CrushImporter{},
			&OpencodeImporter{},
			&PiImporter{},
		},
	}
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