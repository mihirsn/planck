<h1 align="center">> planck</h1>

<p align="center">
  <b>Observe behavior at the smallest scale.</b><br/>
  Meaningful insights from logs with near-zero operational overhead.
</p>

<p align="center">
  <a href="https://github.com/mihirsn/planck/actions/workflows/ci.yml">
    <img src="https://github.com/mihirsn/planck/actions/workflows/ci.yml/badge.svg" alt="CI">
  </a>
  <a href="https://github.com/mihirsn/planck/releases/latest">
    <img src="https://img.shields.io/github/v/release/mihirsn/planck" alt="Latest Release">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT">
  </a>
  <a href="https://go.dev/">
    <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go" alt="Go Version">
  </a>
  <img src="https://img.shields.io/badge/status-early%20alpha-orange" alt="Status: Early Alpha">
</p>

---

> Planck is in active early development. The log format, CLI flags, and JSON output schema **may change between versions** without a deprecation period.
>
> **Platform testing**: Manually tested and verified on **macOS arm64** only. Unit tests pass on Linux (ubuntu-latest) via CI. Binaries for Linux and Windows are built via goreleaser but have not yet been manually verified. If you try Planck on Linux or Windows and run into issues, please [open a bug report](https://github.com/mihirsn/planck/issues/new?template=bug_report.md) — your feedback helps.
>
> Feedback, bug reports, and ideas are very welcome — this is the best time to shape the direction of the project.

---

> 📖 **[Read the full documentation in the `/docs` folder](docs/README.md)**

## The Hook

Planck transforms raw application logs into actionable operational insights — **without requiring Prometheus, Grafana, databases, or agents**.

Built for developers running applications on Docker, VPS, and small cloud environments who rely primarily on logs. Most observability tools assume you already have a metrics pipeline. Small teams, self-hosted apps, and early-stage products often don't — they have **logs**.

Planck bridges that gap: give it a log file or a Docker container name, and it tells you what's actually happening.

```text
⚛️  Planck Analysis
──────────────────────────────────────────────────
Source:          Docker container "my-api"
Total requests:  12,430

🔥 Top endpoints
  /invoice                       ████████░░░░░░░░░░░░  42.1%
  /login                         ████░░░░░░░░░░░░░░░░  18.3%
  /checkout                      ██░░░░░░░░░░░░░░░░░░  11.2%

⏰ Traffic by hour (UTC)
  14:00  ████████████████████  3,200
  15:00  ██████████████████░░  2,900

⚠️  Error rates
  /checkout                      50.0%
  /invoice                       28.6%

🐢 Slow endpoints
  /checkout                      avg: 1103ms  p95: 1980ms

💡 Insights
  ⚠ /checkout has a high error rate of 50.0%
  ⚠ /checkout is slow (avg: 1103ms, p95: 1980ms)
```

## Quick Install

```bash
# macOS / Linux
curl -sSL "https://github.com/mihirsn/planck/releases/latest/download/planck_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" | tar -xz
sudo mv planck /usr/local/bin/
```

*(See the [Installation Guide](docs/getting-started/installation.md) for `go install` and source build options).*

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
