package source

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ContainerStats holds the parsed resource metrics for a single container.
type ContainerStats struct {
	// CPUPercent is the container's CPU usage as a percentage (0–100).
	CPUPercent float64
	// MemUsedMB is the container's current memory usage in megabytes.
	MemUsedMB float64
	// MemLimitMB is the container's memory limit in megabytes.
	MemLimitMB float64
	// MemPercent is the container's memory usage as a percentage of its limit (0–100).
	MemPercent float64
	// ContainerName is the name of the container as reported by Docker.
	ContainerName string
}

// StatsSource is the abstraction for container resource stat providers.
type StatsSource interface {
	// Collect fetches a single point-in-time snapshot of container resource usage.
	Collect() (ContainerStats, error)
}

// dockerStatsJSON is the raw JSON shape returned by `docker stats --no-stream --format json`.
type dockerStatsJSON struct {
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
}

// DockerStatsSource collects container resource metrics via `docker stats --no-stream`.
// It follows the exact same pattern as DockerSource — uses exec.Command, no SDK.
type DockerStatsSource struct {
	container  string
	cmdBuilder func(name string, args ...string) *exec.Cmd // injectable for testing
}

// NewDockerStatsSource creates a DockerStatsSource for the given container.
// Returns an error if Docker CLI is not available on PATH.
func NewDockerStatsSource(container string) (*DockerStatsSource, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf(
			"Docker CLI not found.\nPlease install Docker or use file input mode.",
		)
	}
	return &DockerStatsSource{
		container:  container,
		cmdBuilder: exec.Command,
	}, nil
}

// newDockerStatsSourceWithBuilder creates a DockerStatsSource with a custom command builder.
// Used only in tests to avoid needing a real Docker daemon.
func newDockerStatsSourceWithBuilder(container string, builder func(string, ...string) *exec.Cmd) *DockerStatsSource {
	return &DockerStatsSource{
		container:  container,
		cmdBuilder: builder,
	}
}

// Collect runs `docker stats <container> --no-stream --format json` and returns
// a parsed ContainerStats snapshot. It exits immediately after one sample.
func (d *DockerStatsSource) Collect() (ContainerStats, error) {
	cmd := d.cmdBuilder("docker", "stats", d.container, "--no-stream", "--format", "{{json .}}") //nolint:gosec
	out, err := cmd.Output()
	if err != nil {
		return ContainerStats{}, fmt.Errorf("docker stats failed for container %q: %w", d.container, err)
	}

	var raw dockerStatsJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return ContainerStats{}, fmt.Errorf("failed to parse docker stats output: %w", err)
	}

	cpuPct, err := parsePercent(raw.CPUPerc)
	if err != nil {
		return ContainerStats{}, fmt.Errorf("failed to parse CPU percent %q: %w", raw.CPUPerc, err)
	}

	memUsed, memLimit, err := parseMemUsage(raw.MemUsage)
	if err != nil {
		return ContainerStats{}, fmt.Errorf("failed to parse memory usage %q: %w", raw.MemUsage, err)
	}

	memPct, err := parsePercent(raw.MemPerc)
	if err != nil {
		return ContainerStats{}, fmt.Errorf("failed to parse memory percent %q: %w", raw.MemPerc, err)
	}

	return ContainerStats{
		CPUPercent:    cpuPct,
		MemUsedMB:     memUsed,
		MemLimitMB:    memLimit,
		MemPercent:    memPct,
		ContainerName: raw.Name,
	}, nil
}

// parsePercent strips a trailing "%" and parses the float value.
// e.g. "12.34%" → 12.34.
func parsePercent(s string) (float64, error) {
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	return strconv.ParseFloat(s, 64)
}

// parseMemUsage parses Docker's "<used> / <limit>" memory string into MB values.
// Docker emits values with units like "MiB", "GiB", "MB", "GB", "kB", "B".
// e.g. "512MiB / 2GiB" → (512.0, 2048.0, nil).
func parseMemUsage(s string) (usedMB, limitMB float64, err error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected format %q, expected \"<used> / <limit>\"", s)
	}
	usedMB, err = parseMemValue(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("parsing used: %w", err)
	}
	limitMB, err = parseMemValue(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("parsing limit: %w", err)
	}
	return usedMB, limitMB, nil
}

// parseMemValue converts a memory string with a unit suffix to megabytes.
// Supported units (case-insensitive): B, kB, KiB, MB, MiB, GB, GiB.
func parseMemValue(s string) (float64, error) {
	units := []struct {
		suffix string
		toMB   float64
	}{
		{"GiB", 1024},
		{"GB", 1000},
		{"MiB", 1},
		{"MB", 1},
		{"KiB", 1.0 / 1024},
		{"kB", 1.0 / 1000},
		{"B", 1.0 / (1024 * 1024)},
	}
	upper := strings.ToUpper(s)
	for _, u := range units {
		if strings.HasSuffix(upper, strings.ToUpper(u.suffix)) {
			numStr := strings.TrimSpace(s[:len(s)-len(u.suffix)])
			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("cannot parse numeric part of %q: %w", s, err)
			}
			return val * u.toMB, nil
		}
	}
	// No unit — treat as raw bytes.
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse memory value %q: %w", s, err)
	}
	return val / (1024 * 1024), nil
}
