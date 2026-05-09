// Package parser provides JSON log line parsing for Planck.
package parser

import (
	"encoding/json"

	"github.com/mihirsn/planck/internal/models"
)

// Parser reads raw log lines and converts them into LogEntry structs.
// Malformed lines (invalid JSON, missing required fields) are counted
// but do not halt processing.
type Parser struct{}

// New returns a new Parser instance.
func New() *Parser {
	return &Parser{}
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

// parseLine attempts to parse a single JSON log line into a LogEntry.
// Returns the entry and true on success; returns false if malformed.
func (p *Parser) parseLine(line string) (models.LogEntry, bool) {
	if len(line) == 0 {
		return models.LogEntry{}, false
	}

	var entry models.LogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return models.LogEntry{}, false
	}

	// Validate required fields.
	if entry.Path == "" || entry.Status == 0 {
		return models.LogEntry{}, false
	}

	return entry, true
}
