package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWatchCmd_NoDockerFlag(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	// --docker is required; omitting it should fail.
	rootCmd.SetArgs([]string{"watch"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when --docker flag is missing")
	}
}

func TestWatchCmd_ConfigNotFound(t *testing.T) {
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	// Change to an empty temp dir so no planck.yml is auto-discovered.
	orig, _ := os.Getwd()
	dir := t.TempDir()
	_ = os.Chdir(dir)
	defer os.Chdir(orig)

	rootCmd.SetArgs([]string{"watch", "--docker", "my-container"})
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
		"--docker", "my-container",
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
	// Invalid: missing ntfy_topic
	os.WriteFile(cfgPath, []byte("notify:\n  ntfy_server: https://ntfy.sh\n"), 0644)

	rootCmd.SetArgs([]string{
		"watch",
		"--docker", "my-container",
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
	os.WriteFile(cfgPath, []byte("watch:\n  preset: nonexistent-preset-xyz\nnotify:\n  ntfy_topic: my-topic\n"), 0644)

	rootCmd.SetArgs([]string{
		"watch",
		"--docker", "my-container",
		"--config", cfgPath,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid preset in config")
	}
}
