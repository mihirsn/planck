package main

import (
	"os"
	"testing"
)

func TestMainFunc(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Provide safe args that just print help
	os.Args = []string{"planck", "--help"}

	// main doesn't return anything or exit if there's no error on help
	main()
}
