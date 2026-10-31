package pricing

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestCheckDrift_NoDrift(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	v := NewPriceVerifier(logger)

	// Divergência de 2% (dentro do limite de 5%)
	drifted := v.CheckDrift("gpt-4o", "openai", 1.00, 1.02)

	if drifted {
		t.Error("esperava false para divergência <= 5%, got true")
	}

	output := buf.String()
	if output != "" {
		t.Errorf("não esperava log quando não há divergência, got: %s", output)
	}
}

func TestCheckDrift_DriftDetected(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	v := NewPriceVerifier(logger)

	// Divergência de 10% (acima do limite de 5%)
	drifted := v.CheckDrift("gpt-4o", "openai", 1.00, 1.10)

	if !drifted {
		t.Error("esperava true para divergência > 5%, got false")
	}

	output := buf.String()
	if !strings.Contains(output, "price drift detected") {
		t.Errorf("esperava log WARN com 'price drift detected', got: %s", output)
	}
	if !strings.Contains(output, `model=gpt-4o`) {
		t.Errorf("esperava label model no log, got: %s", output)
	}
	if !strings.Contains(output, `provider=openai`) {
		t.Errorf("esperava label provider no log, got: %s", output)
	}
	if !strings.Contains(output, "calculated_cost=1") {
		t.Errorf("esperava calculated_cost no log, got: %s", output)
	}
	if !strings.Contains(output, "reported_cost=1.1") {
		t.Errorf("esperava reported_cost no log, got: %s", output)
	}
}

func TestCheckDrift_ZeroReportedCost(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	v := NewPriceVerifier(logger)

	// Custo reportado zero (não disponível)
	drifted := v.CheckDrift("gpt-4o", "openai", 1.00, 0)

	if drifted {
		t.Error("esperava false quando custo reportado é zero, got true")
	}

	output := buf.String()
	if output != "" {
		t.Errorf("não esperava log quando custo reportado é zero, got: %s", output)
	}
}

func TestCheckDrift_ZeroCalculatedCost(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	v := NewPriceVerifier(logger)

	// Custo calculado zero
	drifted := v.CheckDrift("gpt-4o", "openai", 0, 1.00)

	if drifted {
		t.Error("esperava false quando custo calculado é zero, got true")
	}

	output := buf.String()
	if output != "" {
		t.Errorf("não esperava log quando custo calculado é zero, got: %s", output)
	}
}

func TestCheckDrift_ExactMatch(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	v := NewPriceVerifier(logger)

	// Custos idênticos
	drifted := v.CheckDrift("claude-3-7-sonnet", "anthropic", 5.00, 5.00)

	if drifted {
		t.Error("esperava false para custos idênticos, got true")
	}

	output := buf.String()
	if output != "" {
		t.Errorf("não esperava log para custos idênticos, got: %s", output)
	}
}
