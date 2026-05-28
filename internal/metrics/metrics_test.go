package metrics_test

import (
	"math"
	"testing"
	"time"

	"github.com/mihirsn/planck/internal/metrics"
	"github.com/mihirsn/planck/internal/models"
)

// makeEntry creates a LogEntry with the given fields for use in tests.
func makeEntry(path string, status, latency int, hour int) models.LogEntry {
	return models.LogEntry{
		Timestamp: time.Date(2026, 5, 8, hour, 0, 0, 0, time.UTC),
		Method:    "GET",
		Path:      path,
		Status:    status,
		LatencyMs: latency,
	}
}

func defaultOpts() metrics.Options {
	return metrics.Options{
		TopN:       5,
		SlowN:      5,
		SourceName: "test",
		Malformed:  0,
	}
}

func TestCalculate_TotalRequests(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/a", 200, 50, 14),
		makeEntry("/b", 200, 60, 14),
		makeEntry("/c", 200, 70, 15),
	}

	report := metrics.Calculate(entries, defaultOpts())

	if report.TotalRequests != 3 {
		t.Errorf("expected TotalRequests=3, got %d", report.TotalRequests)
	}
}

func TestCalculate_MalformedLines(t *testing.T) {
	t.Parallel()

	opts := defaultOpts()
	opts.Malformed = 7

	report := metrics.Calculate([]models.LogEntry{makeEntry("/x", 200, 10, 10)}, opts)

	if report.MalformedLines != 7 {
		t.Errorf("expected MalformedLines=7, got %d", report.MalformedLines)
	}
}

func TestCalculate_TopEndpoints(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/login", 200, 10, 14),
		makeEntry("/login", 200, 10, 14),
		makeEntry("/login", 200, 10, 14),
		makeEntry("/invoice", 200, 10, 14),
		makeEntry("/invoice", 200, 10, 14),
		makeEntry("/checkout", 200, 10, 14),
	}

	opts := defaultOpts()
	opts.TopN = 2
	report := metrics.Calculate(entries, opts)

	if len(report.TopEndpoints) != 2 {
		t.Fatalf("expected 2 top endpoints, got %d", len(report.TopEndpoints))
	}
	if report.TopEndpoints[0].Path != "/login" {
		t.Errorf("expected top endpoint /login, got %s", report.TopEndpoints[0].Path)
	}
	if report.TopEndpoints[1].Path != "/invoice" {
		t.Errorf("expected 2nd endpoint /invoice, got %s", report.TopEndpoints[1].Path)
	}
}

func TestCalculate_TopEndpoints_SharePct(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/a", 200, 10, 14),
		makeEntry("/a", 200, 10, 14),
		makeEntry("/b", 200, 10, 14),
		makeEntry("/b", 200, 10, 14),
	}

	report := metrics.Calculate(entries, defaultOpts())

	for _, ep := range report.TopEndpoints {
		if ep.SharePct < 49.9 || ep.SharePct > 50.1 {
			t.Errorf("expected 50%% share for %s, got %.2f%%", ep.Path, ep.SharePct)
		}
	}
}

func TestCalculate_ErrorEndpoints(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/login", 200, 10, 14),
		makeEntry("/checkout", 500, 200, 14),
		makeEntry("/checkout", 500, 200, 14),
		makeEntry("/checkout", 200, 100, 14),
		makeEntry("/invoice", 400, 50, 14),
	}

	report := metrics.Calculate(entries, defaultOpts())

	if len(report.ErrorEndpoints) != 2 {
		t.Fatalf("expected 2 error endpoints, got %d", len(report.ErrorEndpoints))
	}

	// Verify sorted by error rate descending.
	// /checkout: 2/3 ≈ 66.7%, /invoice: 1/1 = 100%.
	// Error endpoints are sorted descending by error rate.
	for i := 1; i < len(report.ErrorEndpoints); i++ {
		if report.ErrorEndpoints[i].ErrorRate > report.ErrorEndpoints[i-1].ErrorRate {
			t.Errorf("error endpoints not sorted by error rate descending at index %d", i)
		}
	}

	// Verify both paths appear.
	paths := make(map[string]bool)
	for _, ep := range report.ErrorEndpoints {
		paths[ep.Path] = true
	}
	if !paths["/checkout"] {
		t.Error("expected /checkout in error endpoints")
	}
	if !paths["/invoice"] {
		t.Error("expected /invoice in error endpoints")
	}
}

