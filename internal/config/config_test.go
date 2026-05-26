package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mihirsn/planck/internal/config"
)

// writeConfig writes content to a temp file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "planck.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, `
watch:
  interval: 30s
  alert_cooldown: 5m
  preset: fastapi

alerts:
  error_rate_pct: 5.0
  p95_latency_ms: 1000
  rps: 50

notify:
  ntfy_topic: my-alerts
  ntfy_server: https://ntfy.sh
  ntfy_token: secret
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Watch.IntervalDuration != 30*time.Second {
		t.Errorf("expected 30s interval, got %v", cfg.Watch.IntervalDuration)
	}
	if cfg.Watch.CooldownDuration != 5*time.Minute {
		t.Errorf("expected 5m cooldown, got %v", cfg.Watch.CooldownDuration)
	}
	if cfg.Alerts.ErrorRatePct != 5.0 {
		t.Errorf("expected error_rate_pct=5.0, got %v", cfg.Alerts.ErrorRatePct)
	}
	if cfg.Notify.NtfyTopic != "my-alerts" {
		t.Errorf("expected topic=my-alerts, got %q", cfg.Notify.NtfyTopic)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Omit interval, cooldown, server — they should all default.
	path := writeConfig(t, `
notify:
  ntfy_topic: my-topic
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Watch.IntervalDuration != 60*time.Second {
		t.Errorf("expected default interval 60s, got %v", cfg.Watch.IntervalDuration)
	}
	if cfg.Watch.CooldownDuration != 10*time.Minute {
		t.Errorf("expected default cooldown 10m, got %v", cfg.Watch.CooldownDuration)
	}
	if cfg.Notify.NtfyServer != "https://ntfy.sh" {
		t.Errorf("expected default server https://ntfy.sh, got %q", cfg.Notify.NtfyServer)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path/planck.yml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeConfig(t, `{invalid yaml: [unclosed`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_UnknownField(t *testing.T) {
	path := writeConfig(t, `
notify:
  ntfy_topic: my-topic
  unknown_field: bad
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for unknown field (strict mode)")
	}
}

func TestValidate_InvalidInterval(t *testing.T) {
	path := writeConfig(t, `
watch:
  interval: not-a-duration
notify:
  ntfy_topic: my-topic
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid interval")
	}
}

func TestValidate_ZeroInterval(t *testing.T) {
	path := writeConfig(t, `
watch:
  interval: 0s
notify:
  ntfy_topic: my-topic
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for zero interval")
	}
}

func TestValidate_InvalidCooldown(t *testing.T) {
	path := writeConfig(t, `
watch:
  alert_cooldown: bad
notify:
  ntfy_topic: my-topic
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid cooldown")
	}
}

func TestValidate_ErrorRatePctOutOfRange(t *testing.T) {
	path := writeConfig(t, `
alerts:
  error_rate_pct: 150.0
notify:
  ntfy_topic: my-topic
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for error_rate_pct > 100")
	}
}

func TestValidate_NegativeP95(t *testing.T) {
	path := writeConfig(t, `
alerts:
  p95_latency_ms: -1
notify:
  ntfy_topic: my-topic
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for negative p95_latency_ms")
	}
}

func TestValidate_NegativeRPS(t *testing.T) {
	path := writeConfig(t, `
alerts:
  rps: -5
notify:
  ntfy_topic: my-topic
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for negative rps")
	}
}

func TestValidate_MissingTopic(t *testing.T) {
	path := writeConfig(t, `
notify:
  ntfy_server: https://ntfy.sh
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for missing ntfy_topic")
	}
}

func TestValidate_InvalidTopicChars(t *testing.T) {
	cases := []string{
		"my/topic",
		"my topic",
		"my@topic",
		"../secret",
	}
	for _, topic := range cases {
		t.Run(topic, func(t *testing.T) {
			path := writeConfig(t, "notify:\n  ntfy_topic: \""+topic+"\"\n")
			_, err := config.Load(path)
			if err == nil {
				t.Errorf("expected error for invalid topic %q", topic)
			}
		})
	}
}

func TestValidate_ValidTopicChars(t *testing.T) {
	cases := []string{"my-topic", "my_topic", "MyTopic123", "a"}
	for _, topic := range cases {
		t.Run(topic, func(t *testing.T) {
			path := writeConfig(t, "notify:\n  ntfy_topic: \""+topic+"\"\n")
			_, err := config.Load(path)
			if err != nil {
				t.Errorf("expected no error for valid topic %q, got %v", topic, err)
			}
		})
	}
}

func TestValidate_InvalidServer(t *testing.T) {
	cases := []string{
		"not-a-url",
		"ftp://ntfy.sh",
		"/local/path",
	}
	for _, server := range cases {
		t.Run(server, func(t *testing.T) {
			path := writeConfig(t, "notify:\n  ntfy_topic: my-topic\n  ntfy_server: \""+server+"\"\n")
			_, err := config.Load(path)
			if err == nil {
				t.Errorf("expected error for invalid server %q", server)
			}
		})
	}
}

func TestDiscover_NotFound(t *testing.T) {
	// Change to a fresh temp dir so neither ./planck.yml nor ~/.planck.yml exists nearby.
	orig, _ := os.Getwd()
	dir := t.TempDir()
	_ = os.Chdir(dir)
	defer os.Chdir(orig)

	found := config.Discover()
	// We can't guarantee ~/.planck.yml doesn't exist on the test machine,
	// so just verify the function doesn't panic and returns a string.
	_ = found
}

func TestDiscover_CurrentDir(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	_ = os.Chdir(dir)
	defer os.Chdir(orig)

	// Create a planck.yml in the current dir.
	os.WriteFile(filepath.Join(dir, "planck.yml"), []byte("notify:\n  ntfy_topic: test\n"), 0644)

	found := config.Discover()
	if found != "planck.yml" {
		t.Errorf("expected planck.yml, got %q", found)
	}
}
