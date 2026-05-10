// Package formatter provides terminal output rendering for Planck reports.
package formatter

import (
	"fmt"
	"io"
	"strings"

	"github.com/mihirsn/planck/internal/insights"
	"github.com/mihirsn/planck/internal/metrics"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// PrintTerminal writes a human-friendly report to w using ANSI color codes.
func PrintTerminal(w io.Writer, report metrics.Report) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s⚛️  Planck Analysis%s\n", colorBold, colorReset)
	fmt.Fprintf(w, "%s%s%s\n", colorGray, strings.Repeat("─", 50), colorReset)

	fmt.Fprintf(w, "Source:          %s\n", report.SourceName)
	fmt.Fprintf(w, "Total requests:  %s%s%s\n", colorBold, formatInt(report.TotalRequests), colorReset)

	if report.MalformedLines > 0 {
		fmt.Fprintf(w, "%s⚠  Skipped %d malformed log entries.%s\n",
			colorYellow, report.MalformedLines, colorReset)
	}

	if report.ExcludedEntries > 0 {
		fmt.Fprintf(w, "%s⊘  Excluded %d entries matching --exclude-path filters.%s\n",
			colorGray, report.ExcludedEntries, colorReset)
	}

	// Top endpoints.
	if len(report.TopEndpoints) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s🔥 Top endpoints%s\n", colorBold, colorReset)
		for _, ep := range report.TopEndpoints {
			bar := progressBar(ep.SharePct, 20)
			fmt.Fprintf(w, "  %-30s %s%s%s  %.1f%%\n",
				ep.Path, colorCyan, bar, colorReset, ep.SharePct)
		}
	}

	// Traffic by hour.
	if len(report.TrafficByHour) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s⏰ Traffic by hour (UTC)%s\n", colorBold, colorReset)

		maxCount := 0
		for _, h := range report.TrafficByHour {
			if h.Count > maxCount {
				maxCount = h.Count
			}
		}

		for _, h := range report.TrafficByHour {
			bar := trafficBar(h.Count, maxCount, 20)
			fmt.Fprintf(w, "  %02d:00  %s%s%s  %s\n",
				h.Hour, colorCyan, bar, colorReset, formatInt(h.Count))
		}
	}

	// Error rates.
	if len(report.ErrorEndpoints) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s⚠️  Error rates%s\n", colorBold, colorReset)
		for _, ep := range report.ErrorEndpoints {
			color := colorYellow
			if ep.ErrorRate >= 10.0 {
				color = colorRed
			}
			fmt.Fprintf(w, "  %-30s %s%.1f%%%s\n", ep.Path, color, ep.ErrorRate, colorReset)
		}
	}

	// Slow endpoints.
	if len(report.SlowEndpoints) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s🐢 Slow endpoints%s\n", colorBold, colorReset)
		for _, ep := range report.SlowEndpoints {
			fmt.Fprintf(w, "  %-30s avg: %s%.0fms%s  p95: %.0fms\n",
				ep.Path, colorYellow, ep.AvgLatencyMs, colorReset, ep.P95LatencyMs)
		}
	}

	// Insights.
	generated := insights.Generate(report)
	if len(generated) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s💡 Insights%s\n", colorBold, colorReset)
		for _, ins := range generated {
			icon := "  ℹ"
			color := colorCyan
			if ins.Level == "warning" {
				icon = "  ⚠"
				color = colorYellow
			}
			fmt.Fprintf(w, "%s%s %s%s\n", color, icon, ins.Message, colorReset)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%s%s\n", colorGray, strings.Repeat("─", 50), colorReset)
	fmt.Fprintf(w, "%sAnalysis complete.%s\n\n", colorGreen, colorReset)
}

// progressBar returns a fixed-width ASCII bar proportional to pct (0–100).
func progressBar(pct float64, width int) string {
	filled := int(pct / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// trafficBar returns a fixed-width ASCII bar proportional to count/max.
func trafficBar(count, max, width int) string {
	if max == 0 {
		return strings.Repeat("░", width)
	}
	filled := int(float64(count) / float64(max) * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// formatInt formats an integer with thousands separators.
func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
