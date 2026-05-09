// Package cmd contains all Cobra CLI command definitions for Planck.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mihirsn/planck/internal/formatter"
	"github.com/mihirsn/planck/internal/metrics"
	"github.com/mihirsn/planck/internal/parser"
	"github.com/mihirsn/planck/internal/source"
)

// analyzeFlags holds all CLI flags for the analyze command.
type analyzeFlags struct {
	docker string
	tail   int
	since  string
	format string
	top    int
	slow   int
}

var flags analyzeFlags

// analyzeCmd is the primary command: planck analyze <file> or planck analyze --docker <container>.
var analyzeCmd = &cobra.Command{
	Use:   "analyze [file]",
	Short: "Analyze application logs and display operational insights",
	Long: `Analyze JSON-formatted application logs from a file or a Docker container.

Supported log format (one JSON object per line):
  {"timestamp":"2026-05-08T14:05:00Z","method":"POST","path":"/invoice","status":200,"latency_ms":120}

Examples:
  # Analyze a local log file
  planck analyze app.log

  # Analyze Docker container logs
  planck analyze --docker invoice-api

  # Fetch only the last 1000 lines from Docker
  planck analyze --docker invoice-api --tail 1000

  # Fetch Docker logs from the last hour
  planck analyze --docker invoice-api --since 1h

  # Output as JSON (pipe-friendly)
  planck analyze app.log --format json

  # Show top 10 endpoints instead of the default 5
  planck analyze app.log --top 10`,
	Args: func(cmd *cobra.Command, args []string) error {
		dockerFlag := cmd.Flags().Lookup("docker").Value.String()
		if dockerFlag == "" && len(args) == 0 {
			return fmt.Errorf("provide a log file path or use --docker <container>")
		}
		if dockerFlag != "" && len(args) > 0 {
			return fmt.Errorf("cannot use both a file argument and --docker flag simultaneously")
		}
		return nil
	},
	RunE: runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVar(&flags.docker, "docker", "", "Docker container name or ID to analyze")
	analyzeCmd.Flags().IntVar(&flags.tail, "tail", 0, "Number of log lines to fetch from Docker (0 = all)")
	analyzeCmd.Flags().StringVar(&flags.since, "since", "", "Fetch Docker logs since duration (e.g. 1h, 30m, 2024-01-01)")
	analyzeCmd.Flags().StringVar(&flags.format, "format", "text", "Output format: text or json")
	analyzeCmd.Flags().IntVar(&flags.top, "top", 5, "Number of top endpoints to display")
	analyzeCmd.Flags().IntVar(&flags.slow, "slow", 5, "Number of slowest endpoints to display")

	rootCmd.AddCommand(analyzeCmd)
}

// runAnalyze is the main handler for the analyze command.
func runAnalyze(cmd *cobra.Command, args []string) error {
	// Validate --format flag value.
	if flags.format != "text" && flags.format != "json" {
		return fmt.Errorf("unsupported format %q: must be 'text' or 'json'", flags.format)
	}

	// Select the appropriate log source.
	var src source.LogSource
	var sourceName string

	if flags.docker != "" {
		dockerSrc, err := source.NewDockerSource(flags.docker, flags.tail, flags.since)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return nil //nolint:nilerr // user-facing errors are printed above
		}
		src = dockerSrc
		sourceName = fmt.Sprintf("Docker container %q", flags.docker)
	} else {
		fileSrc, err := source.NewFileSource(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return nil //nolint:nilerr // user-facing errors are printed above
		}
		src = fileSrc
		sourceName = fmt.Sprintf("file %q", args[0])
	}

	// Stream and parse log lines.
	lineCh, err := src.Stream()
	if err != nil {
		return fmt.Errorf("failed to open log source: %w", err)
	}

	p := parser.New()
	entries, malformed := p.ParseAll(lineCh)

	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No valid log entries found.")
		return nil
	}

	// Calculate metrics.
	report := metrics.Calculate(entries, metrics.Options{
		TopN:       flags.top,
		SlowN:      flags.slow,
		SourceName: sourceName,
		Malformed:  malformed,
	})

	// Render output.
	switch flags.format {
	case "json":
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	default:
		formatter.PrintTerminal(cmd.OutOrStdout(), report)
	}

	return nil
}
