// Package parser provides JSON log line parsing for Planck.
package parser

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mihirsn/planck/internal/models"
)

// ParseResult is returned by ParseAll and holds all outcomes of a parse run.
type ParseResult struct {
	// Entries contains all successfully parsed log entries.
	Entries []models.LogEntry

	// Malformed is the count of lines that could not be parsed as valid log entries.
	Malformed int

	// PrefixedJSON is the count of malformed lines that appear to contain a JSON
	// object prefixed with text (e.g. "INFO:api.access:{...}"). These lines would
	// have been parsed successfully with --scan-json enabled.
	// Only populated when scan-json mode is disabled.
	PrefixedJSON int

	// Excluded is the count of entries skipped because their path matched one
	// of the --exclude-path prefixes. These are intentionally filtered, not errors.
	Excluded int

	// Filtered is the count of entries skipped by --filter-status or the time
	// range (--since / --until). These are intentionally filtered, not errors.
	Filtered int
}

// Parser reads raw log lines and converts them into LogEntry structs
// using a configurable FieldMap so any JSON log schema is supported.
//
// Malformed lines (invalid JSON, missing required fields) are counted
// but do not halt processing.
type Parser struct {
	fields       models.FieldMap
	scanJSON     bool
	excludePaths []string
	statusFilter string    // e.g. "5xx", "4xx", "200" — empty means no filter
	sinceTime    time.Time // zero means no lower bound
	untilTime    time.Time // zero means no upper bound
}

// New returns a Parser that maps log fields using the given FieldMap.
// Use models.DefaultFieldMap() to parse Planck's native schema.
func New(fields models.FieldMap) *Parser {
	return &Parser{fields: fields}
}

// SetScanJSON enables or disables scan-json mode. When enabled, each line is
// scanned for the first '{' character and parsing begins from that position.
// This handles logs where a text prefix appears before the JSON object, such
// as Python's default logging format: "INFO:api.access:{...}".
//
// Returns the Parser for method chaining.
func (p *Parser) SetScanJSON(enabled bool) *Parser {
	p.scanJSON = enabled
	return p
}

// SetExcludePaths sets a list of path prefixes to exclude from analysis.
// Any parsed log entry whose path starts with one of these prefixes is
// silently dropped and counted in ParseResult.Excluded.
//
// Example: SetExcludePaths([]string{"/health", "/metrics"})
// will exclude /health, /health/check, /metrics, /metrics/prometheus, etc.
//
// Returns the Parser for method chaining.
func (p *Parser) SetExcludePaths(paths []string) *Parser {
	p.excludePaths = paths
	return p
}

// SetStatusFilter restricts analysis to entries matching the given status pattern.
// Supported patterns:
//   - Class patterns: "2xx", "3xx", "4xx", "5xx" (matches any status in that range)
//   - Exact codes:    "200", "404", "500" (matches a single code)
//
// An empty string disables filtering (all statuses are included).
// Returns the Parser for method chaining.
func (p *Parser) SetStatusFilter(pattern string) *Parser {
	p.statusFilter = strings.ToLower(strings.TrimSpace(pattern))
	return p
}

// SetTimeRange restricts analysis to entries whose timestamp falls within
// [since, until]. A zero time.Time for either bound means "no bound".
//
// Entries without a parseable timestamp are never filtered by time range.
// Returns the Parser for method chaining.
func (p *Parser) SetTimeRange(since, until time.Time) *Parser {
	p.sinceTime = since
	p.untilTime = until
	return p
}

// ParseAll consumes all lines from ch and returns a ParseResult containing
// the parsed entries and diagnostic counters.
func (p *Parser) ParseAll(ch <-chan string) ParseResult {
	var result ParseResult

	for line := range ch {
		entry, ok, prefixed := p.parseLine(line)
		if !ok {
			result.Malformed++
			if prefixed {
				result.PrefixedJSON++
			}
			continue
		}
		if p.isExcluded(entry.Path) {
			result.Excluded++
			continue
		}
		if p.isFiltered(entry) {
			result.Filtered++
			continue
		}
		result.Entries = append(result.Entries, entry)
	}

	return result
}

