package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWatchCmd_ConfigNotFound(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	// Change to an empty temp dir so no planck.yml is auto-discovered.
	orig, _ := os.Getwd()
	dir := t.TempDir()
	_ = os.Chdir(dir)
	defer os.Chdir(orig)

	rootCmd.SetArgs([]string{"watch"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when planck.yml is not found")
	}
}

func TestWatchCmd_InvalidConfigPath(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	rootCmd.SetArgs([]string{
		"watch",
		"--config", "/nonexistent/planck.yml",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}

func TestWatchCmd_InvalidConfig(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "planck.yml")
	// Invalid: missing ntfy topic
	os.WriteFile(cfgPath, []byte("notify:\n  ntfy:\n    server: https://ntfy.sh\n"), 0644)

	rootCmd.SetArgs([]string{
		"watch",
		"--config", cfgPath,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected validation error for config missing ntfy_topic")
	}
}

func TestWatchCmd_InvalidPreset(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "planck.yml")
	// Valid YAML structure but invalid preset name — caught at Validate() time.
	os.WriteFile(cfgPath, []byte(`
watch:
  preset: nonexistent-preset-xyz

notify:
  ntfy:
    topic: my-topic
`), 0644)

	rootCmd.SetArgs([]string{
		"watch",
		"--config", cfgPath,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid preset in config")
	}
}

func TestWatchCmd_NoContainersConfigured(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "planck.yml")
	// Valid config but no containers or watch.docker specified.
	os.WriteFile(cfgPath, []byte(`
notify:
  ntfy:
    topic: my-topic
`), 0644)

	rootCmd.SetArgs([]string{
		"watch",
		"--config", cfgPath,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no containers are configured")
	}
}
