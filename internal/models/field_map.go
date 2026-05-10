// Package models defines the core data structures used throughout Planck.
package models

import "fmt"

// FieldMap maps Planck's canonical field names to the actual JSON keys
// present in a user's log file. This allows Planck to work with any
// JSON log schema without requiring the user to change their log format.
//
// All fields have sensible defaults via DefaultFieldMap(). Use PresetFieldMap
// to load a preset for a popular framework, or set individual fields manually.
type FieldMap struct {
	// Timestamp is the JSON key for the request timestamp.
	Timestamp string

	// Method is the JSON key for the HTTP method (GET, POST, etc.).
	Method string

	// Path is the JSON key for the request path (e.g. "/invoice").
	Path string

	// Status is the JSON key for the HTTP status code (integer).
	Status string

	// LatencyMs is the JSON key for the request latency.
	// Planck accepts both integer (milliseconds) and float (seconds) values.
	// If the value is a JSON float, it is automatically converted from seconds
	// to milliseconds (e.g. 0.120 → 120ms).
	LatencyMs string
}

// DefaultFieldMap returns the default FieldMap matching Planck's native
// log schema: timestamp, method, path, status, latency_ms.
func DefaultFieldMap() FieldMap {
	return FieldMap{
		Timestamp: "timestamp",
		Method:    "method",
		Path:      "path",
		Status:    "status",
		LatencyMs: "latency_ms",
	}
}

// Available preset names.
const (
	PresetFastAPI = "fastapi"
	PresetExpress = "express"
	PresetGin     = "gin"
	PresetEcho    = "echo"
	PresetSpring  = "spring"
)

// presets maps preset names to their FieldMap definitions.
var presets = map[string]FieldMap{
	// FastAPI / uvicorn JSON access logs.
	// duration is logged as a float in seconds (e.g. 0.120).
	PresetFastAPI: {
		Timestamp: "timestamp",
		Method:    "method",
		Path:      "path",
		Status:    "status_code",
		LatencyMs: "duration",
	},

	// Express.js with morgan JSON middleware.
	PresetExpress: {
		Timestamp: "timestamp",
		Method:    "method",
		Path:      "url",
		Status:    "statusCode",
		LatencyMs: "responseTime",
	},

	// Go Gin with a JSON logger middleware (e.g. gin-contrib/logger).
	PresetGin: {
		Timestamp: "time",
		Method:    "method",
		Path:      "path",
		Status:    "status",
		LatencyMs: "latency",
	},

	// Go Echo with a JSON logger middleware.
	PresetEcho: {
		Timestamp: "time",
		Method:    "method",
		Path:      "uri",
		Status:    "status",
		LatencyMs: "latency",
	},

	// Spring Boot with a custom JSON HTTP access log filter.
	// Uses @timestamp as emitted by logstash-logback-encoder.
	PresetSpring: {
		Timestamp: "@timestamp",
		Method:    "method",
		Path:      "uri",
		Status:    "status",
		LatencyMs: "duration",
	},
}

// PresetFieldMap returns the FieldMap for the named framework preset.
// Returns an error if the preset name is not recognized.
// Pass an empty string or "default" to get DefaultFieldMap().
func PresetFieldMap(name string) (FieldMap, error) {
	if name == "" || name == "default" {
		return DefaultFieldMap(), nil
	}

	fm, ok := presets[name]
	if !ok {
		return FieldMap{}, fmt.Errorf(
			"unknown preset %q — available presets: fastapi, express, gin, echo, spring",
			name,
		)
	}

	return fm, nil
}

// AvailablePresets returns the list of built-in preset names.
func AvailablePresets() []string {
	return []string{PresetFastAPI, PresetExpress, PresetGin, PresetEcho, PresetSpring}
}
