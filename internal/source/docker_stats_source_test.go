package source

import (
	"fmt"
	"os/exec"
	"testing"
)

// fakeCmdOutput returns a cmdBuilder that always produces a command writing
// the given output to stdout and exiting 0.
func fakeCmdOutput(output string) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "-n", output)
	}
}

// fakeCmdError returns a cmdBuilder that always exits with a non-zero code.
func fakeCmdError() func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
}

func TestDockerStatsSource_Collect_ValidOutput(t *testing.T) {
	// Typical docker stats JSON for a container using MiB / GiB.
	jsonOutput := `{"Name":"my-api","CPUPerc":"12.34%","MemUsage":"512MiB / 2GiB","MemPerc":"25.00%"}`

	src := newDockerStatsSourceWithBuilder("my-api", fakeCmdOutput(jsonOutput))
	stats, err := src.Collect()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stats.ContainerName != "my-api" {
		t.Errorf("expected ContainerName=my-api, got %q", stats.ContainerName)
	}
	if stats.CPUPercent != 12.34 {
		t.Errorf("expected CPUPercent=12.34, got %v", stats.CPUPercent)
	}
	if stats.MemUsedMB != 512 {
		t.Errorf("expected MemUsedMB=512, got %v", stats.MemUsedMB)
	}
	if stats.MemLimitMB != 2048 {
		t.Errorf("expected MemLimitMB=2048 (2GiB), got %v", stats.MemLimitMB)
	}
	if stats.MemPercent != 25.0 {
		t.Errorf("expected MemPercent=25.0, got %v", stats.MemPercent)
	}
}

func TestDockerStatsSource_Collect_MBUnits(t *testing.T) {
	// Docker sometimes reports MB instead of MiB.
	jsonOutput := `{"Name":"worker","CPUPerc":"5.00%","MemUsage":"256MB / 1000MB","MemPerc":"25.60%"}`

	src := newDockerStatsSourceWithBuilder("worker", fakeCmdOutput(jsonOutput))
	stats, err := src.Collect()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.MemUsedMB != 256 {
		t.Errorf("expected MemUsedMB=256, got %v", stats.MemUsedMB)
	}
	if stats.MemLimitMB != 1000 {
		t.Errorf("expected MemLimitMB=1000, got %v", stats.MemLimitMB)
	}
}

func TestDockerStatsSource_Collect_GBUnits(t *testing.T) {
	jsonOutput := `{"Name":"db","CPUPerc":"2.00%","MemUsage":"1.5GB / 4GB","MemPerc":"37.50%"}`

	src := newDockerStatsSourceWithBuilder("db", fakeCmdOutput(jsonOutput))
	stats, err := src.Collect()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.MemUsedMB != 1500 {
		t.Errorf("expected MemUsedMB=1500, got %v", stats.MemUsedMB)
	}
	if stats.MemLimitMB != 4000 {
		t.Errorf("expected MemLimitMB=4000, got %v", stats.MemLimitMB)
	}
}

func TestDockerStatsSource_Collect_CommandError(t *testing.T) {
	src := newDockerStatsSourceWithBuilder("my-api", fakeCmdError())
	_, err := src.Collect()
	if err == nil {
		t.Error("expected error when docker command fails")
	}
}

func TestDockerStatsSource_Collect_MalformedJSON(t *testing.T) {
	src := newDockerStatsSourceWithBuilder("my-api", fakeCmdOutput(`{not valid json`))
	_, err := src.Collect()
	if err == nil {
		t.Error("expected error for malformed JSON output")
	}
}

func TestDockerStatsSource_Collect_MalformedCPUPercent(t *testing.T) {
	jsonOutput := `{"Name":"my-api","CPUPerc":"N/A","MemUsage":"512MiB / 2GiB","MemPerc":"25.00%"}`
	src := newDockerStatsSourceWithBuilder("my-api", fakeCmdOutput(jsonOutput))
	_, err := src.Collect()
	if err == nil {
		t.Error("expected error for non-numeric CPU percent")
	}
}

func TestDockerStatsSource_Collect_MalformedMemUsage(t *testing.T) {
	jsonOutput := `{"Name":"my-api","CPUPerc":"10.00%","MemUsage":"no-slash-here","MemPerc":"25.00%"}`
	src := newDockerStatsSourceWithBuilder("my-api", fakeCmdOutput(jsonOutput))
	_, err := src.Collect()
	if err == nil {
		t.Error("expected error for memory usage without slash separator")
	}
}

// --- Unit tests for internal parse helpers ---

func TestParsePercent(t *testing.T) {
	cases := []struct {
		input    string
		expected float64
		wantErr  bool
	}{
		{"12.34%", 12.34, false},
		{"0.00%", 0.0, false},
		{"100%", 100.0, false},
		{"N/A", 0, true},
		{"", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parsePercent(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestParseMemValue(t *testing.T) {
	cases := []struct {
		input      string
		expectedMB float64
		wantErr    bool
	}{
		{"512MiB", 512, false},
		{"1GiB", 1024, false},
		{"2GiB", 2048, false},
		{"256MB", 256, false},
		{"1GB", 1000, false},
		{"1.5GB", 1500, false},
		{"1024kB", 1.024, false},
		{"1024KiB", 1.0, false},
		{"not-a-number-MiB", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseMemValue(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Allow small floating-point tolerance.
			diff := got - tc.expectedMB
			if diff < -0.01 || diff > 0.01 {
				t.Errorf("expected %.4fMB, got %.4fMB", tc.expectedMB, got)
			}
		})
	}
}

func TestParseMemUsage(t *testing.T) {
	cases := []struct {
		input   string
		usedMB  float64
		limitMB float64
		wantErr bool
	}{
		{"512MiB / 2GiB", 512, 2048, false},
		{"256MB / 1GB", 256, 1000, false},
		{"no-slash", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			used, limit, err := parseMemUsage(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fmt.Sprintf("%.2f", used) != fmt.Sprintf("%.2f", tc.usedMB) {
				t.Errorf("used: expected %.2fMB, got %.2fMB", tc.usedMB, used)
			}
			if fmt.Sprintf("%.2f", limit) != fmt.Sprintf("%.2f", tc.limitMB) {
				t.Errorf("limit: expected %.2fMB, got %.2fMB", tc.limitMB, limit)
			}
		})
	}
}
