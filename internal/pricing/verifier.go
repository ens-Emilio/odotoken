package pricing

import (
	"log/slog"
	"math"
)

// PriceVerifier verifica divergências entre custo calculado e custo reportado pelo provedor.
type PriceVerifier struct {
	logger *slog.Logger
}

// NewPriceVerifier cria uma nova instância de PriceVerifier.
func NewPriceVerifier(logger *slog.Logger) *PriceVerifier {
	return &PriceVerifier{logger: logger}
}

// CheckDrift compara o custo calculado com o custo reportado pelo provedor.
// Se a divergência for maior que 5%, emite log WARN estruturado e retorna true.
// Retorna false se não houver divergência significativa ou se o custo reportado for zero/não disponível.
// Quando houver divergência, incrementa o contador odotoken_price_drift_total com labels model e provider
// (integração com métricas deve ser feita pelo chamador, pois o sistema de métricas está fora deste módulo).
func (v *PriceVerifier) CheckDrift(model, provider string, calculatedCost, reportedCost float64) bool {
	if reportedCost <= 0 || calculatedCost <= 0 {
		return false
	}

	drift := math.Abs(calculatedCost-reportedCost) / calculatedCost * 100
	if drift > 5.0 {
		v.logger.Warn("price drift detected",
			"model", model,
			"provider", provider,
			"calculated_cost", calculatedCost,
			"reported_cost", reportedCost,
			"drift_percent", drift,
		)
		// Nota: incrementar odotoken_price_drift_total{model, provider} deve ser feito
		// pelo código que integra com o sistema de métricas Prometheus do projeto.
		return true
	}

	return false
}
