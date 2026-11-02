package guard

import (
	"math"
	"sync"
	"sync/atomic"
)

var driftAlertsTotal atomic.Int64

// GetDriftAlertsTotal returns the cumulative count of drift alerts.
func GetDriftAlertsTotal() int64 {
	return driftAlertsTotal.Load()
}

type driftSample struct {
	preflight  int
	postflight int
}

// DriftDetector tracks token count discrepancies per model over a rolling window.
type DriftDetector struct {
	mu      sync.Mutex
	windows map[string][]driftSample
}

// NewDriftDetector creates a detector ready to record samples.
func NewDriftDetector() *DriftDetector {
	return &DriftDetector{windows: make(map[string][]driftSample)}
}

// Record adds a preflight/postflight pair for a model, keeping at most 100 samples.
// It also increments the drift alert counter when the new sample triggers 5+ consecutive alerts.
func (d *DriftDetector) Record(model string, preflight, postflight int) {
	d.mu.Lock()
	samples := d.windows[model]
	samples = append(samples, driftSample{preflight: preflight, postflight: postflight})
	if len(samples) > 100 {
		samples = samples[len(samples)-100:]
	}
	d.windows[model] = samples

	// Check alert transition only on write, not on read.
	if len(samples) >= 5 {
		consecutive := 0
		for i := len(samples) - 1; i >= 0 && i >= len(samples)-5; i-- {
			s := samples[i]
			b := s.preflight
			if b == 0 {
				b = 1
			}
			r := math.Abs(float64(s.postflight-s.preflight)) / float64(b)
			if r > 0.15 {
				consecutive++
			} else {
				break
			}
		}
		if consecutive >= 5 {
			driftAlertsTotal.Add(1)
		}
	}
	d.mu.Unlock()
}

// Check returns the latest drift ratio for a model and whether it has been
// alerting (>15% drift) for 5+ consecutive recent samples.
func (d *DriftDetector) Check(model string) (ratio float64, alerting bool) {
	d.mu.Lock()
	samples := d.windows[model]
	d.mu.Unlock()

	if len(samples) == 0 {
		return 0, false
	}

	last := samples[len(samples)-1]
	base := last.preflight
	if base == 0 {
		base = 1
	}
	ratio = math.Abs(float64(last.postflight-last.preflight)) / float64(base)

	if len(samples) < 5 {
		return ratio, false
	}

	consecutive := 0
	for i := len(samples) - 1; i >= 0 && i >= len(samples)-5; i-- {
		s := samples[i]
		b := s.preflight
		if b == 0 {
			b = 1
		}
		r := math.Abs(float64(s.postflight-s.preflight)) / float64(b)
		if r > 0.15 {
			consecutive++
		} else {
			break
		}
	}
	if consecutive >= 5 {
		return ratio, true
	}
	return ratio, false
}