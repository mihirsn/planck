package source_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mihirsn/planck/internal/source"
)

func TestNewFileSource_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := source.NewFileSource("/nonexistent/path/to/file.log")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestNewFileSource_ValidFile(t *testing.T) {
	t.Parallel()

	// Create a temp file.
	tmp, err := os.CreateTemp(t.TempDir(), "test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tmp.Close()

	_, err = source.NewFileSource(tmp.Name())
	if err != nil {
		t.Errorf("expected no error for valid file, got: %v", err)
	}
}

func TestFileSource_Stream_ReadsLines(t *testing.T) {
	t.Parallel()

	// Write test content to a temp file.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	content := "line one\nline two\nline three\n"

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	fs, err := source.NewFileSource(path)
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}

	ch, err := fs.Stream()
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}

	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "line one" {
		t.Errorf("expected first line 'line one', got %q", lines[0])
	}
	if lines[2] != "line three" {
		t.Errorf("expected third line 'line three', got %q", lines[2])
	}
}

func TestFileSource_Stream_EmptyFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "empty.log")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	fs, err := source.NewFileSource(path)
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}

	ch, err := fs.Stream()
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var count int
	for range ch {
		count++
	}

	if count != 0 {
		t.Errorf("expected 0 lines from empty file, got %d", count)
	}
}

func TestFileSource_Stream_SingleLine(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "single.log")
	if err := os.WriteFile(path, []byte(`{"path":"/x","status":200}`), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	fs, err := source.NewFileSource(path)
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}

	ch, _ := fs.Stream()

	var lines []string
	for l := range ch {
		lines = append(lines, l)
	}

	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}
}
