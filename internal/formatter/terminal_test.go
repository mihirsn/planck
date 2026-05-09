package formatter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mihirsn/planck/internal/formatter"
	"github.com/mihirsn/planck/internal/metrics"
)

func makeReport() metrics.Report {
	return metrics.Report{
		SourceName:     "file \"app.log\"",
		TotalRequests:  100,
		MalformedLines: 2,
		TopEndpoints: []metrics.EndpointStat{
			{Path: "/login", Count: 40, SharePct: 40.0},
			{Path: "/invoice", Count: 35, SharePct: 35.0},
		},
		TrafficByHour: []metrics.TrafficByHour{
			{Hour: 14, Count: 60},
			{Hour: 15, Count: 40},
		},
		ErrorEndpoints: []metrics.EndpointStat{
			{Path: "/checkout", ErrorRate: 12.5},
			{Path: "/login", ErrorRate: 2.0},
		},
		SlowEndpoints: []metrics.EndpointStat{
			{Path: "/invoice", AvgLatencyMs: 820, P95LatencyMs: 1400},
		},
	}
}

func TestPrintTerminal_ContainsSections(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	formatter.PrintTerminal(&buf, makeReport())
	out := buf.String()

	sections := []string{
		"Planck Analysis",
		"Top endpoints",
		"Traffic by hour",
		"Error rates",
		"Slow endpoints",
		"Insights",
	}

	for _, section := range sections {
		if !strings.Contains(out, section) {
			t.Errorf("expected output to contain %q", section)
		}
	}
}

func TestPrintTerminal_ContainsSourceName(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	report := makeReport()
	report.SourceName = "Docker container \"api-service\""
	formatter.PrintTerminal(&buf, report)

	if !strings.Contains(buf.String(), "api-service") {
		t.Error("expected output to contain source name")
	}
}

func TestPrintTerminal_MalformedWarning(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	report := makeReport()
	report.MalformedLines = 5
	formatter.PrintTerminal(&buf, report)

	if !strings.Contains(buf.String(), "Skipped 5 malformed") {
		t.Error("expected malformed warning in output")
	}
}

func TestPrintTerminal_NoMalformedWarning(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	report := makeReport()
	report.MalformedLines = 0
	formatter.PrintTerminal(&buf, report)

	if strings.Contains(buf.String(), "malformed") {
		t.Error("expected no malformed warning when count is 0")
	}
}

func TestPrintTerminal_TotalRequests(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	formatter.PrintTerminal(&buf, makeReport())

	if !strings.Contains(buf.String(), "100") {
		t.Error("expected total request count in output")
	}
}

func TestPrintTerminal_EmptyReport(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	// Should not panic on empty report.
	formatter.PrintTerminal(&buf, metrics.Report{
		SourceName:    "test",
		TotalRequests: 0,
	})

	out := buf.String()
	if !strings.Contains(out, "Planck Analysis") {
		t.Error("expected header even for empty report")
	}
}

func TestPrintTerminal_CompletionLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	formatter.PrintTerminal(&buf, makeReport())

	if !strings.Contains(buf.String(), "Analysis complete") {
		t.Error("expected completion line at end of output")
	}
}

func TestPrintTerminal_LargeRequestCount(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	report := makeReport()
	report.TotalRequests = 12430
	formatter.PrintTerminal(&buf, report)

	// formatInt should produce 12,430.
	if !strings.Contains(buf.String(), "12,430") {
		t.Error("expected formatted request count 12,430 in output")
	}
}

func TestPrintTerminal_SmallRequestCount(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	report := makeReport()
	report.TotalRequests = 42
	formatter.PrintTerminal(&buf, report)

	if !strings.Contains(buf.String(), "42") {
		t.Error("expected raw count 42 in output")
	}
}

func TestPrintTerminal_ZeroHourTraffic(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	report := makeReport()
	// Only one hour entry (max == count, bar should be full).
	report.TrafficByHour = []metrics.TrafficByHour{
		{Hour: 0, Count: 100},
	}
	formatter.PrintTerminal(&buf, report)

	if !strings.Contains(buf.String(), "00:00") {
		t.Error("expected hour 00:00 in output")
	}
}

func TestPrintTerminal_HighErrorRateColor(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	report := makeReport()
	report.ErrorEndpoints = []metrics.EndpointStat{
		{Path: "/bad", ErrorRate: 25.0},  // >= 10% → red
		{Path: "/ok", ErrorRate: 5.0},   // < 10% → yellow
	}
	formatter.PrintTerminal(&buf, report)

	out := buf.String()
	if !strings.Contains(out, "/bad") {
		t.Error("expected /bad in error section")
	}
	if !strings.Contains(out, "/ok") {
		t.Error("expected /ok in error section")
	}
}

func TestPrintTerminal_FullSharePctBar(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	report := makeReport()
	report.TopEndpoints = []metrics.EndpointStat{
		{Path: "/all", Count: 100, SharePct: 100.0},
	}
	formatter.PrintTerminal(&buf, report)

	if !strings.Contains(buf.String(), "/all") {
		t.Error("expected /all in top endpoints")
	}
}

func TestPrintTerminal_ZeroMaxHour(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	report := makeReport()
	report.TrafficByHour = []metrics.TrafficByHour{
		{Hour: 10, Count: 0},
	}
	formatter.PrintTerminal(&buf, report)

	// Should not panic when all counts are 0.
	if !strings.Contains(buf.String(), "10:00") {
		t.Error("expected 10:00 in traffic by hour even with zero count")
	}
}
