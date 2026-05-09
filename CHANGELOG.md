# Changelog

All notable changes to Planck will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- Initial project scaffolding
- `planck analyze <file>` — analyze JSON log files
- `planck analyze --docker <container>` — analyze Docker container logs
- `--tail`, `--since` flags for Docker log fetching
- `--format json` flag for machine-readable output
- `--top N` flag to control number of top endpoints shown (default: 5)
- `--slow N` flag to control number of slowest endpoints shown (default: 5)
- Metrics: total requests, top endpoints, traffic by hour, error rates, avg/P95 latency
- Terminal formatter with color-coded output
- Insights: slow endpoint detection, high error rate detection

---

[Unreleased]: https://github.com/mihirsn/planck/compare/HEAD...HEAD
