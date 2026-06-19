package source

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// DockerSource reads log lines from a running Docker container by executing
// `docker logs <container>`. No Docker SDK is used — only exec.Command.
type DockerSource struct {
	container  string
	tail       int
	since      string
	until      string
	cmdBuilder func(name string, args ...string) *exec.Cmd // injectable for testing
}

// NewDockerSource creates a DockerSource for the given container.
//
// tail: number of lines to fetch (0 = all lines).
// since: fetch logs newer than this duration/timestamp (empty = all).
// until: fetch logs older than this duration/timestamp (empty = all).
//
// Returns an error if Docker CLI is not available on PATH.
func NewDockerSource(container string, tail int, since string, until string) (*DockerSource, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf(
			"Docker CLI not found.\nPlease install Docker or use file input mode.",
		)
	}
	return &DockerSource{
		container:  container,
		tail:       tail,
		since:      since,
		until:      until,
		cmdBuilder: exec.Command,
	}, nil
}

// newDockerSourceWithBuilder creates a DockerSource with a custom command builder.
// This is used only in tests to avoid needing a real Docker daemon.
func newDockerSourceWithBuilder(container string, tail int, since string, until string, builder func(string, ...string) *exec.Cmd) *DockerSource {
	return &DockerSource{
		container:  container,
		tail:       tail,
		since:      since,
		until:      until,
		cmdBuilder: builder,
	}
}

// Stream executes `docker logs` and emits each output line through a channel.
// Both stdout and stderr are read concurrently. Docker sends container logs
// to stderr by default, but some log drivers use stdout. Reading them
// concurrently prevents deadlocks if one stream's OS buffer fills up.
// The channel is closed once all lines have been emitted.
func (d *DockerSource) Stream() (<-chan string, error) {
	args := d.buildArgs()
	cmd := d.cmdBuilder("docker", args...) //nolint:gosec // container name is validated by docker

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start docker logs: %w", err)
	}

	ch := make(chan string)
	var wg sync.WaitGroup
	wg.Add(2)

	// Read stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			ch <- scanner.Text()
		}
	}()

	// Read stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			ch <- scanner.Text()
		}
	}()

	// Wait for both streams to finish, then close the channel.
	go func() {
		wg.Wait()
		close(ch)
		// Ignore wait error — partial streaming on stopped containers is fine.
		_ = cmd.Wait()
	}()

	return ch, nil
}

// BuildArgs constructs the argument slice for the `docker logs` command.
// Exported for testing purposes.
func (d *DockerSource) BuildArgs() []string {
	return d.buildArgs()
}

// buildArgs constructs the argument slice for the `docker logs` command
// based on the configured options.
func (d *DockerSource) buildArgs() []string {
	args := []string{"logs"}

	if d.tail > 0 {
		args = append(args, "--tail", strconv.Itoa(d.tail))
	}

	if d.since != "" {
		args = append(args, "--since", normalizeSince(d.since))
	}

	if d.until != "" {
		args = append(args, "--until", normalizeSince(d.until))
	}

	args = append(args, d.container)
	return args
}

// normalizeSince converts a "days" shorthand like "3d" to an equivalent hours
// string ("72h") that Docker's --since flag understands. All other values
// (e.g. "1h", "30m", RFC3339 timestamps) are passed through unchanged.
//
// Examples:
//
//	"3d"  → "72h"
//	"1h"  → "1h"   (unchanged)
//	"30m" → "30m"  (unchanged)
func normalizeSince(s string) string {
	if !strings.HasSuffix(s, "d") {
		return s
	}
	days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
	if err != nil || days <= 0 {
		return s // not a valid day count — pass through and let docker report the error
	}
	return strconv.Itoa(days*24) + "h"
}
