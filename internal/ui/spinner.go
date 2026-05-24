package ui

import (
	"fmt"
	"time"
)

// Spinner is a lightweight, non-blocking terminal loading indicator.
type Spinner struct {
	text    string
	stopCh  chan struct{}
	doneCh  chan struct{}
	stopped bool
}

// NewSpinner creates a new terminal spinner with the given text.
func NewSpinner(text string) *Spinner {
	return &Spinner{
		text:   text,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start launches the spinner animation in a background goroutine.
func (s *Spinner) Start() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	go func() {
		i := 0
		for {
			select {
			case <-s.stopCh:
				// Clear the line completely when stopped
				fmt.Print("\r\033[K")
				close(s.doneCh)
				return
			default:
				// \r returns the cursor to the start of the line
				// \033[36m makes the text cyan
				// \033[0m resets the color
				fmt.Printf("\r\033[36m%s\033[0m %s", frames[i%len(frames)], s.text)
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// Stop halts the spinner and clears its output from the terminal.
func (s *Spinner) Stop() {
	if !s.stopped {
		s.stopped = true
		close(s.stopCh)
		<-s.doneCh // block until the terminal line is fully cleared
	}
}
