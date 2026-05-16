<h1 align="center">⚛️ Planck</h1>

<p align="center">
  <b>Observe behavior at the smallest scale.</b><br/>
  Lightweight log analysis for developers who live in the terminal.
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

> **⚠️ Early Alpha — v0.1.x**
>
> Planck is in active early development. The log format, CLI flags, and JSON output schema **may change between versions** without a deprecation period.
>
> **Platform testing**: Manually tested and verified on **macOS arm64** only. Unit tests pass on Linux (ubuntu-latest) via CI. Binaries for Linux and Windows are built via goreleaser but have not yet been manually verified. If you try Planck on Linux or Windows and run into issues, please [open a bug report](https://github.com/mihirsn/planck/issues/new?template=bug_report.md) — your feedback helps.
>
> Feedback, bug reports, and ideas are very welcome — this is the best time to shape the direction of the project.

---

Planck transforms raw application logs into actionable operational insights — **without requiring Prometheus, Grafana, databases, or agents**.

Built for developers running applications on Docker, VPS, and small cloud environments who rely primarily on logs.

---

## Why Planck?

Most observability tools assume you already have a metrics pipeline. Small teams, self-hosted apps, and early-stage products often don't — they have **logs**.

Planck bridges that gap: give it a log file or a Docker container name, and it tells you what's actually happening.

```
⚛️  Planck Analysis
──────────────────────────────────────────────────
Source:          Docker container "invoice-api"
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

──────────────────────────────────────────────────
Analysis complete.
```

---

## Features

| Feature | V1.1 |
|---|---|
| Analyze JSON log files | ✅ |
| Analyze Docker container logs | ✅ |
| Total requests | ✅ |
| Top endpoints by traffic | ✅ |
| Traffic by hour | ✅ |
| Error rates per endpoint | ✅ |
| Avg + P95 latency per endpoint | ✅ |
| Slowest endpoints | ✅ |
| Graceful handling of malformed lines | ✅ |
| JSON output (`--format json`) | ✅ |
| Single binary, no dependencies | ✅ |

---

## Known Limitations

### Memory usage scales with log file size

Planck currently loads all parsed log entries into memory before computing metrics. This works well for typical log volumes but can be a concern for very large log files on memory-constrained servers (e.g. a t2.micro with 1 GB RAM).

**Rule of thumb**: Planck comfortably handles log files up to ~100k entries on any modern machine. Beyond that, use the workarounds below.

**Workarounds (available now):**

```bash
# Docker: only fetch the last 5000 lines
planck analyze --docker my-api --tail 5000

# Docker: only fetch logs from the last 2 hours
planck analyze --docker my-api --since 2h

# File: use tail before analyzing
tail -n 10000 /var/log/app.log | planck analyze /dev/stdin
```

**Planned fix (V1.3):** A streaming accumulator engine that processes one log entry at a time without building an in-memory slice. See [Roadmap](#roadmap) for details.

---

## Installation

### Download a release binary (recommended)

Download the latest binary for your platform from [GitHub Releases](https://github.com/mihirsn/planck/releases/latest).

```bash
# macOS / Linux
curl -sSL https://github.com/mihirsn/planck/releases/latest/download/planck_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m).tar.gz | tar -xz
sudo mv planck /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/mihirsn/planck.git
cd planck
make build
sudo mv planck /usr/local/bin/
```

---

## Usage

### Analyze a log file

```bash
planck analyze app.log
```

### Analyze Docker container logs

```bash
planck analyze --docker invoice-api
```

### Fetch only the last N lines from Docker

```bash
planck analyze --docker invoice-api --tail 1000
```

### Fetch Docker logs from the last hour

```bash
planck analyze --docker invoice-api --since 1h
```

### Machine-readable JSON output

```bash
planck analyze app.log --format json
planck analyze app.log --format json | jq '.top_endpoints'
```

### Control how many endpoints are shown

```bash
planck analyze app.log --top 10     # show top 10 endpoints (default: 5)
planck analyze app.log --slow 3     # show 3 slowest endpoints (default: 5)
```

---

## Log Format

Planck reads **one JSON object per line** ([JSON Lines](https://jsonlines.org/) format).

> **Note on log format**: Planck currently requires a specific JSON schema (described below). Use `--preset` or `--field-*` flags to adapt Planck to your existing log format — see [Field mapping flags](#field-mapping-flags) for details.

```json
{"timestamp":"2026-05-08T14:05:00Z","method":"POST","path":"/invoice","status":200,"latency_ms":120}
{"timestamp":"2026-05-08T14:05:02Z","method":"GET","path":"/login","status":401,"latency_ms":50}
{"timestamp":"2026-05-08T14:05:04Z","method":"GET","path":"/checkout","status":500,"latency_ms":980}
```

### Required fields

| Field | Type | Description |
|---|---|---|
| `timestamp` | RFC3339 string | When the request was handled |
| `path` | string | Request path (e.g. `/invoice`) |
| `status` | integer | HTTP status code |

### Optional fields

| Field | Type | Description |
|---|---|---|
| `method` | string | HTTP method (GET, POST, etc.) |
| `latency_ms` | integer | Request duration in milliseconds |

> **On malformed lines**: Lines that are not valid JSON or are missing required fields are **skipped gracefully**. Planck reports how many were skipped and continues processing the rest.

---

## Getting the Most Out of Planck

### Add latency to your logs

Latency data (`latency_ms`) is optional — Planck works without it. But without it, the **Slow Endpoints** section will be empty and P95/avg latency won't be shown.

Adding latency is a one-line change in most frameworks and gives you significantly more insight. Here's how to do it:

#### Go — `chi` router

```go
r.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
        next.ServeHTTP(ww, r)
        log.Printf(`{"timestamp":%q,"method":%q,"path":%q,"status":%d,"latency_ms":%d}`,
            time.Now().UTC().Format(time.RFC3339),
            r.Method, r.URL.Path,
            ww.Status(),
            time.Since(start).Milliseconds(),
        )
    })
})
```

#### Python — FastAPI / Starlette

```python
import time, logging
from starlette.middleware.base import BaseHTTPMiddleware

class PlanckLoggingMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request, call_next):
        start = time.time()
        response = await call_next(request)
        latency_ms = int((time.time() - start) * 1000)
        logging.info('{"timestamp":"%s","method":"%s","path":"%s","status":%d,"latency_ms":%d}',
            __import__('datetime').datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ'),
            request.method, request.url.path,
            response.status_code, latency_ms)
        return response

app.add_middleware(PlanckLoggingMiddleware)
```

#### Node.js — Express

```js
app.use((req, res, next) => {
  const start = Date.now();
  res.on('finish', () => {
    console.log(JSON.stringify({
      timestamp: new Date().toISOString(),
      method: req.method,
      path: req.path,
      status: res.statusCode,
      latency_ms: Date.now() - start,
    }));
  });
  next();
});
```

### What if I already have logs without latency?

Planck still works. It will show you request counts, top endpoints, traffic by hour, and error rates — everything except the latency-based sections. Those sections simply won't appear in the output rather than showing misleading zeros.

---

## Flags Reference

### Source flags

| Flag | Default | Description |
|---|---|---|
| `--docker` | — | Docker container name or ID |
| `--tail` | 0 (all) | Number of log lines to fetch from Docker |
| `--since` | — | Fetch logs since duration (e.g. `1h`, `30m`) |

### Output flags

| Flag | Default | Description |
|---|---|---|
| `--format` | `text` | Output format: `text` or `json` |
| `--top` | 5 | Number of top endpoints to display |
| `--slow` | 5 | Number of slowest endpoints to display |

### Field mapping flags

Use these flags to tell Planck which JSON keys to read from your logs.
Individual `--field-*` flags always override the `--preset` value.

| Flag | Default | Description |
|---|---|---|
| `--preset` | — | Load a built-in field mapping preset (see table below) |
| `--field-timestamp` | `timestamp` | JSON key for the timestamp |
| `--field-method` | `method` | JSON key for the HTTP method |
| `--field-path` | `path` | JSON key for the request path |
| `--field-status` | `status` | JSON key for the HTTP status code |
| `--field-latency` | `latency_ms` | JSON key for the request latency |

### Parsing behaviour flags

| Flag | Default | Description |
|---|---|---|
| `--scan-json` | `false` | Scan each line for the first `{` before parsing. Useful when logs have a text prefix (e.g. Python's `INFO:logger:{...}` format). |

### Built-in presets

| Preset | Framework | `--field-path` | `--field-status` | `--field-latency` | Notes |
|---|---|---|---|---|---|
| `fastapi` | FastAPI / uvicorn | `path` | `status_code` | `duration` | latency auto-converted from float seconds |
| `express` | Express.js + morgan | `url` | `statusCode` | `responseTime` | |
| `gin` | Go Gin | `path` | `status` | `latency` | |
| `echo` | Go Echo | `uri` | `status` | `latency` | |
| `spring` | Spring Boot | `uri` | `status` | `duration` | uses `@timestamp` (logstash-logback-encoder) |

---

## Error Handling

| Scenario | Planck output |
|---|---|
| Docker CLI not installed | `Docker CLI not found. Please install Docker or use file input mode.` |
| Container not found | `Container "xyz" not found.` |
| File not found | `log file not found: "app.log"` |
| Malformed log lines | `⚠  Skipped 14 malformed log entries.` |

---

## Development

```bash
# Clone and build
git clone https://github.com/mihirsn/planck.git
cd planck
make build

# Run tests
make test

# Check coverage (must be ≥ 90% on internal packages)
make coverage-check

# Run with sample logs
make run ARGS="analyze sample-logs/app.log"
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contributor guide.

---

## Roadmap

### V1.2 (in progress)
- [x] **Field mapping** — `--preset` flag with built-in presets for FastAPI, Express, Gin, Echo, Spring Boot
- [x] **Custom field flags** — `--field-*` flags to map any JSON schema to Planck's model
- [x] **`--scan-json` flag** — strip text prefixes (e.g. Python's `INFO:logger:{...}`) before parsing
- [x] **`--exclude-path` flag** — exclude URL patterns from analysis (e.g. `--exclude-path /health --exclude-path /metrics`). Supports prefix matching and exact paths.
- [x] **`--since` days support** — `--since 3d` as a shorthand for `--since 72h`. Planck converts `Nd` → `N*24h` before passing to Docker.
- [x] **`--filter-status` flag** — analyze only specific status codes (`2xx`, `4xx`, `5xx`, or exact like `200`, `404`)
- [x] **`--until` flag** — exclude entries after a given time. Accepts a duration (`1h`, `3d`) or RFC3339 timestamp. Works for both Docker and file sources.
- [x] **`--since` / `--until` for file-based logs** — timestamp range filtering applied at the parser level for log files

### V1.3 (planned)
- [ ] **Streaming accumulator engine** — eliminate the current memory bottleneck.
  Currently Planck buffers all log entries into memory before computing metrics.
  V1.3 will replace this with a per-endpoint accumulator that processes one entry at a time:
  - All counters (request count, error count, traffic by hour) become O(1) per entry.
  - Latency storage is **capped per endpoint** (default: 10,000 samples) — enough for statistically accurate P95 computation on any real-world service, while bounding memory to a fixed ~4 MB regardless of log file size.
  - The result: Planck will handle arbitrarily large log files on a 1 GB t2.micro without breaking a sweat.
- [ ] `--latency-samples N` — configure the per-endpoint latency cap (default: 10,000)

### V2.0 (future)
- [ ] Live log streaming (`planck tail`)
- [ ] Multiple container aggregation
- [ ] Docker Compose support

Have an idea? [Open a feature request](https://github.com/mihirsn/planck/issues/new?template=feature_request.md).

---

## Philosophy

> Meaningful insights from logs with near-zero operational overhead.

Planck is intentionally **not**:
- Another Prometheus
- Another Grafana  
- Another observability platform

It's a sharp, focused tool for developers who want answers quickly.

---

## License

[MIT](LICENSE)
