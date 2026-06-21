// Package config reads and validates the planck.yml configuration file.
package config

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// validTopicRe matches only safe ntfy topic names: letters, digits, hyphens, underscores.
var validTopicRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Config is the top-level structure of planck.yml.
type Config struct {
	Watch     WatchConfig     `yaml:"watch"`
	Alerts    AlertConfig     `yaml:"alerts"`
	Resources ResourcesConfig `yaml:"resources"`
	Notify    NotifyConfig    `yaml:"notify"`
}

// WatchConfig controls the polling behaviour.
type WatchConfig struct {
	// Docker is the Docker container name or ID to watch.
	// Can be overridden at runtime with the --docker CLI flag.
	Docker string `yaml:"docker"`
	// Interval is how often Planck polls logs (e.g. "60s", "2m"). Default: 60s.
	Interval string `yaml:"interval"`
	// AlertCooldown prevents repeated alerts for the same breach (e.g. "10m"). Default: 10m.
	AlertCooldown string `yaml:"alert_cooldown"`
	// Preset is the log format preset, same as --preset on analyze.
	Preset string `yaml:"preset"`

	// Parsed values — populated by Validate().
	IntervalDuration time.Duration `yaml:"-"`
	CooldownDuration time.Duration `yaml:"-"`
}

// AlertRule defines thresholds and endpoint filters for a specific alert type.
type AlertRule struct {
	Threshold    float64  `yaml:"threshold"`
	ExcludePaths []string `yaml:"exclude_paths,omitempty"`
	IncludePaths []string `yaml:"include_paths,omitempty"`
}

// AlertConfig holds optional threshold values. Zero value means "not configured".
type AlertConfig struct {
	// ErrorRate triggers an alert if any endpoint's error rate exceeds its threshold percentage.
	ErrorRate AlertRule `yaml:"error_rate"`
	// P95Latency triggers an alert if any endpoint's p95 latency exceeds its threshold (in ms).
	P95Latency AlertRule `yaml:"p95_latency"`
	// RPS triggers an alert if requests-per-second exceed this value.
	RPS float64 `yaml:"rps"`
}

// ResourcesConfig holds container-level resource monitoring settings.
// All fields are optional; omitting the block entirely disables resource alerts.
type ResourcesConfig struct {
	// Interval controls how often container stats are polled (e.g. "30s", "1m").
	// Defaults to watch.interval when omitted.
	Interval string `yaml:"interval"`
	// CPU holds the CPU usage threshold.
	CPU CPUThreshold `yaml:"cpu"`
	// Memory holds the memory usage thresholds.
	Memory MemThreshold `yaml:"memory"`

	// IntervalDuration is the parsed form of Interval — populated by Validate().
	IntervalDuration time.Duration `yaml:"-"`
}

// CPUThreshold triggers an alert when the container's CPU usage >= Threshold percent.
type CPUThreshold struct {
	// Threshold is the CPU usage percentage (0–100) that triggers an alert.
	Threshold float64 `yaml:"threshold"`
}

// MemThreshold triggers an alert when EITHER the percent OR absolute condition is met.
type MemThreshold struct {
	// Percent triggers an alert when memory usage >= this percentage of the container's limit.
	Percent float64 `yaml:"percent"`
	// Absolute triggers an alert when memory usage >= this value in MB.
	Absolute float64 `yaml:"absolute"`
}

// NotifyConfig holds ntfy connection details.
type NotifyConfig struct {
	// NtfyTopic is the ntfy topic name (required).
	NtfyTopic string `yaml:"ntfy_topic"`
	// NtfyServer is the ntfy server URL. Defaults to https://ntfy.sh.
	NtfyServer string `yaml:"ntfy_server"`
	// NtfyToken is an optional bearer token for protected topics. Never logged.
	NtfyToken string `yaml:"ntfy_token"`
}

// DefaultConfigPaths defines where Planck looks for planck.yml, in order.
var DefaultConfigPaths = []string{
	"planck.yml",
	"~/.planck.yml",
	"/etc/planck/planck.yml",
}