// isExcluded reports whether path matches any of the configured exclude prefixes.
func (p *Parser) isExcluded(path string) bool {
	for _, prefix := range p.excludePaths {
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// isFiltered reports whether an entry should be dropped by the status filter
// or by the time range (since/until). Returns true when the entry should be skipped.
func (p *Parser) isFiltered(entry models.LogEntry) bool {
	// Status filter.
	if p.statusFilter != "" && !matchesStatusFilter(entry.Status, p.statusFilter) {
		return true
	}
	// Time range filter — only applied when the entry has a parseable timestamp.
	if !entry.Timestamp.IsZero() {
		if !p.sinceTime.IsZero() && entry.Timestamp.Before(p.sinceTime) {
			return true
		}
		if !p.untilTime.IsZero() && entry.Timestamp.After(p.untilTime) {
			return true
		}
	}
	return false
}

// matchesStatusFilter reports whether status matches the given pattern.
// Patterns: "2xx"/"3xx"/"4xx"/"5xx" for class ranges, or an exact code string.
func matchesStatusFilter(status int, pattern string) bool {
	switch pattern {
	case "2xx":
		return status >= 200 && status < 300
	case "3xx":
		return status >= 300 && status < 400
	case "4xx":
		return status >= 400 && status < 500
	case "5xx":
		return status >= 500 && status < 600
	default:
		// Treat as exact status code.
		return strconv.Itoa(status) == pattern
	}
}

// parseLine parses a single log line. It returns:
//   - entry: the parsed LogEntry (zero value if not ok)
//   - ok: true if the line was successfully parsed
//   - prefixed: true if the line appeared to contain JSON prefixed with text
//     (only set when scan-json mode is disabled, used for the hint message)
func (p *Parser) parseLine(line string) (entry models.LogEntry, ok bool, prefixed bool) {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return models.LogEntry{}, false, false
	}

	candidate := line

	if p.scanJSON {
		// Scan mode: find the first '{' and parse from there.
		idx := strings.Index(line, "{")
		if idx < 0 {
			return models.LogEntry{}, false, false
		}
		candidate = line[idx:]
	}

	entry, ok = p.parseJSON(candidate)
	if ok {
		return entry, true, false
	}

	// Normal mode: check whether stripping a text prefix would have succeeded.
	// This populates ParseResult.PrefixedJSON for the hint message.
	if !p.scanJSON {
		if idx := strings.Index(line, "{"); idx > 0 {
			if _, tryOk := p.parseJSON(line[idx:]); tryOk {
				return models.LogEntry{}, false, true
			}
		}
	}

	return models.LogEntry{}, false, false
}

// parseJSON is the core parsing logic. It expects a string that begins with '{'
// and attempts to decode it as a JSON log entry using the configured FieldMap.
func (p *Parser) parseJSON(line string) (models.LogEntry, bool) {
	if len(line) == 0 || line[0] != '{' {
		return models.LogEntry{}, false
	}

	// Decode into a raw map preserving all values as json.RawMessage.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return models.LogEntry{}, false
	}

	var entry models.LogEntry

	// --- timestamp ---
	if v, ok := raw[p.fields.Timestamp]; ok {
		var ts string
		if err := json.Unmarshal(v, &ts); err == nil {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				entry.Timestamp = t
			} else if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				entry.Timestamp = t
			}
		}
	}

	// --- method ---
	if v, ok := raw[p.fields.Method]; ok {
		var method string
		if err := json.Unmarshal(v, &method); err == nil {
			entry.Method = method
		}
	}

	// --- path (required) ---
	if v, ok := raw[p.fields.Path]; ok {
		var path string
		if err := json.Unmarshal(v, &path); err == nil {
			entry.Path = path
		}
	}
	if entry.Path == "" {
		return models.LogEntry{}, false
	}

	// --- status (required) ---
	if v, ok := raw[p.fields.Status]; ok {
		var num json.Number
		if err := json.Unmarshal(v, &num); err == nil {
			if n, err := num.Int64(); err == nil {
				entry.Status = int(n)
			}
		}
	}
	if entry.Status == 0 {
		return models.LogEntry{}, false
	}

	// --- latency (optional) ---
	if v, ok := raw[p.fields.LatencyMs]; ok {
		var num json.Number
		if err := json.Unmarshal(v, &num); err == nil {
			entry.LatencyMs = parseLatency(num)
		}
	}

	return entry, true
}

// parseLatency converts a json.Number latency value to integer milliseconds.
//
// Auto-detection rule:
//   - If the value is a JSON float (contains "."), it is treated as seconds
//     and multiplied by 1000. This handles frameworks like FastAPI that log
//     duration as a decimal seconds value (e.g. 0.120 → 120ms).
//   - If the value is a JSON integer, it is used as-is (already milliseconds).
func parseLatency(num json.Number) int {
	s := num.String()

	if strings.Contains(s, ".") {
		// Float value → treat as seconds, convert to ms.
		if f, err := num.Float64(); err == nil {
			return int(math.Round(f * 1000))
		}
	}

	// Integer value → already milliseconds.
	if n, err := num.Int64(); err == nil {
		return int(n)
	}

	return 0
}
