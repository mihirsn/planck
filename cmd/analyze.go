// Package cmd contains all Cobra CLI command definitions for Planck.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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

	// Parsing behaviour.
	scanJSON     bool
	excludePaths []string
	filterStatus string
	filterMethod string
	until        string
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
  planck analyze --docker my-api --tail 1000 --since 1h
  planck analyze --docker my-api --since 3d --preset fastapi

  # Filter by status class or exact code
  planck analyze app.log --filter-status 5xx
  planck analyze --docker my-api --preset fastapi --filter-status 4xx

  # Filter by HTTP method (case-insensitive)
  planck analyze app.log --filter-method POST
  planck analyze --docker my-api --filter-method get

  # Combine multiple filters (AND logic)
  planck analyze app.log --filter-status 5xx --filter-method POST --since 1h
  planck analyze --docker my-api --exclude-path /health --exclude-path /metrics --filter-method GET --filter-status 200 --since 1d

  # Handle logs with text prefixes (e.g. Python "INFO:logger:{...}")
  planck analyze app.log --scan-json

  # Exclude noisy endpoints (repeatable flag)
  planck analyze --docker my-api --preset fastapi --exclude-path /health --exclude-path /metrics

  # Time range filtering
  planck analyze app.log --since 2h --until 2026-05-10T18:00:00Z
  planck analyze --docker my-api --since 3d

  # Limit terminal output (top N endpoints, slow N endpoints)
  planck analyze app.log --top 10 --slow 3

  # Machine-readable JSON output
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
	analyzeCmd.Flags().StringVar(&flags.since, "since", "", "Fetch logs since this duration or timestamp (e.g. 1h, 30m, 3d). For Docker, passed to docker logs; for files, filters by entry timestamp.")

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

	// Parsing behaviour flags.
	analyzeCmd.Flags().BoolVar(&flags.scanJSON, "scan-json", false,
		"Scan each line for the first '{' before parsing (useful for logs with text prefixes like 'INFO:logger:{...}')")
	analyzeCmd.Flags().StringArrayVar(&flags.excludePaths, "exclude-path", nil,
		"Exclude entries whose path starts with this prefix (repeatable: --exclude-path /health --exclude-path /metrics)")
	analyzeCmd.Flags().StringVar(&flags.filterStatus, "filter-status", "",
		"Only include entries matching this status pattern: 2xx, 3xx, 4xx, 5xx, or an exact code (e.g. 200, 404)")
	analyzeCmd.Flags().StringVar(&flags.filterMethod, "filter-method", "",
		"Only include entries matching this HTTP method (e.g. GET, POST, PUT)")
	analyzeCmd.Flags().StringVar(&flags.until, "until", "",
		"Exclude entries after this time. Accepts a duration (e.g. 1h, 30m, 3d) or RFC3339 timestamp")

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

	p := parser.New(fieldMap).
		SetScanJSON(flags.scanJSON).
		SetExcludePaths(flags.excludePaths).
		SetStatusFilter(flags.filterStatus).
		SetMethodFilter(flags.filterMethod)

	// Apply time range filtering for file-based sources.
	// (Docker sources already pre-filter via --since passed to `docker logs`;
	// applying it at the parser level too ensures --since and --until are
	// consistent across both source types.)
	sinceTime, err := parseSinceAsTime(flags.since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --since value %q: %v\n", flags.since, err)
		return nil //nolint:nilerr
	}
	untilTime, err := parseSinceAsTime(flags.until)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --until value %q: %v\n", flags.until, err)
		return nil //nolint:nilerr
	}
	p.SetTimeRange(sinceTime, untilTime)
	result := p.ParseAll(lineCh)

	if len(result.Entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No valid log entries found.")
		if result.PrefixedJSON > 0 {
			fmt.Fprintf(cmd.OutOrStdout(),
				"\nHint: %d line(s) contained JSON prefixed with text (e.g. \"INFO:logger:{...}\").\n"+
					"      Add --scan-json to strip the prefix and parse these lines.\n"+
					"      Or configure your logger with propagate=False (Python) to emit bare JSON.\n",
				result.PrefixedJSON,
			)
		} else if result.Malformed > 0 {
			fmt.Fprintf(cmd.OutOrStdout(),
				"\nHint: %d line(s) were skipped because they could not be parsed as JSON.\n"+
					"      If you are analyzing a Docker container, this often happens if Docker outputs\n"+
					"      an error message (e.g., container not found, or missing sudo permissions).\n"+
					"      Try running the command with sudo, or check the container name.\n",
				result.Malformed,
			)
		}
		return nil
	}

	// Calculate metrics.
	report := metrics.Calculate(result.Entries, metrics.Options{
		TopN:       flags.top,
		SlowN:      flags.slow,
		SourceName: sourceName,
		Malformed:  result.Malformed,
		Excluded:   result.Excluded,
		Filtered:   result.Filtered,
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

// parseSinceAsTime converts a --since / --until string to an absolute time.Time.
// Supported formats (in order of attempt):
//
//	"3d"               → now minus 3 days   (Planck extension, docker logs doesn't support days)
//	"1h", "30m", "5s"  → now minus the Go duration
//	RFC3339 timestamp  → parsed directly as an absolute time
//
// Returns a zero time.Time and nil error when s is empty (no bound set).
func parseSinceAsTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	// Days shorthand: "3d" → 72h ago.
	if strings.HasSuffix(s, "d") {
		n, err := fmt.Sscanf(s[:len(s)-1], "%d", new(int))
		_ = n
		var days int
		if _, err2 := fmt.Sscanf(s, "%dd", &days); err2 == nil && days > 0 {
			return time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour), nil
		}
		_ = err
	}

	// Go duration: "1h", "30m", etc.
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().UTC().Add(-d), nil
	}

	// Absolute RFC3339 timestamp.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unrecognized format %q — use a duration (1h, 30m, 3d) or RFC3339 timestamp", s)
}
