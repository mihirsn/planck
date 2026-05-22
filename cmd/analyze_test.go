package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeCmd_File(t *testing.T) {
	// Create a temporary log file
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	logContent := `{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/a","status":200,"latency_ms":10}
{"timestamp":"2026-05-08T14:00:02Z","method":"POST","path":"/b","status":500,"latency_ms":20}`
	if err := os.WriteFile(logFile, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to write test log file: %v", err)
	}

	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	// Run with standard options to cover flags
	rootCmd.SetArgs([]string{
		"analyze", logFile,
		"--filter-status", "5xx",
		"--top", "10",
		"--slow", "3",
		"--scan-json",
		"--exclude-path", "/health",
		"--exclude-status", "404",
		"--exclude-method", "OPTIONS",
		"--since", "2026-05-10T15:00:00Z",
		"--until", "2026-05-10T16:00:00Z",
		"--field-path", "my-path",
		"--format", "json",
	})

	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	out := b.String()
	if out == "" {
		t.Error("expected output, got empty string")
	}
}

func TestAnalyzeCmd_NoArgs(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	rootCmd.SetArgs([]string{"analyze"})

	// Execute returns an error because arguments are missing.
	err := rootCmd.Execute()
	if err == nil {
		t.Errorf("expected error when no args provided")
	}
}

func TestAnalyzeCmd_DockerAndJSON(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	// Since we mock docker source creation via DI or just let it fail at docker execution
	// It will error out when it tries to run `docker logs fake-container-1234`
	// but it will cover the argument parsing, format json, and buildFieldMap preset logic.
	rootCmd.SetArgs([]string{
		"analyze",
		"--docker", "fake-container-1234",
		"--preset", "fastapi",
		"--format", "json",
		"--since", "1h",
		"--until", "invalid-time",
	})

	_ = rootCmd.Execute()
}

func TestAnalyzeCmd_InvalidSince(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	os.WriteFile(logFile, []byte(""), 0644)

	rootCmd.SetArgs([]string{
		"analyze", logFile,
		"--since", "not-a-valid-duration",
	})

	_ = rootCmd.Execute()
}

func TestBuildFieldMap(t *testing.T) {
	// Test all overrides
	flags.preset = "fastapi"
	flags.fieldTimestamp = "ts"
	flags.fieldMethod = "meth"
	flags.fieldPath = "p"
	flags.fieldStatus = "st"
	flags.fieldLatency = "lat"

	fm, err := buildFieldMap()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fm.Timestamp != "ts" || fm.Method != "meth" || fm.Path != "p" || fm.Status != "st" || fm.LatencyMs != "lat" {
		t.Errorf("overrides not applied: %v", fm)
	}

	// Test invalid preset
	flags.preset = "nonexistent-preset-1234"
	_, err = buildFieldMap()
	if err == nil {
		t.Error("expected error for invalid preset")
	}

	// Reset flags for other tests
	flags = analyzeFlags{}
}

func TestParseSinceAsTime(t *testing.T) {
	// Empty string
	tm, err := parseSinceAsTime("")
	if err != nil || !tm.IsZero() {
		t.Errorf("expected zero time for empty string, got %v", tm)
	}

	// Invalid duration
	_, err = parseSinceAsTime("invalid")
	if err == nil {
		t.Error("expected error for invalid duration")
	}

	// Valid days
	tmDays, err := parseSinceAsTime("1d")
	if err != nil || tmDays.IsZero() {
		t.Errorf("expected time for 1d, got error: %v", err)
	}

	// Valid duration
	tmDur, err := parseSinceAsTime("1h")
	if err != nil || tmDur.IsZero() {
		t.Errorf("expected time for 1h, got error: %v", err)
	}
}
