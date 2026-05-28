package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mihirsn/planck/internal/config"
	"github.com/mihirsn/planck/internal/models"
	"github.com/mihirsn/planck/internal/watcher"
	"github.com/spf13/cobra"
)

// watchFlags holds the CLI flags for the watch command.
// Deliberately minimal — all alert thresholds live in planck.yml.
var watchFlags struct {
	docker     string
	configPath string
}

// watchCmd is the `planck watch` command.
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously monitor a Docker container and alert on threshold breaches",
	Long: `> Planck Watch — Continuous monitoring with real-time alerting.

Planck watch polls a Docker container's logs on a configurable interval,
evaluates your alert thresholds, and sends notifications via ntfy when
a threshold is breached.

All configuration lives in planck.yml. Planck searches for it in:
  1. The path passed to --config
  2. The current working directory (./planck.yml)
  3. Your home directory (~/.planck.yml)

Example planck.yml:

  watch:
    interval: 60s
    alert_cooldown: 10m
    preset: fastapi

  alerts:
    error_rate_pct: 10.0
    p95_latency_ms: 2000
    rps: 100

  notify:
    ntfy_topic: my-api-alerts
    ntfy_server: https://ntfy.sh

Usage:
  planck watch --docker my-api
  planck watch --docker my-api --config /etc/planck/planck.yml`,

	RunE: runWatch,
}

func init() {
	watchCmd.Flags().StringVar(&watchFlags.docker, "docker", "", "Docker container name or ID to watch (required)")
	watchCmd.Flags().StringVar(&watchFlags.configPath, "config", "", "Path to planck.yml (default: auto-discover)")
	_ = watchCmd.MarkFlagRequired("docker")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, _ []string) error {
	// --- Resolve config file ---
	cfgPath := watchFlags.configPath
	if cfgPath == "" {
		cfgPath = config.Discover()
	}
	if cfgPath == "" {
		return fmt.Errorf(
			"planck.yml not found.\n" +
				"Create one in the current directory or pass --config <path>.\n" +
				"See: https://github.com/mihirsn/planck#watch-mode",
		)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	// --- Build field map from preset ---
	var fieldMap models.FieldMap
	if cfg.Watch.Preset != "" {
		fm, err := models.PresetFieldMap(cfg.Watch.Preset)
		if err != nil {
			return fmt.Errorf("invalid preset %q in planck.yml: %w", cfg.Watch.Preset, err)
		}
		fieldMap = fm
	} else {
		// Default field map (standard JSON keys)
		fieldMap = models.FieldMap{
			Timestamp: "timestamp",
			Method:    "method",
			Path:      "path",
			Status:    "status",
			LatencyMs: "latency_ms",
		}
	}

	// --- Set up graceful shutdown on Ctrl+C / SIGTERM ---
	stop := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		close(stop)
	}()

	// --- Run the watcher ---
	w := watcher.New(cfg, watchFlags.docker, fieldMap, cmd.OutOrStdout())
	w.Run(stop)

	return nil
}
