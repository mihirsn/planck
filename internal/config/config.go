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

	"github.com/mihirsn/planck/internal/models"
)

// validTopicRe matches only safe ntfy topic names: letters, digits, hyphens, underscores.
var validTopicRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Config is the top-level structure of planck.yml.
type Config struct {
	Watch      WatchConfig       `yaml:"watch"`
	Alerts     AlertConfig       `yaml:"alerts"`
	Resources  ResourcesConfig   `yaml:"resources"`
	Notify     NotifyConfig      `yaml:"notify"`
	Containers []ContainerConfig `yaml:"containers,omitempty"`
}

// WatchConfig controls the polling behaviour.
type WatchConfig struct {
	// Docker is the legacy single-container field.
	// Deprecated in favour of the top-level containers: list.
	// When present and no containers: block is defined, it is automatically
	// promoted to a single-entry container list (backward-compatible).
	Docker string `yaml:"docker,omitempty"`
	// Interval is how often Planck polls logs (e.g. "60s", "2m"). Default: 60s.
	Interval string `yaml:"interval"`
	// AlertCooldown prevents repeated alerts for the same breach (e.g. "10m"). Default: 10m.
	AlertCooldown string `yaml:"alert_cooldown"`
	// Preset is the global default log format preset, same as --preset on analyze.
	// Individual containers can override this with their own preset field.
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
	// Defaults to watch.interval when omitted. Global-only — ignored in per-container overrides.
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

// NotifyConfig holds notification destination settings.
type NotifyConfig struct {
	Ntfy    *NtfyConfig    `yaml:"ntfy,omitempty"`
	Webhook *WebhookConfig `yaml:"webhook,omitempty"`
}

// NtfyConfig holds ntfy connection details.
type NtfyConfig struct {
	// Topic is the ntfy topic name (required).
	Topic string `yaml:"topic"`
	// Server is the ntfy server URL. Defaults to https://ntfy.sh.
	Server string `yaml:"server"`
	// Token is an optional bearer token for protected topics. Never logged.
	Token string `yaml:"token"`
}

// WebhookConfig holds webhook connection details.
type WebhookConfig struct {
	// URL is the destination HTTP endpoint (required).
	URL string `yaml:"url"`
	// Headers is a map of optional HTTP headers. Supports environment variable expansion.
	Headers map[string]string `yaml:"headers,omitempty"`
}

// ContainerConfig holds per-container settings. All fields except Name are optional;
// omitted fields fall back to the global defaults defined at the top level of planck.yml.
type ContainerConfig struct {
	// Name is the Docker container name or ID (required).
	Name string `yaml:"name"`
	// Preset overrides the global watch.preset for this specific container.
	Preset string `yaml:"preset,omitempty"`
	// Alerts overrides the global alert thresholds for this container.
	// Only the fields that are explicitly set override the global value;
	// unset fields inherit from the global alerts config (field-level merge).
	Alerts *AlertConfig `yaml:"alerts,omitempty"`
	// Resources overrides the global resource thresholds for this container.
	// The resources.interval field is global-only and is ignored here.
	Resources *ResourcesConfig `yaml:"resources,omitempty"`
}

// ResolvedContainer is the fully-merged, ready-to-run configuration for a single
// container. It is built by ResolveContainers(), which layers per-container
// overrides on top of global defaults.
type ResolvedContainer struct {
	// Name is the Docker container name or ID.
	Name string
	// Preset is the resolved preset name (per-container > global watch.preset > "" = default).
	Preset string
	// Alerts is the effective alert configuration after merging global + per-container.
	Alerts AlertConfig
	// Resources is the effective resource configuration after merging global + per-container.
	// Resources.IntervalDuration is always sourced from the global config.
	Resources ResourcesConfig
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

	// --- watch.docker + containers: mutual exclusivity ---
	if c.Watch.Docker != "" && len(c.Containers) > 0 {
		return fmt.Errorf(
			"cannot use both watch.docker and containers: in the same config; " +
				"use containers: for multi-container support, or watch.docker for single-container (legacy)",
		)
	}

	// --- global preset ---
	if c.Watch.Preset != "" {
		if _, err := models.PresetFieldMap(c.Watch.Preset); err != nil {
			return fmt.Errorf("watch.preset: %w", err)
		}
	}

	// --- global alerts ---
	if err := validateAlertConfig(c.Alerts, "alerts"); err != nil {
		return err
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

	// --- per-container configs ---
	for i, cc := range c.Containers {
		if cc.Name == "" {
			return fmt.Errorf("containers[%d]: name is required", i)
		}
		if cc.Preset != "" {
			if _, err := models.PresetFieldMap(cc.Preset); err != nil {
				return fmt.Errorf("containers[%s].preset: %w", cc.Name, err)
			}
		}
		if cc.Alerts != nil {
			if err := validateAlertConfig(*cc.Alerts, fmt.Sprintf("containers[%s].alerts", cc.Name)); err != nil {
				return err
			}
		}
		if cc.Resources != nil {
			if cc.Resources.CPU.Threshold < 0 || cc.Resources.CPU.Threshold > 100 {
				return fmt.Errorf("containers[%s].resources.cpu.threshold must be between 0 and 100, got %.2f",
					cc.Name, cc.Resources.CPU.Threshold)
			}
			if cc.Resources.Memory.Percent < 0 || cc.Resources.Memory.Percent > 100 {
				return fmt.Errorf("containers[%s].resources.memory.percent must be between 0 and 100, got %.2f",
					cc.Name, cc.Resources.Memory.Percent)
			}
			if cc.Resources.Memory.Absolute < 0 {
				return fmt.Errorf("containers[%s].resources.memory.absolute must be >= 0, got %.2f",
					cc.Name, cc.Resources.Memory.Absolute)
			}
		}
	}

	// --- notify ---
	if c.Notify.Ntfy == nil && c.Notify.Webhook == nil {
		return fmt.Errorf("notify requires at least one destination (ntfy or webhook)")
	}

	if c.Notify.Ntfy != nil {
		if c.Notify.Ntfy.Topic == "" {
			return fmt.Errorf("notify.ntfy.topic is required")
		}
		if !validTopicRe.MatchString(c.Notify.Ntfy.Topic) {
			return fmt.Errorf("notify.ntfy.topic %q contains invalid characters: only letters, digits, hyphens, and underscores are allowed", c.Notify.Ntfy.Topic)
		}
		if c.Notify.Ntfy.Server == "" {
			c.Notify.Ntfy.Server = "https://ntfy.sh"
		}
		u, err := url.ParseRequestURI(c.Notify.Ntfy.Server)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("notify.ntfy.server %q must be a valid http:// or https:// URL", c.Notify.Ntfy.Server)
		}
	}

	if c.Notify.Webhook != nil {
		if c.Notify.Webhook.URL == "" {
			return fmt.Errorf("notify.webhook.url is required")
		}
		u, err := url.ParseRequestURI(c.Notify.Webhook.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("notify.webhook.url %q must be a valid http:// or https:// URL", c.Notify.Webhook.URL)
		}
		// Expand environment variables in headers (e.g. ${API_KEY})
		for k, v := range c.Notify.Webhook.Headers {
			c.Notify.Webhook.Headers[k] = os.ExpandEnv(v)
		}
	}

	return nil
}

// validateAlertConfig validates an AlertConfig, using prefix in error messages.
func validateAlertConfig(a AlertConfig, prefix string) error {
	if a.ErrorRate.Threshold < 0 || a.ErrorRate.Threshold > 100 {
		return fmt.Errorf("%s.error_rate.threshold must be between 0 and 100, got %.2f", prefix, a.ErrorRate.Threshold)
	}
	if a.P95Latency.Threshold < 0 {
		return fmt.Errorf("%s.p95_latency.threshold must be >= 0, got %.2f", prefix, a.P95Latency.Threshold)
	}
	if a.RPS < 0 {
		return fmt.Errorf("%s.rps must be >= 0, got %.2f", prefix, a.RPS)
	}
	return nil
}

// ResolveContainers returns the fully-merged list of ResolvedContainers to monitor.
//
// Resolution rules:
//   - If watch.docker is set and no containers: block exists, the single container
//     is auto-promoted (backward-compatible with the legacy single-container format).
//   - Each container in containers: is merged with the global defaults.
//   - watch.docker and containers: set simultaneously is an error (caught in Validate).
//   - Duplicate container names are rejected.
//   - An empty container list is an error.
func (c *Config) ResolveContainers() ([]ResolvedContainer, error) {
	// Build raw container list, handling legacy auto-promote.
	var rawContainers []ContainerConfig
	if c.Watch.Docker != "" {
		rawContainers = []ContainerConfig{{Name: c.Watch.Docker}}
	} else {
		rawContainers = c.Containers
	}

	if len(rawContainers) == 0 {
		return nil, fmt.Errorf(
			"no containers configured.\n" +
				"Add a containers: list to planck.yml, or use watch.docker for single-container (legacy).\n" +
				"See: https://github.com/mihirsn/planck/blob/main/docs/configuration/watch-mode.md",
		)
	}

	// Reject duplicate names.
	seen := make(map[string]bool, len(rawContainers))
	for i, cc := range rawContainers {
		if cc.Name == "" {
			return nil, fmt.Errorf("containers[%d]: name is required", i)
		}
		if seen[cc.Name] {
			return nil, fmt.Errorf("duplicate container name %q in containers list", cc.Name)
		}
		seen[cc.Name] = true
	}

	// Resolve each container against the global defaults.
	resolved := make([]ResolvedContainer, 0, len(rawContainers))
	for _, cc := range rawContainers {
		resolved = append(resolved, c.resolveContainer(cc))
	}

	return resolved, nil
}

// resolveContainer merges the global defaults with per-container overrides to produce
// a fully self-contained ResolvedContainer ready for the watcher.
func (c *Config) resolveContainer(cc ContainerConfig) ResolvedContainer {
	// Preset: per-container > global watch.preset > "" (caller uses DefaultFieldMap).
	preset := cc.Preset
	if preset == "" {
		preset = c.Watch.Preset
	}

	return ResolvedContainer{
		Name:      cc.Name,
		Preset:    preset,
		Alerts:    mergeAlerts(c.Alerts, cc.Alerts),
		Resources: mergeResources(c.Resources, cc.Resources),
	}
}

// mergeAlerts returns an AlertConfig composed of per-container override fields layered
// on top of the global defaults. Merge is field-level within each AlertRule, so a
// container can override just the threshold without losing global exclude/include paths.
func mergeAlerts(global AlertConfig, override *AlertConfig) AlertConfig {
	if override == nil {
		return global
	}
	return AlertConfig{
		ErrorRate:  mergeAlertRule(global.ErrorRate, override.ErrorRate),
		P95Latency: mergeAlertRule(global.P95Latency, override.P95Latency),
		RPS:        mergeFloat64(global.RPS, override.RPS),
	}
}

// mergeAlertRule returns an AlertRule where each field is taken from override only
// when it is explicitly set (non-zero / non-nil), otherwise falls back to global.
func mergeAlertRule(global, override AlertRule) AlertRule {
	result := global
	if override.Threshold != 0 {
		result.Threshold = override.Threshold
	}
	if override.ExcludePaths != nil {
		result.ExcludePaths = override.ExcludePaths
	}
	if override.IncludePaths != nil {
		result.IncludePaths = override.IncludePaths
	}
	return result
}

// mergeResources returns a ResourcesConfig where threshold fields are taken from
// override when non-zero, otherwise from global. Interval and IntervalDuration
// are always taken from the global config (interval is a global-only setting).
func mergeResources(global ResourcesConfig, override *ResourcesConfig) ResourcesConfig {
	if override == nil {
		return global
	}
	// Start from global to preserve IntervalDuration and Interval.
	result := global
	if override.CPU.Threshold != 0 {
		result.CPU.Threshold = override.CPU.Threshold
	}
	if override.Memory.Percent != 0 {
		result.Memory.Percent = override.Memory.Percent
	}
	if override.Memory.Absolute != 0 {
		result.Memory.Absolute = override.Memory.Absolute
	}
	return result
}

// mergeFloat64 returns override if non-zero, otherwise global.
func mergeFloat64(global, override float64) float64 {
	if override != 0 {
		return override
	}
	return global
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
