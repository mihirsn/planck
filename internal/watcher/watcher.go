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
	// globalCfg provides shared global settings: poll interval, cooldown, notify destination.
	globalCfg *config.Config
	// container is the Docker container name or ID being monitored.
	container string
	// fieldMap maps Planck's canonical field names to the container's actual JSON keys.
	fieldMap models.FieldMap
	// alerts is the effective alert configuration for this container
	// (global defaults merged with per-container overrides).
	alerts config.AlertConfig
	// resources is the effective resource configuration for this container
	// (global defaults merged with per-container overrides).
	resources config.ResourcesConfig
	// notifier dispatches alert payloads to all configured destinations.
	notifier *notify.Client
	// out is the writer for all log and status output.
	out io.Writer
	// label is the container name prefix shown in multi-container output, e.g. "[my-api]".
	// An empty label means no prefix (single-container or legacy watch.docker mode).
	label string

	// mu guards the cooldowns map for concurrent access.
	mu        sync.Mutex
	cooldowns map[string]time.Time

	// NewSource is called on each log poll cycle to create a fresh log source.
	// Exposed as a field to allow injection in tests.
	NewSource SourceFunc
	// NewStatsSource is called on each resource poll cycle to create a fresh stats source.
	// Exposed as a field to allow injection in tests.
	NewStatsSource StatsSourceFunc
}

