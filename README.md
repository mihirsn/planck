# ⚛️ Planck

**Meaningful insights from logs with near-zero operational overhead.**

[![Go Build](https://github.com/mihirsn/planck/actions/workflows/go.yml/badge.svg)](https://github.com/mihirsn/planck/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mihirsn/planck)](https://goreportcard.com/report/github.com/mihirsn/planck)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

> 📖 **[Read the full documentation in the `/docs` folder](docs/README.md)**

## The Hook

Planck is a sharp, focused terminal tool for developers who want answers quickly. It runs completely locally without external dependencies. 

Planck is intentionally **not**:
- Another Prometheus
- Another Grafana  
- Another massive observability platform

It's a single static binary that instantly parses JSON logs and gives you actionable metrics.

## Quick Install

If you have Go installed:

```bash
go install github.com/mihirsn/planck@latest
```
*(See the [Installation Guide](docs/getting-started/installation.md) for more options).*

## Quick Start

Planck has two primary modes of operation.

### 1. Analyze Mode (Ad-hoc insights)
Parse a log file or a Docker container's current log stream and generate a summary report.

```bash
planck analyze --docker my-api
```

### 2. Watch Mode (Continuous monitoring)
Continuously monitor a Docker container's logs and send real-time push notifications via [ntfy.sh](https://ntfy.sh) when things go wrong.

```bash
planck watch --docker my-api
```

*(See the [Watch Mode Docs](docs/configuration/watch-mode.md) for configuration details).*

---

## Documentation Directory

Dive into the full documentation:

* **Getting Started:** [Installation](docs/getting-started/installation.md) | [Quickstart](docs/getting-started/quickstart.md)
* **Configuration:** [Analyze Flags](docs/configuration/analyze-flags.md) | [Watch Mode](docs/configuration/watch-mode.md) | [Custom Fields](docs/configuration/custom-fields.md)
* **Guides:** [Parsing Docker Logs](docs/guides/parsing-docker.md) | [Adding Latency](docs/guides/adding-latency.md)
* **Reference:** [Architecture & Roadmap](docs/architecture.md)

---

## License

[MIT](LICENSE)
