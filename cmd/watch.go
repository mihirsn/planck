package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/mihirsn/planck/internal/config"
	"github.com/mihirsn/planck/internal/watcher"
)

// watchFlags holds the CLI flags for the watch command.
// Deliberately minimal — all container config and alert thresholds live in planck.yml.
var watchFlags struct {
	configPath string
}

// watchCmd is the `planck watch` command.
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously monitor Docker containers and alert on threshold breaches",
	Long: `> Planck Watch — Continuous monitoring with real-time alerting.

Planck watch polls one or more Docker containers' logs on a configurable interval,
evaluates your alert thresholds, and sends notifications via ntfy or webhook when
a threshold is breached.

All configuration lives in planck.yml. Planck searches for it in:
  1. The path passed to --config
  2. The current working directory (./planck.yml)
  3. Your home directory (~/.planck.yml)
  4. Global config (/etc/planck/planck.yml)

Single-container example (planck.yml):

  watch:
    docker: my-api
    interval: 60s
    alert_cooldown: 10m
    preset: fastapi

  alerts:
    error_rate:
      threshold: 10.0
    p95_latency:
      threshold: 2000

  notify:
    ntfy:
      topic: my-api-alerts

Multi-container example (planck.yml):

  watch:
    interval: 60s
    preset: fastapi          # global default

  alerts:
    error_rate:
      threshold: 10.0        # global default

  notify:
    ntfy:
      topic: my-alerts

  containers:
    - name: my-api           # inherits all global defaults
    - name: my-worker
      preset: express        # override preset for this container
      alerts:
        error_rate:
          threshold: 5.0     # stricter threshold for this container

Usage:
  planck watch                              # reads containers from planck.yml
  planck watch --config /etc/planck/planck.yml`,

	RunE: runWatch,
}

func init() {
	watchCmd.Flags().StringVar(&watchFlags.configPath, "config", "", "Path to planck.yml (default: auto-discover)")
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

	// --- Resolve the container list (handles auto-promote and per-container merging) ---
	containers, err := cfg.ResolveContainers()
	if err != nil {
		return err
	}

	// --- Set up graceful shutdown on Ctrl+C / SIGTERM ---
	stop := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		close(stop)
	}()

	// --- Run the watcher(s) ---
	watcher.RunAll(cfg, containers, cmd.OutOrStdout(), stop)

	return nil
}
