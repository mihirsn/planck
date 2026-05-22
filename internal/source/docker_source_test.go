package source // white-box test: accesses newDockerSourceWithBuilder

import (
	"os/exec"
	"reflect"
	"testing"
)

// TestBuildArgs_NoTailNoSince verifies default args with no optional flags.
func TestBuildArgs_NoTailNoSince(t *testing.T) {
	t.Parallel()

	src := newDockerSourceWithBuilder("my-container", 0, "", exec.Command)
	got := src.BuildArgs()
	want := []string{"logs", "my-container"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs() = %v, want %v", got, want)
	}
}

// TestBuildArgs_WithTail verifies --tail flag is appended correctly.
func TestBuildArgs_WithTail(t *testing.T) {
	t.Parallel()

	src := newDockerSourceWithBuilder("api", 500, "", exec.Command)
	got := src.BuildArgs()
	want := []string{"logs", "--tail", "500", "api"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs() = %v, want %v", got, want)
	}
}

// TestBuildArgs_WithSince verifies --since flag is appended correctly.
func TestBuildArgs_WithSince(t *testing.T) {
	t.Parallel()

	src := newDockerSourceWithBuilder("api", 0, "1h", exec.Command)
	got := src.BuildArgs()
	want := []string{"logs", "--since", "1h", "api"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs() = %v, want %v", got, want)
	}
}

// TestBuildArgs_WithTailAndSince verifies both flags together.
func TestBuildArgs_WithTailAndSince(t *testing.T) {
	t.Parallel()

	src := newDockerSourceWithBuilder("invoice-api", 100, "30m", exec.Command)
	got := src.BuildArgs()
	want := []string{"logs", "--tail", "100", "--since", "30m", "invoice-api"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs() = %v, want %v", got, want)
	}
}

// TestDockerSource_Stream_WithEcho uses the system `echo` command as a fake
// docker binary to verify that Stream correctly reads and emits lines.
func TestDockerSource_Stream_WithEcho(t *testing.T) {
	t.Parallel()

	// Replace exec.Command("docker", ...) with echo so we can control output.
	fakeBuilder := func(_ string, _ ...string) *exec.Cmd {
		return exec.Command("echo", "line-one\nline-two\nline-three")
	}

	src := newDockerSourceWithBuilder("fake", 0, "", fakeBuilder)

	ch, err := src.Stream()
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var lines []string
	for l := range ch {
		lines = append(lines, l)
	}

	// echo outputs one line containing literal \n — so we get 1 line.
	if len(lines) == 0 {
		t.Error("expected at least 1 line from fake stream")
	}
}

// TestDockerSource_Stream_ErrorOnStart verifies Stream returns an error
// when the command cannot be started (invalid binary).
func TestDockerSource_Stream_ErrorOnStart(t *testing.T) {
	t.Parallel()

	fakeBuilder := func(_ string, _ ...string) *exec.Cmd {
		// Use a command that does not exist to force cmd.Start() to fail.
		return exec.Command("/nonexistent/binary/that/does/not/exist")
	}

	src := newDockerSourceWithBuilder("fake", 0, "", fakeBuilder)

	_, err := src.Stream()
	if err == nil {
		t.Error("expected error when command cannot start, got nil")
	}
}

// TestDockerSource_Stream_ReadsStderr verifies that Stream captures lines
// written to stderr. This is critical because `docker logs` sends container
// output to stderr by default, not stdout.
func TestDockerSource_Stream_ReadsStderr(t *testing.T) {
	t.Parallel()

	// sh -c 'echo ... >&2' writes to stderr only.
	fakeBuilder := func(_ string, _ ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "echo stderr-line-one >&2 && echo stderr-line-two >&2")
	}

	src := newDockerSourceWithBuilder("fake", 0, "", fakeBuilder)

	ch, err := src.Stream()
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var lines []string
	for l := range ch {
		lines = append(lines, l)
	}

	if len(lines) < 2 {
		t.Errorf("expected 2 lines from stderr, got %d: %v", len(lines), lines)
	}
}

// TestDockerSource_Stream_MergesStdoutAndStderr verifies that Stream captures
// lines from both stdout and stderr in a single pass.
func TestDockerSource_Stream_MergesStdoutAndStderr(t *testing.T) {
	t.Parallel()

	// Write one line to stdout and one to stderr.
	fakeBuilder := func(_ string, _ ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "echo stdout-line && echo stderr-line >&2")
	}

	src := newDockerSourceWithBuilder("fake", 0, "", fakeBuilder)

	ch, err := src.Stream()
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var lines []string
	for l := range ch {
		lines = append(lines, l)
	}

	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines (stdout + stderr), got %d: %v", len(lines), lines)
	}
}

func TestNewDockerSource(t *testing.T) {
	// Valid should return source (no validation on container name in constructor)
	_, err := NewDockerSource("", 0, "")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Valid should return source
	src, err := NewDockerSource("my-container", 100, "1h")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if src == nil {
		t.Error("expected source, got nil")
	}
}

func TestNormalizeSince(t *testing.T) {
	// Empty should return empty
	if got := normalizeSince(""); got != "" {
		t.Errorf("expected empty string, got %s", got)
	}

	// Duration passes through unchanged
	gotDur := normalizeSince("1h")
	if gotDur != "1h" {
		t.Errorf("expected 1h, got %s", gotDur)
	}

	// Days converts to hours
	gotDays := normalizeSince("3d")
	if gotDays != "72h" {
		t.Errorf("expected 72h, got %s", gotDays)
	}

	// Already formatted should return as-is
	rfcTime := "2026-05-10T15:00:00Z"
	if got := normalizeSince(rfcTime); got != rfcTime {
		t.Errorf("expected %s, got %s", rfcTime, got)
	}
}
