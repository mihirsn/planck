// Package metrics provides all log analysis calculations for Planck.
package metrics

import (
	"math"
	"sort"
	"time"

	"github.com/mihirsn/planck/internal/models"
)

// Options controls the behavior of the metrics calculation.
type Options struct {
	// TopN is the number of top endpoints to include in the report.
	TopN int

	// SlowN is the number of slowest endpoints to include in the report.
	SlowN int

	// SourceName is a human-readable description of the log source
	// (e.g. `file "app.log"` or `Docker container "api"`).
	SourceName string

	// Malformed is the number of log lines that could not be parsed.
	Malformed int

	// Excluded is the number of log entries filtered out by --exclude-path.
	Excluded int

	// Filtered is the number of entries filtered out by --filter-status or time range.
	Filtered int

	// FixedIntervalSec, when > 0, overrides the log timestamp span as the RPS
	// denominator. Use this in watch mode so RPS reflects "requests per poll
	// interval" rather than the (potentially much shorter) burst window.
	FixedIntervalSec float64
}

// EndpointStat holds aggregated statistics for a single endpoint path.
type EndpointStat struct {
	// Path is the request path (e.g. "/invoice").
	Path string `json:"path"`

	// Count is the total number of requests to this endpoint.
	Count int `json:"count"`

	// ErrorCount is the number of requests with HTTP status >= 400.
	ErrorCount int `json:"error_count"`

	// ErrorRate is ErrorCount / Count as a percentage (0–100).
	ErrorRate float64 `json:"error_rate"`

	// AvgLatencyMs is the mean latency in milliseconds.
	AvgLatencyMs float64 `json:"avg_latency_ms"`

	// P95LatencyMs is the 95th-percentile latency in milliseconds.
	// This requires storing all latencies for the endpoint in memory,
	// which is an acceptable trade-off for the target log sizes.
	P95LatencyMs float64 `json:"p95_latency_ms"`

	// SharePct is this endpoint's share of total requests as a percentage.
	SharePct float64 `json:"share_pct"`
}

// TrafficByHour holds per-hour request counts.
type TrafficByHour struct {
	// Hour is the UTC hour (0–23).
	Hour int `json:"hour"`

	// Count is the number of requests in that hour.
	Count int `json:"count"`
}

// Report is the complete analysis result produced by Calculate.
type Report struct {
	// SourceName describes where the logs came from.
	SourceName string `json:"source"`

	// TotalRequests is the total number of valid log entries analyzed.
	TotalRequests int `json:"total_requests"`

	// AvgRPS is the average requests per second across the analyzed time range.
	// This is 0 if the time range is less than 1 second.
	AvgRPS float64 `json:"avg_rps,omitempty"`

	// MalformedLines is the count of lines that were skipped due to parsing errors.
	MalformedLines int `json:"malformed_lines"`

	// ExcludedEntries is the count of entries filtered out by --exclude-path.
	ExcludedEntries int `json:"excluded_entries"`

	// FilteredEntries is the count of entries dropped by --filter-status or time range.
	FilteredEntries int `json:"filtered_entries"`

	// TopEndpoints is the list of most-requested endpoints (up to Options.TopN).
	TopEndpoints []EndpointStat `json:"top_endpoints"`

	// TrafficByHour is a list of per-hour request counts, sorted by hour.
	TrafficByHour []TrafficByHour `json:"traffic_by_hour"`

	// ErrorEndpoints is the list of endpoints with at least one error,
	// sorted by error rate descending.
	ErrorEndpoints []EndpointStat `json:"error_endpoints"`

	// SlowEndpoints is the list of slowest endpoints (up to Options.SlowN),
	// sorted by avg latency descending.
	SlowEndpoints []EndpointStat `json:"slow_endpoints"`
}

// endpointAccumulator holds intermediate state per endpoint while processing.
type endpointAccumulator struct {
	count      int
	errorCount int
	latencies  []int
}