// Discover returns the first planck.yml path that exists, searching DefaultConfigPaths.
// It returns an empty string if none is found.
func Discover() string {
	for _, p := range DefaultConfigPaths {
		expanded := expandHome(p)
		if _, err := os.Stat(expanded); err == nil {
			return expanded
		}
	}
	return ""
}

// Load reads the config file at path, strictly decodes it, and validates all fields.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %q: %w", path, err)
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // reject unknown keys — strict mode
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config file %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate checks all fields for correctness and populates parsed duration values.
func (c *Config) Validate() error {
	// --- watch.interval ---
	interval := c.Watch.Interval
	if interval == "" {
		interval = "60s"
	}
	d, err := time.ParseDuration(interval)
	if err != nil || d <= 0 {
		return fmt.Errorf("watch.interval %q must be a positive duration (e.g. \"60s\", \"2m\")", interval)
	}
	c.Watch.IntervalDuration = d

	// --- watch.alert_cooldown ---
	cooldown := c.Watch.AlertCooldown
	if cooldown == "" {
		cooldown = "10m"
	}
	cd, err := time.ParseDuration(cooldown)
	if err != nil || cd <= 0 {
		return fmt.Errorf("watch.alert_cooldown %q must be a positive duration (e.g. \"10m\", \"1h\")", cooldown)
	}
	c.Watch.CooldownDuration = cd

	// --- alerts ---
	if c.Alerts.ErrorRate.Threshold < 0 || c.Alerts.ErrorRate.Threshold > 100 {
		return fmt.Errorf("alerts.error_rate.threshold must be between 0 and 100, got %.2f", c.Alerts.ErrorRate.Threshold)
	}
	if c.Alerts.P95Latency.Threshold < 0 {
		return fmt.Errorf("alerts.p95_latency.threshold must be >= 0, got %.2f", c.Alerts.P95Latency.Threshold)
	}
	if c.Alerts.RPS < 0 {
		return fmt.Errorf("alerts.rps must be >= 0, got %.2f", c.Alerts.RPS)
	}

	// --- resources ---
	resInterval := c.Resources.Interval
	if resInterval == "" {
		// Default to watch.interval when not explicitly set.
		resInterval = interval
	}
	rd, err := time.ParseDuration(resInterval)
	if err != nil || rd <= 0 {
		return fmt.Errorf("resources.interval %q must be a positive duration (e.g. \"30s\", \"1m\")", resInterval)
	}
	c.Resources.IntervalDuration = rd

	if c.Resources.CPU.Threshold < 0 || c.Resources.CPU.Threshold > 100 {
		return fmt.Errorf("resources.cpu.threshold must be between 0 and 100, got %.2f", c.Resources.CPU.Threshold)
	}
	if c.Resources.Memory.Percent < 0 || c.Resources.Memory.Percent > 100 {
		return fmt.Errorf("resources.memory.percent must be between 0 and 100, got %.2f", c.Resources.Memory.Percent)
	}
	if c.Resources.Memory.Absolute < 0 {
		return fmt.Errorf("resources.memory.absolute must be >= 0, got %.2f", c.Resources.Memory.Absolute)
	}

	// --- notify.ntfy_topic ---
	if c.Notify.NtfyTopic == "" {
		return fmt.Errorf("notify.ntfy_topic is required")
	}
	if !validTopicRe.MatchString(c.Notify.NtfyTopic) {
		return fmt.Errorf("notify.ntfy_topic %q contains invalid characters: only letters, digits, hyphens, and underscores are allowed", c.Notify.NtfyTopic)
	}

	// --- notify.ntfy_server ---
	if c.Notify.NtfyServer == "" {
		c.Notify.NtfyServer = "https://ntfy.sh"
	}
	u, err := url.ParseRequestURI(c.Notify.NtfyServer)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("notify.ntfy_server %q must be a valid http:// or https:// URL", c.Notify.NtfyServer)
	}

	return nil
}

// expandHome replaces a leading "~/" with the user's home directory.
func expandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home + path[1:]
	}
	return path
}