func TestCalculate_NoErrors(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/a", 200, 10, 14),
		makeEntry("/b", 201, 20, 14),
	}

	report := metrics.Calculate(entries, defaultOpts())

	if len(report.ErrorEndpoints) != 0 {
		t.Errorf("expected no error endpoints, got %d", len(report.ErrorEndpoints))
	}
}

func TestCalculate_AvgLatency(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/api", 200, 100, 14),
		makeEntry("/api", 200, 200, 14),
		makeEntry("/api", 200, 300, 14),
	}

	report := metrics.Calculate(entries, defaultOpts())

	if len(report.TopEndpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(report.TopEndpoints))
	}
	if report.TopEndpoints[0].AvgLatencyMs != 200.0 {
		t.Errorf("expected avg latency 200ms, got %.1f", report.TopEndpoints[0].AvgLatencyMs)
	}
}

func TestCalculate_P95Latency(t *testing.T) {
	t.Parallel()

	// 20 entries: 19 at 100ms, 1 at 1000ms.
	// P95 index = ceil(0.95 * 20) - 1 = ceil(19) - 1 = 18 (0-indexed) → 19th value sorted = 100.
	var entries []models.LogEntry
	for i := 0; i < 19; i++ {
		entries = append(entries, makeEntry("/api", 200, 100, 14))
	}
	entries = append(entries, makeEntry("/api", 200, 1000, 14))

	report := metrics.Calculate(entries, defaultOpts())

	p95 := report.TopEndpoints[0].P95LatencyMs
	// P95 of [100x19, 1000x1] sorted = index 18 = 100ms.
	if p95 != 100 {
		t.Errorf("expected P95=100ms, got %.0f", p95)
	}
}

func TestCalculate_SlowEndpoints(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/fast", 200, 10, 14),
		makeEntry("/medium", 200, 300, 14),
		makeEntry("/slow", 200, 900, 14),
		makeEntry("/veryslow", 200, 1500, 14),
	}

	opts := defaultOpts()
	opts.SlowN = 2
	report := metrics.Calculate(entries, opts)

	if len(report.SlowEndpoints) != 2 {
		t.Fatalf("expected 2 slow endpoints, got %d", len(report.SlowEndpoints))
	}
	if report.SlowEndpoints[0].Path != "/veryslow" {
		t.Errorf("expected /veryslow as slowest, got %s", report.SlowEndpoints[0].Path)
	}
	if report.SlowEndpoints[1].Path != "/slow" {
		t.Errorf("expected /slow as 2nd slowest, got %s", report.SlowEndpoints[1].Path)
	}
}

func TestCalculate_TrafficByHour(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/a", 200, 10, 14),
		makeEntry("/b", 200, 10, 14),
		makeEntry("/c", 200, 10, 15),
	}

	report := metrics.Calculate(entries, defaultOpts())

	if len(report.TrafficByHour) != 2 {
		t.Fatalf("expected 2 hour buckets, got %d", len(report.TrafficByHour))
	}
	if report.TrafficByHour[0].Hour != 14 || report.TrafficByHour[0].Count != 2 {
		t.Errorf("expected hour 14 with 2 requests, got hour %d with %d",
			report.TrafficByHour[0].Hour, report.TrafficByHour[0].Count)
	}
	if report.TrafficByHour[1].Hour != 15 || report.TrafficByHour[1].Count != 1 {
		t.Errorf("expected hour 15 with 1 request, got hour %d with %d",
			report.TrafficByHour[1].Hour, report.TrafficByHour[1].Count)
	}
}

