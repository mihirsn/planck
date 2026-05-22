# Changelog

All notable changes to Planck will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [0.1.4] - 2026-05-22

### Added
- **Requests Per Second (RPS)** — new calculation in the report header based purely on the filtered logs.
- **`--filter-method` flag** — restrict analysis to logs matching a specific HTTP method (e.g. `GET`, `POST`).
- **`--exclude-status` blocklist flag** — explicitly exclude entries by status code or pattern (repeatable: `--exclude-status 404 --exclude-status 3xx`).
- **`--exclude-method` blocklist flag** — explicitly exclude entries by HTTP method (repeatable: `--exclude-method OPTIONS --exclude-method HEAD`).
- **Contextual Hints** — clearer terminal output distinguishing when 0 logs match your filters versus when 0 logs could be properly JSON parsed.

### Fixed
- **Project Test Coverage** — dramatically increased core and CLI test coverage above 90%.

---

## [0.1.3] - 2026-05-16

### Added
- **`--scan-json` flag** — scans each log line for the first `{` before parsing, handling logs with text prefixes (e.g. Python's `INFO:logger:{...}` format)
- **Prefixed JSON hint** — when no valid entries are found and lines appear to contain prefixed JSON, Planck shows a clear hint suggesting `--scan-json` or `propagate=False`
- **`--exclude-path` flag** — exclude URL prefixes from analysis (repeatable: `--exclude-path /health --exclude-path /metrics`). Excluded count shown in report header.
- **`--filter-status` flag** — restrict analysis to a status class (`2xx`, `3xx`, `4xx`, `5xx`) or an exact code (`200`, `404`). Filtered count shown in report header.
- **`--since` days support** — `--since 3d` as a shorthand for `--since 72h`. Planck converts `Nd` → `N*24h` before passing to Docker since Docker does not natively support days.
- **`--until` flag** — exclude log entries after a given time. Accepts a duration (`1h`, `3d`) or an RFC3339 absolute timestamp. Works for both Docker and file-based sources.
- **Time range filtering for file logs** — `--since` and `--until` are now applied at the parser level, enabling timestamp-based filtering when analyzing log files.

### Fixed
- **Docker stderr capture** — `docker logs` sends container output to stderr by default. Planck previously only read stdout, silently dropping all log lines for most containers. Both streams are now read concurrently, preventing the deadlock that occurred on containers with large log volumes.
- **Spring Boot preset `@timestamp`** — corrected the Spring preset's timestamp field from `timestamp` to `@timestamp`, matching the field name used by `logstash-logback-encoder`.

## [0.1.1] - 2026-05-09

### Added
- **Field mapping** — `--preset` flag with built-in presets for FastAPI, Express, Gin, Echo, and Spring Boot
- **Custom field flags** — `--field-timestamp`, `--field-method`, `--field-path`, `--field-status`, `--field-latency` to map any JSON log schema
- **Float latency auto-detection** — latency values expressed as decimal seconds (e.g. FastAPI `duration: 0.120`) are automatically converted to milliseconds (120ms)
- Sample log files: `sample-logs/fastapi.log`, `sample-logs/express.log`
- `models.FieldMap` — public struct for field name configuration
- `models.PresetFieldMap()` — preset lookup with validation
- `models.AvailablePresets()` — list of all built-in presets
- Git pre-commit hook (`.githooks/pre-commit`) — auto-runs `gofmt` on staged Go files
- `make hooks` — install git hooks (run once after cloning)
- `make fmt` — format all Go source files

### Fixed
- `go.mod` Go version directive lowered from `1.26.2` to `1.22` for golangci-lint compatibility

---

## [0.1.0] - 2026-05-07

### Added
- Initial release
- `planck analyze <file>` — analyze JSON log files
- `planck analyze --docker <container>` — analyze Docker container logs
- `--tail`, `--since` flags for Docker log fetching
- `--format json` flag for machine-readable output
- `--top N` flag to control number of top endpoints shown (default: 5)
- `--slow N` flag to control number of slowest endpoints shown (default: 5)
- Metrics: total requests, top endpoints, traffic by hour, error rates, avg/P95 latency
- Terminal formatter with ANSI color-coded output and ASCII progress bars
- Insights: high error rate and slow endpoint detection
- GitHub Actions CI and release workflows with goreleaser

---

[Unreleased]: https://github.com/mihirsn/planck/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/mihirsn/planck/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/mihirsn/planck/releases/tag/v0.1.0
