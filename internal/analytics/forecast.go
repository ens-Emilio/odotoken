package analytics

import (
	"math"
	"time"

	"github.com/ens-Emilio/odotoken/internal/db"
)

// ForecastPoint is one projected day.
type ForecastPoint struct {
	Day     string  `json:"day"`
	CostUSD float64 `json:"cost_usd"`
}

// Forecast projects the next days from a least-squares fit over the daily
// series. Two points define a line; one point is carried forward flat; no
// points produce an empty projection instead of an invented number.
func Forecast(daily []db.DailyUsage, days int) []ForecastPoint {
	if days <= 0 {
		days = 7
	}
	n := len(daily)
	var slope, intercept float64
	switch {
	case n >= 2:
		mx, my := 0.0, 0.0
		for i, d := range daily {
			mx += float64(i)
			my += d.CostUSD
		}
		mx /= float64(n)
		my /= float64(n)
		var denom, num float64
		for i, d := range daily {
			dx := float64(i) - mx
			denom += dx * dx
			num += dx * (d.CostUSD - my)
		}
		if denom > 0 {
			slope = num / denom
			intercept = my - slope*mx
		}
	case n == 1:
		intercept = daily[0].CostUSD
	}
	base := time.Now().UTC()
	if n > 0 {
		if parsed, err := time.Parse("2006-01-02", daily[n-1].Date); err == nil {
			base = parsed
		}
	}
	out := make([]ForecastPoint, 0, days)
	for i := range days {
		out = append(out, ForecastPoint{
			Day:     base.AddDate(0, 0, i+1).Format("2006-01-02"),
			CostUSD: round(math.Max(0, slope*float64(n+i)+intercept), 6),
		})
	}
	return out
}
