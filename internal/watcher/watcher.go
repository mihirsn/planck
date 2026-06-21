// Package watcher implements the planck watch polling loop and threshold evaluation.
package watcher

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mihirsn/planck/internal/config"
	"github.com/mihirsn/planck/internal/metrics"
	"github.com/mihirsn/planck/internal/models"
	"github.com/mihirsn/planck/internal/notify"
	"github.com/mihirsn/planck/internal/parser"
	"github.com/mihirsn/planck/internal/source"
)

// SourceFunc is a function that returns a new log source for a given container and since duration.
// It is a field on Watcher to allow injection in tests.
type SourceFunc func(container string, tail int, since string, until string) (source.LogSource, error)

// StatsSourceFunc is a function that returns a new stats source for a given container.
// It is a field on Watcher to allow injection in tests.
type StatsSourceFunc func(container string) (source.StatsSource, error)

// Watcher continuously polls a log source, evaluates thresholds, and fires alerts.
type Watcher struct {
	cfg       *config.Config
	container string
	fieldMap  models.FieldMap
	notifier  *notify.Client
	out       io.Writer

	// cooldowns tracks the last time each "endpoint:metric" key fired an alert.
	mu        sync.Mutex
	cooldowns map[string]time.Time

	// NewSource is called on each log poll cycle to create a fresh log source.
	NewSource SourceFunc
	// NewStatsSource is called on each resource poll cycle to create a fresh stats source.
	NewStatsSource StatsSourceFunc
}

// New creates a ready-to-run Watcher.
func New(cfg *config.Config, container string, fieldMap models.FieldMap, out io.Writer) *Watcher {
	return &Watcher{
		cfg:       cfg,
		container: container,
		fieldMap:  fieldMap,
		notifier:  notify.NewClient(cfg.Notify.NtfyServer, cfg.Notify.NtfyTopic, cfg.Notify.NtfyToken),
		out:       out,
		cooldowns: make(map[string]time.Time),
		NewSource: func(container string, tail int, since string, until string) (source.LogSource, error) {
			return source.NewDockerSource(container, tail, since, until)
		},
		NewStatsSource: func(container string) (source.StatsSource, error) {
			return source.NewDockerStatsSource(container)
		},
	}
}

// Run starts the polling loop. It blocks until the context is cancelled via the stop channel.
func (w *Watcher) Run(stop <-chan struct{}) {
	fmt.Fprintf(w.out, "> Planck watch started — container: %q, interval: %s\n", w.container, w.cfg.Watch.IntervalDuration)
	if summary := formatAlerts(w.cfg.Alerts); summary != "" {
		fmt.Fprintf(w.out, "   Alerts: %s\n", summary)
	} else {
		fmt.Fprintln(w.out, "   Alerts: none configured (add thresholds to planck.yml)")
	}
	if summary := formatResourceAlerts(w.cfg.Resources); summary != "" {
		fmt.Fprintf(w.out, "   Resources: %s (interval: %s)\n", summary, w.cfg.Resources.IntervalDuration)
	}
	fmt.Fprintln(w.out)

	// --- Log polling ticker ---
	logTicker := time.NewTicker(w.cfg.Watch.IntervalDuration)
	defer logTicker.Stop()

	// Run an immediate first log poll, then tick.
	w.poll()

	// --- Resource polling goroutine (runs independently if any resource threshold is set) ---
	if w.hasResourceAlerts() {
		go func() {
			resTicker := time.NewTicker(w.cfg.Resources.IntervalDuration)
			defer resTicker.Stop()
			// Run an immediate first resource poll, then tick.
			w.pollResources()
			for {
				select {
				case <-stop:
					return
				case <-resTicker.C:
					w.pollResources()
				}
			}
		}()
	}

	for {
		select {
		case <-stop:
			fmt.Fprintln(w.out, "\n> Planck watch stopped.")
			return
		case <-logTicker.C:
			w.poll()
		}
	}
}

