package pricing

import (
	"os"
	"path/filepath"
	"testing"
)

// A model that is not in the catalog must resolve through the most specific
// prefix, not whichever map entry Go happens to visit first.
func TestLongestPrefixWinsForUnknownSuffix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.json")
	body := `{
		"acme-1": {"input": 30, "output": 60},
		"acme-1.2": {"input": 4, "output": 20},
		"acme-1.2-mini": {"input": 2.5, "output": 15}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write prices: %v", err)
	}
	c := NewCalculator(path)

	const model = "acme-1.2-mini-2026-01-01"
	want := ModelRates{Input: 2.5, Output: 15}
	for i := range 200 {
		if got := c.GetModelRates(model); got != want {
			t.Fatalf("iteration %d: rates = %+v, want %+v", i, got, want)
		}
	}
	if !c.HasExplicitPrice(model) {
		t.Fatalf("HasExplicitPrice(%q) = false, want true", model)
	}
}

func TestLongestPrefixWinsWithoutDeepestEntry(t *testing.T) {
	c := NewCalculator("")
	c.prices["acme-1"] = ModelRates{Input: 30, Output: 60}
	c.prices["acme-1.2"] = ModelRates{Input: 4, Output: 20}

	want := ModelRates{Input: 4, Output: 20}
	for i := range 200 {
		if got := c.GetModelRates("acme-1.2-turbo"); got != want {
			t.Fatalf("iteration %d: rates = %+v, want %+v", i, got, want)
		}
	}
}

func TestUnknownModelFallsBackToDefault(t *testing.T) {
	c := NewCalculator("")
	if got := c.GetModelRates("nothing-matches-this"); got != c.prices["default"] {
		t.Fatalf("rates = %+v, want the default row", got)
	}
	if c.HasExplicitPrice("nothing-matches-this") {
		t.Fatal("HasExplicitPrice = true for a model with no catalog price")
	}
}