// New creates a ready-to-run Watcher for a single resolved container.
//
//   - globalCfg provides the shared global settings (interval, cooldown, notify).
//   - rc is the fully-resolved per-container configuration (alerts, resources, preset).
//   - fieldMap is the resolved JSON field map for parsing this container's logs.
//   - label is the prefix added to each output line in multi-container mode ("" = no prefix).
//   - out is the writer for all log output.
func New(globalCfg *config.Config, rc config.ResolvedContainer, fieldMap models.FieldMap, label string, out io.Writer) *Watcher {
	return &Watcher{
		globalCfg: globalCfg,
		container: rc.Name,
		fieldMap:  fieldMap,
		alerts:    rc.Alerts,
		resources: rc.Resources,
		label:     label,
		notifier:  notify.NewClient(globalCfg.Notify, out),
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

// Run starts the watcher for a single container, prints a startup message, and
// blocks until stop is closed. Suitable for single-container use and unit tests.
// For multi-container production use, prefer RunAll.
func (w *Watcher) Run(stop <-chan struct{}) {
	fmt.Fprintf(w.out, "> Planck watch started — container: %q, interval: %s\n", w.container, w.globalCfg.Watch.IntervalDuration)
	if summary := formatAlerts(w.alerts); summary != "" {
		fmt.Fprintf(w.out, "   Alerts: %s\n", summary)
	} else {
		fmt.Fprintln(w.out, "   Alerts: none configured (add thresholds to planck.yml)")
	}
	if summary := formatResourceAlerts(w.resources); summary != "" {
		fmt.Fprintf(w.out, "   Resources: %s (interval: %s)\n", summary, w.globalCfg.Resources.IntervalDuration)
	}
	fmt.Fprintln(w.out)

	w.runLoop(stop)

	fmt.Fprintln(w.out, "\n> Planck watch stopped.")
}

// runLoop is the internal polling loop. It blocks until stop is closed.
// Unlike Run, it does not print startup or shutdown messages — those are
// handled by the caller (Run for single-container, RunAll for multi-container).
func (w *Watcher) runLoop(stop <-chan struct{}) {
	// --- Log polling ticker ---
	logTicker := time.NewTicker(w.globalCfg.Watch.IntervalDuration)
	defer logTicker.Stop()

	// Run an immediate first log poll, then tick.
	w.poll()

	// --- Resource polling goroutine (runs independently if any resource threshold is set) ---
	if w.hasResourceAlerts() {
		go func() {
			resTicker := time.NewTicker(w.globalCfg.Resources.IntervalDuration)
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
			return
		case <-logTicker.C:
			w.poll()
		}
	}
}

// RunAll monitors multiple containers concurrently. It prints a combined startup
// message, spawns one goroutine per container, and blocks until stop is closed.
// Container names are prefixed on each output line when monitoring more than one container.
func RunAll(cfg *config.Config, containers []config.ResolvedContainer, out io.Writer, stop <-chan struct{}) {
	useLabels := len(containers) > 1

	// --- Startup message ---
	if useLabels {
		names := make([]string, len(containers))
		for i, rc := range containers {
			names[i] = rc.Name
		}
		fmt.Fprintf(out, "> Planck watch started — %d containers, interval: %s\n",
			len(containers), cfg.Watch.IntervalDuration)
		fmt.Fprintf(out, "   Containers: %s\n", strings.Join(names, ", "))
	} else {
		fmt.Fprintf(out, "> Planck watch started — container: %q, interval: %s\n",
			containers[0].Name, cfg.Watch.IntervalDuration)
	}
	if summary := formatAlerts(cfg.Alerts); summary != "" {
		fmt.Fprintf(out, "   Alerts: %s\n", summary)
	} else {
		fmt.Fprintln(out, "   Alerts: none configured (add thresholds to planck.yml)")
	}
	if summary := formatResourceAlerts(cfg.Resources); summary != "" {
		fmt.Fprintf(out, "   Resources: %s (interval: %s)\n", summary, cfg.Resources.IntervalDuration)
	}
	fmt.Fprintln(out)

	// --- Spawn one goroutine per container ---
	var wg sync.WaitGroup
	for _, rc := range containers {
		label := ""
		if useLabels {
			label = rc.Name
		}
		fieldMap := resolveFieldMap(rc.Preset)
		w := New(cfg, rc, fieldMap, label, out)

		wg.Add(1)
		go func(w *Watcher) {
			defer wg.Done()
			w.runLoop(stop)
		}(w)
	}

	wg.Wait()
	fmt.Fprintln(out, "\n> Planck watch stopped.")
}

// resolveFieldMap returns the FieldMap for the given preset name.
// Falls back to DefaultFieldMap if preset is empty or unrecognised (preset is
// already validated at config load time, so this path is a safety fallback only).
func resolveFieldMap(preset string) models.FieldMap {
	if preset != "" {
		if fm, err := models.PresetFieldMap(preset); err == nil {
			return fm
		}
	}
	return models.DefaultFieldMap()
}

// ts returns the timestamp prefix for a log line.
//
//	With label:    "[14:05:01] [my-api]"
//	Without label: "[14:05:01]"
func (w *Watcher) ts() string {
	t := time.Now().UTC().Format("15:04:05")
	if w.label != "" {
		return fmt.Sprintf("[%s] [%s]", t, w.label)
	}
	return fmt.Sprintf("[%s]", t)
}

// poll fetches logs for the last interval window and evaluates thresholds.
func (w *Watcher) poll() {
	since := durationToSince(w.globalCfg.Watch.IntervalDuration)

	src, err := w.NewSource(w.container, 0, since, "")
	if err != nil {
		fmt.Fprintf(w.out, "%s ⚠  Failed to open log source: %v\n", w.ts(), err)
		return
	}

	lineCh, err := src.Stream()
	if err != nil {
		fmt.Fprintf(w.out, "%s ⚠  Failed to stream logs: %v\n", w.ts(), err)
		return
	}

	p := parser.New(w.fieldMap)
	result := p.ParseAll(lineCh)

	if len(result.Entries) == 0 {
		if result.Malformed > 0 {
			fmt.Fprintf(w.out, "%s — quiet window (%d malformed lines skipped)\n", w.ts(), result.Malformed)
		} else {
			fmt.Fprintf(w.out, "%s — quiet window\n", w.ts())
		}
		return
	}

	report := metrics.Calculate(result.Entries, metrics.Options{
		TopN:             10,
		SlowN:            10,
		SourceName:       w.container,
		Malformed:        result.Malformed,
		FixedIntervalSec: w.globalCfg.Watch.IntervalDuration.Seconds(),
	})

	fmt.Fprintf(w.out, "%s ✓ Analysed %d requests (%.2f req/s)\n",
		w.ts(), report.TotalRequests, report.AvgRPS)

	w.evaluate(report)
}

// pollResources fetches a single container stats snapshot and evaluates resource thresholds.
func (w *Watcher) pollResources() {
	statsSrc, err := w.NewStatsSource(w.container)
	if err != nil {
		fmt.Fprintf(w.out, "%s ⚠  Failed to open stats source: %v\n", w.ts(), err)
		return
	}

	stats, err := statsSrc.Collect()
	if err != nil {
		fmt.Fprintf(w.out, "%s ⚠  Failed to collect container stats: %v\n", w.ts(), err)
		return
	}

	fmt.Fprintf(w.out, "%s ✓ Resources — CPU: %.1f%% | MEM: %.0fMB / %.0fMB (%.1f%%)\n",
		w.ts(), stats.CPUPercent, stats.MemUsedMB, stats.MemLimitMB, stats.MemPercent)

	w.evaluateResources(stats)
}

// hasResourceAlerts reports whether any resource threshold is configured for this container.
func (w *Watcher) hasResourceAlerts() bool {
	r := w.resources
	return r.CPU.Threshold > 0 || r.Memory.Percent > 0 || r.Memory.Absolute > 0
}

// shouldAlertOnPath determines if an endpoint should trigger an alert based on include/exclude paths.
func (w *Watcher) shouldAlertOnPath(path string, rule config.AlertRule) bool {
	// Excludes act as a hard override.
	for _, exclude := range rule.ExcludePaths {
		if strings.HasPrefix(path, exclude) {
			return false
		}
	}

	// If include_paths is defined, the path MUST match at least one of them.
	if len(rule.IncludePaths) > 0 {
		for _, include := range rule.IncludePaths {
			if strings.HasPrefix(path, include) {
				return true
			}
		}
		return false
	}

	// No include paths defined — all non-excluded paths are eligible.
	return true
}

// evaluate checks the report against configured thresholds and fires alerts as needed.
func (w *Watcher) evaluate(report metrics.Report) {
	cfg := w.alerts

	// RPS threshold
	if cfg.RPS > 0 && report.AvgRPS >= cfg.RPS {
		w.maybeAlert(
			"global:rps",
			notify.AlertPayload{
				Container: w.container,
				Type:      "rps",
				Title:     "Planck - High Traffic",
				Value:     report.AvgRPS,
				Threshold: cfg.RPS,
				Unit:      "req/s",
				Message:   fmt.Sprintf("**Container:** %s\n**RPS:** %.2f (Threshold: %.0f)", w.container, report.AvgRPS, cfg.RPS),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		)
	}

	// Per-endpoint: error rate
	for _, ep := range report.ErrorEndpoints {
		if !w.shouldAlertOnPath(ep.Path, cfg.ErrorRate) {
			continue
		}
		if cfg.ErrorRate.Threshold > 0 && ep.ErrorRate >= cfg.ErrorRate.Threshold {
			w.maybeAlert(
				fmt.Sprintf("endpoint:error:%s", ep.Path),
				notify.AlertPayload{
					Container: w.container,
					Type:      "error_rate",
					Title:     "Planck: High Error Rate",
					Endpoint:  ep.Path,
					Value:     ep.ErrorRate,
					Threshold: cfg.ErrorRate.Threshold,
					Unit:      "%",
					Message: fmt.Sprintf("**Container:** %s\n**Endpoint:** %s\n**Rate:** %.1f%% (Threshold: %.0f%%)",
						w.container, ep.Path, ep.ErrorRate, cfg.ErrorRate.Threshold),
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				},
			)
		}
	}

	// Per-endpoint: p95 latency
	for _, ep := range report.SlowEndpoints {
		if !w.shouldAlertOnPath(ep.Path, cfg.P95Latency) {
			continue
		}
		if cfg.P95Latency.Threshold > 0 && float64(ep.P95LatencyMs) >= cfg.P95Latency.Threshold {
			w.maybeAlert(
				fmt.Sprintf("endpoint:latency:%s", ep.Path),
				notify.AlertPayload{
					Container: w.container,
					Type:      "p95_latency",
					Title:     "Planck: High Latency",
					Endpoint:  ep.Path,
					Value:     float64(ep.P95LatencyMs),
					Threshold: cfg.P95Latency.Threshold,
					Unit:      "ms",
					Message: fmt.Sprintf("**Container:** %s\n**Endpoint:** %s\n**P95 Latency:** %.0fms (Threshold: %.0fms)",
						w.container, ep.Path, ep.P95LatencyMs, cfg.P95Latency.Threshold),
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				},
			)
		}
	}
}

// evaluateResources checks container stats against configured resource thresholds
// and fires alerts as needed.
func (w *Watcher) evaluateResources(stats source.ContainerStats) {
	res := w.resources

	// CPU threshold
	if res.CPU.Threshold > 0 && stats.CPUPercent >= res.CPU.Threshold {
		w.maybeAlert(
			"resource:cpu",
			notify.AlertPayload{
				Container: w.container,
				Type:      "cpu",
				Title:     "Planck – High CPU Usage",
				Value:     stats.CPUPercent,
				Threshold: res.CPU.Threshold,
				Unit:      "%",
				Message: fmt.Sprintf("**Container:** %s\n**CPU:** %.1f%% (Threshold: %.0f%%)",
					w.container, stats.CPUPercent, res.CPU.Threshold),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		)
	}

	// Memory percent threshold
	if res.Memory.Percent > 0 && stats.MemPercent >= res.Memory.Percent {
		w.maybeAlert(
			"resource:memory:percent",
			notify.AlertPayload{
				Container: w.container,
				Type:      "memory",
				Title:     "Planck – High Memory Usage",
				Value:     stats.MemPercent,
				Threshold: res.Memory.Percent,
				Unit:      "%",
				Message: fmt.Sprintf("**Container:** %s\n**Memory:** %.0fMB / %.0fMB (%.1f%%) (Threshold: %.0f%%)",
					w.container, stats.MemUsedMB, stats.MemLimitMB, stats.MemPercent, res.Memory.Percent),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		)
	}

	// Memory absolute threshold (MB)
	if res.Memory.Absolute > 0 && stats.MemUsedMB >= res.Memory.Absolute {
		w.maybeAlert(
			"resource:memory:absolute",
			notify.AlertPayload{
				Container: w.container,
				Type:      "memory",
				Title:     "Planck – High Memory Usage",
				Value:     stats.MemUsedMB,
				Threshold: res.Memory.Absolute,
				Unit:      "MB",
				Message: fmt.Sprintf("**Container:** %s\n**Memory:** %.0fMB / %.0fMB (Threshold: %.0fMB)",
					w.container, stats.MemUsedMB, stats.MemLimitMB, res.Memory.Absolute),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		)
	}
}

// maybeAlert sends a notification only if the cooldown for the given key has expired.
func (w *Watcher) maybeAlert(key string, payload notify.AlertPayload) {
	w.mu.Lock()
	last, exists := w.cooldowns[key]
	w.mu.Unlock()

	if exists && time.Since(last) < w.globalCfg.Watch.CooldownDuration {
		return // still within cooldown window, skip
	}

	// Dispatch notification (non-blocking).
	w.notifier.Send(payload)

	w.mu.Lock()
	w.cooldowns[key] = time.Now()
	w.mu.Unlock()

	// Print the alert with blank line separators so it stands out from heartbeat lines.
	fmt.Fprintf(w.out, "\n   🚨 Alert sent: %s\n\n", strings.SplitN(payload.Message, "\n", 2)[0])
}

// durationToSince converts a time.Duration to the --since string format Planck uses.
func durationToSince(d time.Duration) string {
	return fmt.Sprintf("%.0fs", d.Seconds())
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