// poll fetches logs for the last interval window and evaluates thresholds.
func (w *Watcher) poll() {
	since := durationToSince(w.cfg.Watch.IntervalDuration)

	src, err := w.NewSource(w.container, 0, since, "")
	if err != nil {
		fmt.Fprintf(w.out, "[%s] ⚠  Failed to open log source: %v\n", timestamp(), err)
		return
	}

	lineCh, err := src.Stream()
	if err != nil {
		fmt.Fprintf(w.out, "[%s] ⚠  Failed to stream logs: %v\n", timestamp(), err)
		return
	}

	p := parser.New(w.fieldMap)
	result := p.ParseAll(lineCh)

	if len(result.Entries) == 0 {
		if result.Malformed > 0 {
			fmt.Fprintf(w.out, "[%s] — quiet window (%d malformed lines skipped)\n", timestamp(), result.Malformed)
		} else {
			fmt.Fprintf(w.out, "[%s] — quiet window\n", timestamp())
		}
		return
	}

	report := metrics.Calculate(result.Entries, metrics.Options{
		TopN:             10,
		SlowN:            10,
		SourceName:       w.container,
		Malformed:        result.Malformed,
		FixedIntervalSec: w.cfg.Watch.IntervalDuration.Seconds(),
	})

	fmt.Fprintf(w.out, "[%s] ✓ Analysed %d requests (%.2f req/s)\n",
		timestamp(), report.TotalRequests, report.AvgRPS)

	w.evaluate(report)
}

// pollResources fetches a single container stats snapshot and evaluates resource thresholds.
func (w *Watcher) pollResources() {
	statsSrc, err := w.NewStatsSource(w.container)
	if err != nil {
		fmt.Fprintf(w.out, "[%s] ⚠  Failed to open stats source: %v\n", timestamp(), err)
		return
	}

	stats, err := statsSrc.Collect()
	if err != nil {
		fmt.Fprintf(w.out, "[%s] ⚠  Failed to collect container stats: %v\n", timestamp(), err)
		return
	}

	fmt.Fprintf(w.out, "[%s] ✓ Resources — CPU: %.1f%% | MEM: %.0fMB / %.0fMB (%.1f%%)\n",
		timestamp(), stats.CPUPercent, stats.MemUsedMB, stats.MemLimitMB, stats.MemPercent)

	w.evaluateResources(stats)
}

// hasResourceAlerts reports whether any resource threshold is configured.
func (w *Watcher) hasResourceAlerts() bool {
	r := w.cfg.Resources
	return r.CPU.Threshold > 0 || r.Memory.Percent > 0 || r.Memory.Absolute > 0
}

// shouldAlertOnPath determines if an endpoint should trigger an alert based on include/exclude paths.
func (w *Watcher) shouldAlertOnPath(path string, rule config.AlertRule) bool {
	// Excludes act as a hard override
	for _, exclude := range rule.ExcludePaths {
		if strings.HasPrefix(path, exclude) {
			return false // Never alert on excluded paths
		}
	}

	// If include_paths is defined, the path MUST match at least one of them
	if len(rule.IncludePaths) > 0 {
		for _, include := range rule.IncludePaths {
			if strings.HasPrefix(path, include) {
				return true // Matched an include path
			}
		}
		return false // Did not match any include paths
	}

	// No include paths defined, so all non-excluded paths are allowed
	return true
}

// evaluate checks the report against configured thresholds and fires alerts as needed.
func (w *Watcher) evaluate(report metrics.Report) {
	cfg := w.cfg.Alerts

	// RPS threshold
	if cfg.RPS > 0 && report.AvgRPS >= cfg.RPS {
		w.maybeAlert(
			"global:rps",
			"Planck - High Traffic",
			fmt.Sprintf("**Container:** %s\n**RPS:** %.2f (Threshold: %.0f)", w.container, report.AvgRPS, cfg.RPS),
		)
	}

	// Per-endpoint: error rate and p95 latency
	for _, ep := range report.ErrorEndpoints {
		if !w.shouldAlertOnPath(ep.Path, cfg.ErrorRate) {
			continue
		}
		if cfg.ErrorRate.Threshold > 0 && ep.ErrorRate >= cfg.ErrorRate.Threshold {
			w.maybeAlert(
				fmt.Sprintf("endpoint:error:%s", ep.Path),
				"Planck: High Error Rate",
				fmt.Sprintf("**Container:** %s\n**Endpoint:** %s\n**Rate:** %.1f%% (Threshold: %.0f%%)", w.container, ep.Path, ep.ErrorRate, cfg.ErrorRate.Threshold),
			)
		}
	}

	for _, ep := range report.SlowEndpoints {
		if !w.shouldAlertOnPath(ep.Path, cfg.P95Latency) {
			continue
		}
		if cfg.P95Latency.Threshold > 0 && float64(ep.P95LatencyMs) >= cfg.P95Latency.Threshold {
			w.maybeAlert(
				fmt.Sprintf("endpoint:latency:%s", ep.Path),
				"Planck: High Latency",
				fmt.Sprintf("**Container:** %s\n**Endpoint:** %s\n**P95 Latency:** %.0fms (Threshold: %.0fms)", w.container, ep.Path, ep.P95LatencyMs, cfg.P95Latency.Threshold),
			)
		}
	}
}

