// Package models defines the core data structures used throughout Planck.
package models

import "time"

// LogEntry represents a single parsed log line from an application log.
// All fields are parsed from JSON. Only Timestamp, Path, and Status are
// mandatory; Method and LatencyMs are optional but recommended.
type LogEntry struct {
	// Timestamp is when the request was handled. Required.
	Timestamp time.Time `json:"timestamp"`

	// Method is the HTTP method (GET, POST, etc.). Optional.
	Method string `json:"method"`

	// Path is the request path (e.g. "/invoice"). Required.
	Path string `json:"path"`

	// Status is the HTTP response status code. Required.
	Status int `json:"status"`

	// LatencyMs is the request duration in milliseconds. Optional.
	LatencyMs int `json:"latency_ms"`
}