func TestCalculate_TrafficByHour_Sorted(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/a", 200, 10, 16),
		makeEntry("/b", 200, 10, 10),
		makeEntry("/c", 200, 10, 13),
	}

	report := metrics.Calculate(entries, defaultOpts())

	for i := 1; i < len(report.TrafficByHour); i++ {
		if report.TrafficByHour[i].Hour < report.TrafficByHour[i-1].Hour {
			t.Errorf("traffic by hour not sorted at index %d", i)
		}
	}
}

func TestCalculate_ZeroLatency_ExcludedFromSlow(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		// latency_ms = 0 means no latency data
		{Path: "/nolatency", Status: 200, Timestamp: time.Now()},
	}

	report := metrics.Calculate(entries, defaultOpts())

	if len(report.SlowEndpoints) != 0 {
		t.Errorf("expected 0 slow endpoints for zero-latency entries, got %d", len(report.SlowEndpoints))
	}
}

func TestCalculate_SourceName(t *testing.T) {
	t.Parallel()

	opts := defaultOpts()
	opts.SourceName = "file \"app.log\""

	report := metrics.Calculate([]models.LogEntry{makeEntry("/x", 200, 10, 10)}, opts)

	if report.SourceName != opts.SourceName {
		t.Errorf("expected source name %q, got %q", opts.SourceName, report.SourceName)
	}
}

func TestCalculate_TopN_LargerThanEndpoints(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/a", 200, 10, 14),
		makeEntry("/b", 200, 10, 14),
	}

	opts := defaultOpts()
	opts.TopN = 100 // more than the 2 available

	report := metrics.Calculate(entries, opts)

	if len(report.TopEndpoints) != 2 {
		t.Errorf("expected all 2 endpoints when TopN > available, got %d", len(report.TopEndpoints))
	}
}

func TestCalculate_P95Latency_SingleEntry(t *testing.T) {
	t.Parallel()

	// Single entry: P95 index = ceil(0.95 * 1) - 1 = 0.
	entries := []models.LogEntry{
		makeEntry("/api", 200, 250, 14),
	}
	report := metrics.Calculate(entries, defaultOpts())

	if len(report.TopEndpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(report.TopEndpoints))
	}
	if report.TopEndpoints[0].P95LatencyMs != 250 {
		t.Errorf("expected P95=250ms for single entry, got %.0f", report.TopEndpoints[0].P95LatencyMs)
	}
}

func TestCalculate_AvgLatency_SingleEntry(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/api", 200, 333, 14),
	}
	report := metrics.Calculate(entries, defaultOpts())

	if report.TopEndpoints[0].AvgLatencyMs != 333.0 {
		t.Errorf("expected avg latency 333ms, got %.1f", report.TopEndpoints[0].AvgLatencyMs)
	}
}

func TestCalculate_SlowN_LargerThanSlowEndpoints(t *testing.T) {
	t.Parallel()

	entries := []models.LogEntry{
		makeEntry("/a", 200, 100, 14),
		makeEntry("/b", 200, 200, 14),
	}

	opts := defaultOpts()
	opts.SlowN = 100 // more than available

	report := metrics.Calculate(entries, opts)

	if len(report.SlowEndpoints) != 2 {
		t.Errorf("expected all 2 slow endpoints when SlowN > available, got %d", len(report.SlowEndpoints))
	}
}