// evaluateResources checks container stats against configured resource thresholds
// and fires alerts as needed. Strategy: instant breach with cooldown (v1).
func (w *Watcher) evaluateResources(stats source.ContainerStats) {
	res := w.cfg.Resources

	// CPU threshold
	if res.CPU.Threshold > 0 && stats.CPUPercent >= res.CPU.Threshold {
		w.maybeAlert(
			"resource:cpu",
			"Planck – High CPU Usage",
			fmt.Sprintf("**Container:** %s\n**CPU:** %.1f%% (Threshold: %.0f%%)",
				w.container, stats.CPUPercent, res.CPU.Threshold),
		)
	}

	// Memory percent threshold
	if res.Memory.Percent > 0 && stats.MemPercent >= res.Memory.Percent {
		w.maybeAlert(
			"resource:memory:percent",
			"Planck – High Memory Usage",
			fmt.Sprintf("**Container:** %s\n**Memory:** %.0fMB / %.0fMB (%.1f%%) — exceeded %.0f%% threshold",
				w.container, stats.MemUsedMB, stats.MemLimitMB, stats.MemPercent, res.Memory.Percent),
		)
	}

	// Memory absolute threshold (MB)
	if res.Memory.Absolute > 0 && stats.MemUsedMB >= res.Memory.Absolute {
		w.maybeAlert(
			"resource:memory:absolute",
			"Planck – High Memory Usage",
			fmt.Sprintf("**Container:** %s\n**Memory:** %.0fMB / %.0fMB — exceeded %.0fMB threshold",
				w.container, stats.MemUsedMB, stats.MemLimitMB, res.Memory.Absolute),
		)
	}
}

// maybeAlert sends a notification only if the cooldown for the given key has expired.
func (w *Watcher) maybeAlert(key, title, message string) {
	w.mu.Lock()
	last, exists := w.cooldowns[key]
	w.mu.Unlock()

	if exists && time.Since(last) < w.cfg.Watch.CooldownDuration {
		return // still within cooldown window, skip
	}

	err := w.notifier.Send(title, message)
	if err != nil {
		fmt.Fprintf(w.out, "     ⚠  Failed to send alert: %v\n", err)
		return
	}

	w.mu.Lock()
	w.cooldowns[key] = time.Now()
	w.mu.Unlock()

	// Print the alert with blank line separators so it stands out from heartbeat lines.
	fmt.Fprintf(w.out, "\n   🚨 Alert sent: %s\n\n", strings.SplitN(message, "\n", 2)[0])
}

// durationToSince converts a time.Duration to the --since string format Planck uses.
func durationToSince(d time.Duration) string {
	return fmt.Sprintf("%.0fs", d.Seconds())
}

// timestamp returns a short UTC timestamp string for log lines.
func timestamp() string {
	return time.Now().UTC().Format("15:04:05")
}

// formatAlerts builds a concise summary of only the thresholds that are actually configured
// (i.e. non-zero). Returns an empty string if no thresholds are set.
func formatAlerts(a config.AlertConfig) string {
	var parts []string
	if a.ErrorRate.Threshold > 0 {
		parts = append(parts, fmt.Sprintf("error_rate≥%.0f%%", a.ErrorRate.Threshold))
	}
	if a.P95Latency.Threshold > 0 {
		parts = append(parts, fmt.Sprintf("p95≥%.0fms", a.P95Latency.Threshold))
	}
	if a.RPS > 0 {
		parts = append(parts, fmt.Sprintf("rps≥%.0f", a.RPS))
	}
	return strings.Join(parts, " | ")
}

// formatResourceAlerts builds a concise summary of configured resource thresholds.
// Returns an empty string if no resource thresholds are set.
func formatResourceAlerts(r config.ResourcesConfig) string {
	var parts []string
	if r.CPU.Threshold > 0 {
		parts = append(parts, fmt.Sprintf("cpu≥%.0f%%", r.CPU.Threshold))
	}
	if r.Memory.Percent > 0 {
		parts = append(parts, fmt.Sprintf("mem≥%.0f%%", r.Memory.Percent))
	}
	if r.Memory.Absolute > 0 {
		parts = append(parts, fmt.Sprintf("mem≥%.0fMB", r.Memory.Absolute))
	}
	return strings.Join(parts, " | ")
}
