package insights_test

import (
	"testing"

	"github.com/mihirsn/planck/internal/insights"
	"github.com/mihirsn/planck/internal/metrics"
)

func TestGenerate_NoInsights(t *testing.T) {
	t.Parallel()

	report := metrics.Report{
		ErrorEndpoints: []metrics.EndpointStat{
			{Path: "/a", ErrorRate: 5.0}, // below threshold
		},
		SlowEndpoints: []metrics.EndpointStat{
			{Path: "/b", AvgLatencyMs: 300}, // below threshold
		},
	}

	ins := insights.Generate(report)
	if len(ins) != 0 {
		t.Errorf("expected 0 insights, got %d", len(ins))
	}
}

func TestGenerate_HighErrorRate(t *testing.T) {
	t.Parallel()

	report := metrics.Report{
		ErrorEndpoints: []metrics.EndpointStat{
			{Path: "/checkout", ErrorRate: 15.5},
		},
	}

	ins := insights.Generate(report)

	if len(ins) != 1 {
		t.Fatalf("expected 1 insight, got %d", len(ins))
	}
	if ins[0].Level != "warning" {
		t.Errorf("expected level=warning, got %s", ins[0].Level)
	}
}

func TestGenerate_SlowEndpoint(t *testing.T) {
	t.Parallel()

	report := metrics.Report{
		SlowEndpoints: []metrics.EndpointStat{
			{Path: "/invoice", AvgLatencyMs: 820, P95LatencyMs: 1400},
		},
	}

	ins := insights.Generate(report)

	if len(ins) != 1 {
		t.Fatalf("expected 1 insight, got %d", len(ins))
	}
	if ins[0].Level != "warning" {
		t.Errorf("expected level=warning, got %s", ins[0].Level)
	}
}

func TestGenerate_MultipleInsights(t *testing.T) {
	t.Parallel()

	report := metrics.Report{
		ErrorEndpoints: []metrics.EndpointStat{
			{Path: "/checkout", ErrorRate: 25.0},
			{Path: "/login", ErrorRate: 3.0}, // below threshold
		},
		SlowEndpoints: []metrics.EndpointStat{
			{Path: "/invoice", AvgLatencyMs: 900},
			{Path: "/fast", AvgLatencyMs: 100}, // below threshold
		},
	}

	ins := insights.Generate(report)

	if len(ins) != 2 {
		t.Errorf("expected 2 insights, got %d", len(ins))
	}
}

func TestGenerate_ExactThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		errorRate float64
		latency   float64
		wantCount int
	}{
		{"error at threshold", 10.0, 0, 1},
		{"error below threshold", 9.9, 0, 0},
		{"latency at threshold", 0, 501, 1},
		{"latency exactly at 500", 0, 500, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report := metrics.Report{}
			if tt.errorRate > 0 {
				report.ErrorEndpoints = []metrics.EndpointStat{
					{Path: "/x", ErrorRate: tt.errorRate},
				}
			}
			if tt.latency > 0 {
				report.SlowEndpoints = []metrics.EndpointStat{
					{Path: "/y", AvgLatencyMs: tt.latency},
				}
			}

			ins := insights.Generate(report)
			if len(ins) != tt.wantCount {
				t.Errorf("expected %d insights, got %d", tt.wantCount, len(ins))
			}
		})
	}
}

func TestGenerate_EmptyReport(t *testing.T) {
	t.Parallel()

	ins := insights.Generate(metrics.Report{})
	if len(ins) != 0 {
		t.Errorf("expected 0 insights for empty report, got %d", len(ins))
	}
}
