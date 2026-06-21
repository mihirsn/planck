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
  error_rate:
    threshold: 5.0
  p95_latency:
    threshold: 1000
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
	if cfg.Alerts.ErrorRate.Threshold != 5.0 {
		t.Errorf("expected error_rate.threshold=5.0, got %v", cfg.Alerts.ErrorRate.Threshold)
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

func TestValidate_ErrorRateThresholdOutOfRange(t *testing.T) {
	path := writeConfig(t, `
alerts:
  error_rate:
    threshold: 150.0
notify:
  ntfy_topic: my-topic
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for error_rate.threshold > 100")
	}
}

func TestValidate_NegativeP95(t *testing.T) {
	path := writeConfig(t, `
alerts:
  p95_latency:
    threshold: -1
notify:
  ntfy_topic: my-topic
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for negative p95_latency.threshold")
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

func TestLoad_WatchDocker(t *testing.T) {
	t.Parallel()

	t.Run("docker field is read from config", func(t *testing.T) {
		t.Parallel()
		path := writeConfig(t, `
watch:
  docker: my-api
notify:
  ntfy_topic: my-topic
`)
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg.Watch.Docker != "my-api" {
			t.Errorf("expected watch.docker=my-api, got %q", cfg.Watch.Docker)
		}
	})

	t.Run("docker field is optional and defaults to empty string", func(t *testing.T) {
		t.Parallel()
		path := writeConfig(t, `
notify:
  ntfy_topic: my-topic
`)
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg.Watch.Docker != "" {
			t.Errorf("expected empty watch.docker, got %q", cfg.Watch.Docker)
		}
	})
}

func TestLoad_Resources_ValidFullBlock(t *testing.T) {
	path := writeConfig(t, `
notify:
  ntfy_topic: my-topic

resources:
  interval: 30s
  cpu:
    threshold: 80
  memory:
    percent: 75
    absolute: 1500
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Resources.IntervalDuration != 30*time.Second {
		t.Errorf("expected resources interval 30s, got %v", cfg.Resources.IntervalDuration)
	}
	if cfg.Resources.CPU.Threshold != 80 {
		t.Errorf("expected cpu.threshold=80, got %v", cfg.Resources.CPU.Threshold)
	}
	if cfg.Resources.Memory.Percent != 75 {
		t.Errorf("expected memory.percent=75, got %v", cfg.Resources.Memory.Percent)
	}
	if cfg.Resources.Memory.Absolute != 1500 {
		t.Errorf("expected memory.absolute=1500, got %v", cfg.Resources.Memory.Absolute)
	}
}

func TestLoad_Resources_IntervalDefaultsToWatchInterval(t *testing.T) {
	path := writeConfig(t, `
watch:
  interval: 45s

notify:
  ntfy_topic: my-topic

resources:
  cpu:
    threshold: 70
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// resources.interval not set — should inherit watch.interval
	if cfg.Resources.IntervalDuration != 45*time.Second {
		t.Errorf("expected resources interval to default to 45s, got %v", cfg.Resources.IntervalDuration)
	}
}

func TestLoad_Resources_EmptyBlockIsValid(t *testing.T) {
	// An empty resources: block should be valid — no thresholds configured means no alerts.
	path := writeConfig(t, `
notify:
  ntfy_topic: my-topic

resources: {}
`)
	_, err := config.Load(path)
	if err != nil {
		t.Errorf("expected no error for empty resources block, got %v", err)
	}
}

func TestLoad_Resources_OmittedIsValid(t *testing.T) {
	// Omitting resources entirely should be valid (no resource alerts).
	path := writeConfig(t, `
notify:
  ntfy_topic: my-topic
`)
	_, err := config.Load(path)
	if err != nil {
		t.Errorf("expected no error when resources is omitted, got %v", err)
	}
}

func TestValidate_Resources_InvalidInterval(t *testing.T) {
	path := writeConfig(t, `
notify:
  ntfy_topic: my-topic

resources:
  interval: not-a-duration
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid resources.interval")
	}
}

func TestValidate_Resources_CPUThresholdOutOfRange(t *testing.T) {
	cases := []struct {
		name      string
		threshold string
	}{
		{"negative", "-1"},
		{"over 100", "101"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfig(t, `
notify:
  ntfy_topic: my-topic

resources:
  cpu:
    threshold: `+tc.threshold+`
`)
			_, err := config.Load(path)
			if err == nil {
				t.Errorf("expected error for cpu.threshold=%s", tc.threshold)
			}
		})
	}
}

func TestValidate_Resources_MemoryPercentOutOfRange(t *testing.T) {
	cases := []struct {
		name    string
		percent string
	}{
		{"negative", "-1"},
		{"over 100", "110"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfig(t, `
notify:
  ntfy_topic: my-topic

resources:
  memory:
    percent: `+tc.percent+`
`)
			_, err := config.Load(path)
			if err == nil {
				t.Errorf("expected error for memory.percent=%s", tc.percent)
			}
		})
	}
}

func TestValidate_Resources_NegativeMemoryAbsolute(t *testing.T) {
	path := writeConfig(t, `
notify:
  ntfy_topic: my-topic

resources:
  memory:
    absolute: -500
`)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for negative memory.absolute")
	}
}

func TestLoad_Resources_MemoryBothConditionsCoexist(t *testing.T) {
	// Percent and absolute can both be set — either triggers an alert.
	path := writeConfig(t, `
notify:
  ntfy_topic: my-topic

resources:
  memory:
    percent: 80
    absolute: 2048
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Resources.Memory.Percent != 80 {
		t.Errorf("expected memory.percent=80, got %v", cfg.Resources.Memory.Percent)
	}
	if cfg.Resources.Memory.Absolute != 2048 {
		t.Errorf("expected memory.absolute=2048, got %v", cfg.Resources.Memory.Absolute)
	}
}
