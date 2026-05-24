package ui_test

import (
	"testing"
	"time"

	"github.com/mihirsn/planck/internal/ui"
)

func TestSpinner_StartStop(t *testing.T) {
	s := ui.NewSpinner("testing spinner")

	// Start the spinner
	s.Start()

	// Let it run for a short duration to ensure the goroutine executes
	// and goes through at least one select cycle.
	time.Sleep(150 * time.Millisecond)

	// Stop it
	s.Stop()

	// Calling Stop again should be a no-op and not panic or deadlock
	s.Stop()
}

func TestSpinner_ImmediateStop(t *testing.T) {
	s := ui.NewSpinner("immediate stop")

	// Start and immediately stop to test race conditions
	s.Start()
	s.Stop()
}