func TestCalculate_AvgRPS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []models.LogEntry
		wantRPS float64
	}{
		{
			name: "10 seconds duration with 50 requests",
			entries: []models.LogEntry{
				{Timestamp: time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC)},
				{Timestamp: time.Date(2026, 5, 8, 14, 0, 10, 0, time.UTC)},
			}, // len(entries)=2, duration=10 => 0.2
			wantRPS: 0.2,
		},
		{
			name: "less than 1 second duration",
			entries: []models.LogEntry{
				{Timestamp: time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC)},
				{Timestamp: time.Date(2026, 5, 8, 14, 0, 0, 500000000, time.UTC)}, // 500ms
			},
			wantRPS: 0, // Should be 0 for <1s
		},
		{
			name: "exactly 1 second duration",
			entries: []models.LogEntry{
				{Timestamp: time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC)},
				{Timestamp: time.Date(2026, 5, 8, 14, 0, 1, 0, time.UTC)},
			},
			wantRPS: 2.0, // len=2, duration=1s => 2.0
		},
		{
			name:    "zero entries",
			entries: []models.LogEntry{},
			wantRPS: 0,
		},
		{
			name: "unordered timestamps",
			entries: []models.LogEntry{
				{Timestamp: time.Date(2026, 5, 8, 14, 0, 10, 0, time.UTC)}, // Newest
				{Timestamp: time.Date(2026, 5, 8, 14, 0, 5, 0, time.UTC)},
				{Timestamp: time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC)}, // Oldest
			},
			wantRPS: 0.3, // len=3, duration=10s => 0.3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report := metrics.Calculate(tt.entries, metrics.Options{})
			if math.Abs(report.AvgRPS-tt.wantRPS) > 0.0001 {
				t.Errorf("AvgRPS = %v, want %v", report.AvgRPS, tt.wantRPS)
			}
		})
	}
}

// TestCalculate_AvgRPS_FixedInterval verifies that when FixedIntervalSec is set
// (watch mode), RPS is based on the fixed interval — not the timestamp span.
// This ensures a burst of 8 requests in 4s over a 30s window gives 8/30 = 0.27,
// not 8/4 = 2.0, which is the confusing behaviour without this fix.
func TestCalculate_AvgRPS_FixedInterval(t *testing.T) {
	t.Parallel()

	// 8 requests all arrived in a 4-second burst within a 30s poll window.
	base := time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC)
	entries := []models.LogEntry{
		{Timestamp: base},
		{Timestamp: base.Add(1 * time.Second)},
		{Timestamp: base.Add(2 * time.Second)},
		{Timestamp: base.Add(2 * time.Second)},
		{Timestamp: base.Add(3 * time.Second)},
		{Timestamp: base.Add(3 * time.Second)},
		{Timestamp: base.Add(4 * time.Second)},
		{Timestamp: base.Add(4 * time.Second)},
	}

	t.Run("without FixedIntervalSec (analyze mode) uses timestamp span", func(t *testing.T) {
		t.Parallel()
		report := metrics.Calculate(entries, metrics.Options{})
		// span = 4s, requests = 8 => 2.0 req/s
		wantRPS := 8.0 / 4.0
		if math.Abs(report.AvgRPS-wantRPS) > 0.0001 {
			t.Errorf("AvgRPS = %.4f, want %.4f (timestamp span mode)", report.AvgRPS, wantRPS)
		}
	})

	t.Run("with FixedIntervalSec=30 (watch mode) uses poll interval", func(t *testing.T) {
		t.Parallel()
		report := metrics.Calculate(entries, metrics.Options{FixedIntervalSec: 30})
		// fixed interval = 30s, requests = 8 => 0.2667 req/s
		wantRPS := 8.0 / 30.0
		if math.Abs(report.AvgRPS-wantRPS) > 0.0001 {
			t.Errorf("AvgRPS = %.4f, want %.4f (fixed interval mode)", report.AvgRPS, wantRPS)
		}
	})

	t.Run("FixedIntervalSec overrides even when timestamp span is larger", func(t *testing.T) {
		t.Parallel()
		// Entries spread over 60s, but interval is fixed at 30s.
		wideEntries := []models.LogEntry{
			{Timestamp: base},
			{Timestamp: base.Add(60 * time.Second)},
		}
		report := metrics.Calculate(wideEntries, metrics.Options{FixedIntervalSec: 30})
		wantRPS := 2.0 / 30.0
		if math.Abs(report.AvgRPS-wantRPS) > 0.0001 {
			t.Errorf("AvgRPS = %.4f, want %.4f", report.AvgRPS, wantRPS)
		}
	})

	t.Run("zero entries with FixedIntervalSec returns 0", func(t *testing.T) {
		t.Parallel()
		report := metrics.Calculate([]models.LogEntry{}, metrics.Options{FixedIntervalSec: 30})
		if report.AvgRPS != 0 {
			t.Errorf("AvgRPS = %v, want 0 for empty entries", report.AvgRPS)
		}
	})
}
