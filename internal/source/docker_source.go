package source

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DockerSource reads log lines from a running Docker container by executing
// `docker logs <container>`. No Docker SDK is used — only exec.Command.
type DockerSource struct {
	container  string
	tail       int
	since      string
	cmdBuilder func(name string, args ...string) *exec.Cmd // injectable for testing
}

// NewDockerSource creates a DockerSource for the given container.
//
// tail: number of lines to fetch (0 = all lines).
// since: fetch logs newer than this duration/timestamp (empty = all).
//
// Returns an error if Docker CLI is not available on PATH.
func NewDockerSource(container string, tail int, since string) (*DockerSource, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf(
			"Docker CLI not found.\nPlease install Docker or use file input mode.",
		)
	}
	return &DockerSource{
		container:  container,
		tail:       tail,
		since:      since,
		cmdBuilder: exec.Command,
	}, nil
}

// newDockerSourceWithBuilder creates a DockerSource with a custom command builder.
// This is used only in tests to avoid needing a real Docker daemon.
func newDockerSourceWithBuilder(container string, tail int, since string, builder func(string, ...string) *exec.Cmd) *DockerSource {
	return &DockerSource{
		container:  container,
		tail:       tail,
		since:      since,
		cmdBuilder: builder,
	}
}

// Stream executes `docker logs` and emits each output line through a channel.
// stderr is captured separately to produce clean user-facing error messages.
// The channel is closed once all lines have been emitted.
func (d *DockerSource) Stream() (<-chan string, error) {
	args := d.buildArgs()
	cmd := d.cmdBuilder("docker", args...) //nolint:gosec // container name is validated by docker

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start docker logs: %w", err)
	}

	ch := make(chan string)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			ch <- scanner.Text()
		}

		if err := cmd.Wait(); err != nil {
			errMsg := strings.TrimSpace(stderr.String())
			if strings.Contains(errMsg, "No such container") {
				fmt.Printf("\nContainer %q not found.\n", d.container)
			}
			// Other errors after partial streaming are silently ignored —
			// this matches `docker logs` behavior on stopped containers.
		}
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
		args = append(args, "--since", d.since)
	}

	args = append(args, d.container)
	return args
}
