// Package cmd contains all Cobra CLI command definitions for Planck.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mihirsn/planck/internal/formatter"
	"github.com/mihirsn/planck/internal/metrics"
	"github.com/mihirsn/planck/internal/models"
	"github.com/mihirsn/planck/internal/parser"
	"github.com/mihirsn/planck/internal/source"
)

// analyzeFlags holds all CLI flags for the analyze command.
type analyzeFlags struct {
	// Source selection.
	docker string
	tail   int
	since  string

	// Output.
	format string
	top    int
	slow   int

	// Field mapping.
	preset         string
	fieldTimestamp string
	fieldMethod    string
	fieldPath      string
	fieldStatus    string
	fieldLatency   string
}

var flags analyzeFlags

// analyzeCmd is the primary command: planck analyze <file> or planck analyze --docker <container>.
var analyzeCmd = &cobra.Command{
	Use:   "analyze [file]",
	Short: "Analyze application logs and display operational insights",
	Long: `Analyze JSON-formatted application logs from a file or a Docker container.

Planck supports any JSON log schema via field mapping flags and presets.

Default schema (one JSON object per line):
  {"timestamp":"2026-05-08T14:05:00Z","method":"POST","path":"/invoice","status":200,"latency_ms":120}

Built-in presets:
  fastapi   FastAPI / uvicorn (status_code, duration as float seconds)
  express   Express.js with morgan JSON middleware (url, statusCode, responseTime)
  gin       Go Gin with JSON logger middleware (time, status, latency)
  echo      Go Echo with JSON logger middleware (time, uri, status, latency)
  spring    Spring Boot with custom JSON HTTP filter (uri, status, duration)

Examples:
  # Analyze with default schema
  planck analyze app.log

  # Use a built-in preset
  planck analyze app.log --preset fastapi
  planck analyze --docker my-api --preset express

  # Custom field mapping
  planck analyze app.log --field-path url --field-status code --field-latency dur

  # Mix: preset + override one field
  planck analyze app.log --preset fastapi --field-latency process_time_ms

  # Docker with options
  planck analyze --docker invoice-api --tail 1000 --since 1h

  # JSON output
  planck analyze app.log --format json | jq '.top_endpoints'`,
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
	// Source flags.
	analyzeCmd.Flags().StringVar(&flags.docker, "docker", "", "Docker container name or ID to analyze")
	analyzeCmd.Flags().IntVar(&flags.tail, "tail", 0, "Number of log lines to fetch from Docker (0 = all)")
	analyzeCmd.Flags().StringVar(&flags.since, "since", "", "Fetch Docker logs since duration (e.g. 1h, 30m)")

	// Output flags.
	analyzeCmd.Flags().StringVar(&flags.format, "format", "text", "Output format: text or json")
	analyzeCmd.Flags().IntVar(&flags.top, "top", 5, "Number of top endpoints to display")
	analyzeCmd.Flags().IntVar(&flags.slow, "slow", 5, "Number of slowest endpoints to display")

	// Field mapping flags.
	analyzeCmd.Flags().StringVar(&flags.preset, "preset", "",
		fmt.Sprintf("Field mapping preset for a known framework (%s)", strings.Join(models.AvailablePresets(), ", ")))
	analyzeCmd.Flags().StringVar(&flags.fieldTimestamp, "field-timestamp", "", "JSON key for the timestamp field (overrides preset)")
	analyzeCmd.Flags().StringVar(&flags.fieldMethod, "field-method", "", "JSON key for the HTTP method field (overrides preset)")
	analyzeCmd.Flags().StringVar(&flags.fieldPath, "field-path", "", "JSON key for the request path field (overrides preset)")
	analyzeCmd.Flags().StringVar(&flags.fieldStatus, "field-status", "", "JSON key for the HTTP status code field (overrides preset)")
	analyzeCmd.Flags().StringVar(&flags.fieldLatency, "field-latency", "", "JSON key for the latency field (overrides preset)")

	rootCmd.AddCommand(analyzeCmd)
}

// runAnalyze is the main handler for the analyze command.
func runAnalyze(cmd *cobra.Command, args []string) error {
	// Validate --format flag.
	if flags.format != "text" && flags.format != "json" {
		return fmt.Errorf("unsupported format %q: must be 'text' or 'json'", flags.format)
	}

	// Build the FieldMap: start from preset (or default), then apply overrides.
	fieldMap, err := buildFieldMap()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil //nolint:nilerr
	}

	// Select the appropriate log source.
	var src source.LogSource
	var sourceName string

	if flags.docker != "" {
		dockerSrc, err := source.NewDockerSource(flags.docker, flags.tail, flags.since)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return nil //nolint:nilerr
		}
		src = dockerSrc
		sourceName = fmt.Sprintf("Docker container %q", flags.docker)
	} else {
		fileSrc, err := source.NewFileSource(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return nil //nolint:nilerr
		}
		src = fileSrc
		sourceName = fmt.Sprintf("file %q", args[0])
	}

	// Stream and parse log lines.
	lineCh, err := src.Stream()
	if err != nil {
		return fmt.Errorf("failed to open log source: %w", err)
	}

	p := parser.New(fieldMap)
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

// buildFieldMap constructs a FieldMap by loading the preset (if set) and
// then applying any individual --field-* overrides on top.
// Resolution order (highest priority first): --field-* flags → preset → default.
func buildFieldMap() (models.FieldMap, error) {
	fm, err := models.PresetFieldMap(flags.preset)
	if err != nil {
		return models.FieldMap{}, err
	}

	// Individual flags override preset values when explicitly set.
	if flags.fieldTimestamp != "" {
		fm.Timestamp = flags.fieldTimestamp
	}
	if flags.fieldMethod != "" {
		fm.Method = flags.fieldMethod
	}
	if flags.fieldPath != "" {
		fm.Path = flags.fieldPath
	}
	if flags.fieldStatus != "" {
		fm.Status = flags.fieldStatus
	}
	if flags.fieldLatency != "" {
		fm.LatencyMs = flags.fieldLatency
	}

	return fm, nil
}
