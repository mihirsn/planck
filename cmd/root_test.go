package cmd

import (
	"bytes"
	"testing"
)

func TestExecute(t *testing.T) {
	// Redirect output to avoid cluttering test output
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	// Test without args to ensure it doesn't panic
	rootCmd.SetArgs([]string{})

	// Execute does not return error, it calls os.Exit(1) on failure.
	// Since no args just prints help, it won't fail.
	Execute()
}
