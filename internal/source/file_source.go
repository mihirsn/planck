package source

import (
	"bufio"
	"fmt"
	"os"
)

// FileSource reads log lines from a local file.
type FileSource struct {
	path string
}

// NewFileSource creates a FileSource for the given file path.
// Returns an error if the file cannot be opened.
func NewFileSource(path string) (*FileSource, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("log file not found: %q", path)
	}
	return &FileSource{path: path}, nil
}

// Stream opens the file and emits each line through a channel.
// The channel is closed once all lines have been read or on read error.
// Lines are read with bufio.Scanner for memory-efficient streaming.
func (f *FileSource) Stream() (<-chan string, error) {
	file, err := os.Open(f.path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %q: %w", f.path, err)
	}

	ch := make(chan string)

	go func() {
		defer close(ch)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			ch <- scanner.Text()
		}
	}()

	return ch, nil
}
