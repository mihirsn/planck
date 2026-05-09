// Package parser provides JSON log line parsing for Planck.
package parser

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/mihirsn/planck/internal/models"
)

// Parser reads raw log lines and converts them into LogEntry structs
// using a configurable FieldMap so any JSON log schema is supported.
//
// Malformed lines (invalid JSON, missing required fields) are counted
// but do not halt processing.
type Parser struct {
	fields models.FieldMap
}

// New returns a Parser that maps log fields using the given FieldMap.
// Use models.DefaultFieldMap() to parse Planck's native schema.
func New(fields models.FieldMap) *Parser {
	return &Parser{fields: fields}
}

// ParseAll consumes all lines from ch and returns the successfully parsed
// entries along with a count of malformed (skipped) lines.
//
// A line is considered malformed if:
//   - It is not valid JSON.
//   - The parsed entry has an empty Path or a zero Status code.
func (p *Parser) ParseAll(ch <-chan string) ([]models.LogEntry, int) {
	var entries []models.LogEntry
	malformed := 0

	for line := range ch {
		entry, ok := p.parseLine(line)
		if !ok {
			malformed++
			continue
		}
		entries = append(entries, entry)
	}

	return entries, malformed
}

// parseLine parses a single JSON log line into a LogEntry using the
// configured FieldMap. Returns false if the line is malformed.
//
// We decode into map[string]json.RawMessage so we can handle each field
// individually — string fields use json.Unmarshal into string, numeric
// fields use json.Number to preserve integer vs float distinction.
func (p *Parser) parseLine(line string) (models.LogEntry, bool) {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
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
