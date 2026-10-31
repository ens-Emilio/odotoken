package pricing

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ErrPriceNotFound reports a model that has no editable override. A delete is
// not a way to remove a built-in rate.
var ErrPriceNotFound = errors.New("price not found")

// PriceRow is one model as the price editor sees it. Explicit marks a rate that
// comes from prices.json; the rest are built-in fallbacks.
type PriceRow struct {
	Model      string  `json:"model"`
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Explicit   bool    `json:"explicit"`
}

// Store edits prices.json without ever holding a partially written file. The
// proxy reads through Calculator, so a write swaps the whole catalog atomically
// instead of leaving one request priced by the old table.
type Store struct {
	path string
	calc *Calculator

	mu        sync.Mutex
	overrides map[string]ModelRates
	loadErr   error
}

// NewStore loads the override file once. The file holds thousands of models,
// so re-reading it on every dashboard poll would cost more than the query.
func NewStore(path string, calc *Calculator) *Store {
	s := &Store{path: ResolvePriceFile(path), calc: calc, overrides: map[string]ModelRates{}}
	overrides, err := readPriceFile(s.path)
	if err != nil {
		s.loadErr = err
		return s
	}
	s.overrides = overrides
	_ = calc.ReplacePrices(overrides)
	return s
}

// ResolvePriceFile mirrors NewCalculator's lookup: a relative path missing from
// the working directory is searched next to the executable.
func ResolvePriceFile(path string) string {
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err == nil || filepath.IsAbs(path) {
		return path
	}
	if exe, err := os.Executable(); err == nil {
		for _, candidate := range []string{
			filepath.Join(filepath.Dir(exe), path),
			filepath.Join(filepath.Dir(exe), "..", path),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return path
}

// Path is the file edits are written to.
func (s *Store) Path() string { return s.path }

// LoadError reports a catalog that exists but could not be parsed. Writing over
// an unreadable file would destroy every price it still holds.
func (s *Store) LoadError() error { return s.loadErr }

// List returns the merged catalog, explicit rows first. limit <= 0 is unlimited.
func (s *Store) List(query string, limit int) []PriceRow {
	merged := s.calc.Prices()
	s.mu.Lock()
	overrides := s.overrides
	s.mu.Unlock()
	needle := strings.ToLower(strings.TrimSpace(query))
	rows := make([]PriceRow, 0, len(merged))
	for model, rates := range merged {
		if needle != "" && !strings.Contains(strings.ToLower(model), needle) {
			continue
		}
		_, explicit := overrides[model]
		rows = append(rows, PriceRow{
			Model: model, Input: rates.Input, Output: rates.Output,
			CacheRead: rates.CacheRead, CacheWrite: rates.CacheWrite,
			Explicit: explicit,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Explicit != rows[j].Explicit {
			return rows[i].Explicit
		}
		return rows[i].Model < rows[j].Model
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// Get returns one model's merged rates.
func (s *Store) Get(model string) (PriceRow, bool) {
	s.mu.Lock()
	_, explicit := s.overrides[model]
	s.mu.Unlock()
	if !explicit && !s.calc.HasExplicitPrice(model) {
		return PriceRow{}, false
	}
	rates := s.calc.GetModelRates(model)
	return PriceRow{Model: model, Input: rates.Input, Output: rates.Output,
		CacheRead: rates.CacheRead, CacheWrite: rates.CacheWrite, Explicit: explicit}, true
}

// Set writes one override and reloads the calculator. Negative or non-finite
// rates are rejected: one bad entry silently corrupts every future cost.
func (s *Store) Set(model string, rates ModelRates) (PriceRow, error) {
	model = strings.TrimSpace(model)
	if model == "" || len(model) > 200 {
		return PriceRow{}, fmt.Errorf("model name must be 1..200 characters")
	}
	if err := ValidatePrices(map[string]ModelRates{model: rates}); err != nil {
		return PriceRow{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return PriceRow{}, fmt.Errorf("catalog unreadable, refusing to overwrite: %w", s.loadErr)
	}
	next := make(map[string]ModelRates, len(s.overrides)+1)
	for k, v := range s.overrides {
		next[k] = v
	}
	next[model] = rates
	if err := SavePrices(s.path, next); err != nil {
		return PriceRow{}, err
	}
	if err := s.calc.ReplacePrices(next); err != nil {
		return PriceRow{}, err
	}
	s.overrides = next
	return PriceRow{Model: model, Input: rates.Input, Output: rates.Output,
		CacheRead: rates.CacheRead, CacheWrite: rates.CacheWrite, Explicit: true}, nil
}

// Delete removes an override, so the model falls back to its built-in rate.
func (s *Store) Delete(model string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.overrides[model]; !ok {
		return ErrPriceNotFound
	}
	next := make(map[string]ModelRates, len(s.overrides))
	for k, v := range s.overrides {
		if k != model {
			next[k] = v
		}
	}
	if err := SavePrices(s.path, next); err != nil {
		return err
	}
	if err := s.calc.ReplacePrices(next); err != nil {
		return err
	}
	s.overrides = next
	return nil
}

func readPriceFile(path string) (map[string]ModelRates, error) {
	if path == "" {
		return map[string]ModelRates{}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]ModelRates{}, nil
	}
	if err != nil {
		return nil, err
	}
	var prices map[string]ModelRates
	if err := json.Unmarshal(data, &prices); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if prices == nil {
		prices = map[string]ModelRates{}
	}
	return prices, nil
}

// ValidatePrices rejects a rate table that would make cost meaningless.
func ValidatePrices(prices map[string]ModelRates) error {
	for model, rates := range prices {
		if strings.TrimSpace(model) == "" || len(model) > 200 {
			return fmt.Errorf("invalid model name")
		}
		for name, rate := range map[string]float64{
			"input": rates.Input, "output": rates.Output,
			"cache_read": rates.CacheRead, "cache_write": rates.CacheWrite,
		} {
			if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
				return fmt.Errorf("%s rate for %q must be a finite non-negative number", name, model)
			}
			if rate > 1_000_000 {
				return fmt.Errorf("%s rate for %q exceeds 1000000 per 1M tokens", name, model)
			}
		}
	}
	return nil
}

// SavePrices writes the complete override catalog atomically and privately. A
// half-written prices.json would zero the cost of every later request.
func SavePrices(path string, prices map[string]ModelRates) error {
	if path == "" {
		return fmt.Errorf("no price file configured")
	}
	if err := ValidatePrices(prices); err != nil {
		return err
	}
	data, err := json.MarshalIndent(prices, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".prices-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