// Calculate processes log entries and produces a Report.
func Calculate(entries []models.LogEntry, opts Options) Report {
	accMap := make(map[string]*endpointAccumulator)
	hourMap := make(map[int]int)

	var hasTime bool
	var minTime, maxTime time.Time

	for i := range entries {
		e := &entries[i]

		// Accumulate per-endpoint data.
		acc, ok := accMap[e.Path]
		if !ok {
			acc = &endpointAccumulator{}
			accMap[e.Path] = acc
		}
		acc.count++
		if e.Status >= 400 {
			acc.errorCount++
		}
		if e.LatencyMs > 0 {
			acc.latencies = append(acc.latencies, e.LatencyMs)
		}

		// Accumulate per-hour traffic.
		hour := e.Timestamp.UTC().Hour()
		hourMap[hour]++

		// Track min/max time for RPS calculation.
		if !hasTime {
			minTime, maxTime = e.Timestamp, e.Timestamp
			hasTime = true
		} else {
			if e.Timestamp.Before(minTime) {
				minTime = e.Timestamp
			}
			if e.Timestamp.After(maxTime) {
				maxTime = e.Timestamp
			}
		}
	}

	stats := buildStats(accMap, len(entries))

	var avgRPS float64
	if len(entries) > 0 {
		var durationSec float64
		if opts.FixedIntervalSec > 0 {
			// Watch mode: use the configured poll interval as the denominator so
			// RPS reflects "requests in the last N seconds" consistently,
			// regardless of whether traffic arrived in a burst or spread out.
			durationSec = opts.FixedIntervalSec
		} else if hasTime {
			// Analyze mode: use the actual timestamp span of the log entries.
			durationSec = maxTime.Sub(minTime).Seconds()
		}
		if durationSec >= 1.0 {
			avgRPS = float64(len(entries)) / durationSec
		}
	}

	return Report{
		SourceName:      opts.SourceName,
		TotalRequests:   len(entries),
		AvgRPS:          avgRPS,
		MalformedLines:  opts.Malformed,
		ExcludedEntries: opts.Excluded,
		FilteredEntries: opts.Filtered,
		TopEndpoints:    topN(stats, opts.TopN),
		TrafficByHour:   buildHourly(hourMap),
		ErrorEndpoints:  errorEndpoints(stats),
		SlowEndpoints:   slowN(stats, opts.SlowN),
	}
}

// buildStats converts the accumulator map into a slice of EndpointStat,
// computing derived fields (error rate, avg latency, P95).
func buildStats(accMap map[string]*endpointAccumulator, total int) []EndpointStat {
	stats := make([]EndpointStat, 0, len(accMap))

	for path, acc := range accMap {
		stat := EndpointStat{
			Path:       path,
			Count:      acc.count,
			ErrorCount: acc.errorCount,
		}

		if acc.count > 0 {
			stat.ErrorRate = float64(acc.errorCount) / float64(acc.count) * 100
		}

		if total > 0 {
			stat.SharePct = float64(acc.count) / float64(total) * 100
		}

		if len(acc.latencies) > 0 {
			stat.AvgLatencyMs = avgInt(acc.latencies)
			stat.P95LatencyMs = p95Int(acc.latencies)
		}

		stats = append(stats, stat)
	}

	return stats
}

// topN returns the top n endpoints by request count, descending.
func topN(stats []EndpointStat, n int) []EndpointStat {
	sorted := make([]EndpointStat, len(stats))
	copy(sorted, stats)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})
	if n > 0 && n < len(sorted) {
		return sorted[:n]
	}
	return sorted
}

// errorEndpoints returns all endpoints that have at least one error,
// sorted by error rate descending.
func errorEndpoints(stats []EndpointStat) []EndpointStat {
	var result []EndpointStat
	for _, s := range stats {
		if s.ErrorCount > 0 {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ErrorRate > result[j].ErrorRate
	})
	return result
}

// slowN returns the n slowest endpoints by average latency, descending.
// Only endpoints that have latency data are included.
func slowN(stats []EndpointStat, n int) []EndpointStat {
	var withLatency []EndpointStat
	for _, s := range stats {
		if s.AvgLatencyMs > 0 {
			withLatency = append(withLatency, s)
		}
	}
	sort.Slice(withLatency, func(i, j int) bool {
		return withLatency[i].AvgLatencyMs > withLatency[j].AvgLatencyMs
	})
	if n > 0 && n < len(withLatency) {
		return withLatency[:n]
	}
	return withLatency
}

// buildHourly converts the hour map into a sorted slice of TrafficByHour.
func buildHourly(hourMap map[int]int) []TrafficByHour {
	result := make([]TrafficByHour, 0, len(hourMap))
	for hour, count := range hourMap {
		result = append(result, TrafficByHour{Hour: hour, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Hour < result[j].Hour
	})
	return result
}

// avgInt computes the arithmetic mean of a slice of integers.
func avgInt(vals []int) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0
	for _, v := range vals {
		sum += v
	}
	return float64(sum) / float64(len(vals))
}

// p95Int computes the 95th-percentile value of a slice of integers.
// Formula: index = ceil(0.95 * N) - 1 on a sorted ascending slice.
//
// Trade-off note: this requires storing all latency values in memory.
// For the typical log sizes Planck targets this is entirely acceptable.
func p95Int(vals []int) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]int, len(vals))
	copy(sorted, vals)
	sort.Ints(sorted)
	idx := int(math.Ceil(0.95*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	return float64(sorted[idx])
}
