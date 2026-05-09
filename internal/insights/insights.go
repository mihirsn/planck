// Package insights generates human-readable observations from a metrics Report.
package insights

import (
	"fmt"

	"github.com/mihirsn/planck/internal/metrics"
)

// Insight represents a single actionable observation derived from the metrics.
type Insight struct {
	// Level is the severity: "warning", "info".
	Level string `json:"level"`

	// Message is the human-readable insight text.
	Message string `json:"message"`
}

// Generate produces a list of insights from the given report.
// Insights flag notable patterns such as high error rates or slow endpoints.
func Generate(report metrics.Report) []Insight {
	var insights []Insight

	// Flag endpoints with a high error rate.
	for _, ep := range report.ErrorEndpoints {
		if ep.ErrorRate >= 10.0 {
			insights = append(insights, Insight{
				Level:   "warning",
				Message: fmt.Sprintf("%s has a high error rate of %.1f%%", ep.Path, ep.ErrorRate),
			})
		}
	}

	// Flag slow endpoints (avg latency > 500ms).
	for _, ep := range report.SlowEndpoints {
		if ep.AvgLatencyMs > 500 {
			insights = append(insights, Insight{
				Level:   "warning",
				Message: fmt.Sprintf("%s is slow (avg: %.0fms, p95: %.0fms)", ep.Path, ep.AvgLatencyMs, ep.P95LatencyMs),
			})
		}
	}

	return insights
}
