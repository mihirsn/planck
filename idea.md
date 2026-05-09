# Planck — V1.1 Implementation Specification

## Vision

> **Observe behavior at the smallest scale.**

Planck is a lightweight CLI tool that transforms raw application logs into actionable operational insights — without requiring heavyweight observability stacks.

Primary target users:

* Docker users
* small teams
* self-hosted applications
* cost-sensitive deployments
* developers using EC2/VPS/Docker Compose
* teams relying mostly on logs

---

# 1. Product Philosophy

## Core Principles

### ✅ Lightweight

* single binary
* no database
* no agents
* no external services
* minimal dependencies

---

### ✅ Log-first

Logs are the source of truth.

---

### ✅ Insight-oriented

Planck explains behavior instead of only displaying raw logs.

---

### ✅ CLI-first

No web UI in V1/V1.1.

---

# 2. V1.1 Scope (STRICT)

## Supported Inputs

### 1. JSON log file

Example:

```bash
planck analyze app.log
```

---

### 2. Docker container logs

Example:

```bash
planck analyze --docker invoice-api
```

---

# 3. Features Included in V1.1

Planck MUST support:

## Metrics

* total requests
* top endpoints
* requests grouped by hour
* error rate per endpoint
* average latency
* P95 latency

---

## Docker Integration

* analyze logs from Docker containers
* use `docker logs <container>`
* no Docker SDK initially

---

# 4. Features NOT Allowed in V1.1

DO NOT IMPLEMENT:

* web UI
* TUI
* dashboards
* Prometheus integration
* Kubernetes integration
* OpenTelemetry
* live log streaming
* distributed tracing
* multi-container aggregation
* Docker Compose integration
* databases/storage
* AI/LLM analysis
* authentication
* plugins

---

# 5. Expected Log Format

## Supported format ONLY

One JSON object per line.

Example:

```json
{"timestamp":"2026-05-08T14:05:00Z","method":"POST","path":"/invoice","status":200,"latency_ms":120}
{"timestamp":"2026-05-08T14:05:02Z","method":"GET","path":"/login","status":200,"latency_ms":50}
```

---

# 6. Required Log Fields

## Mandatory

* timestamp
* path
* status

## Optional but recommended

* latency_ms
* method

---

# 7. Recommended Repository Structure

```text
planck/
├── cmd/
│   ├── root.go
│   └── analyze.go
│
├── internal/
│   ├── models/
│   │   └── log_entry.go
│   │
│   ├── source/
│   │   ├── source.go
│   │   ├── file_source.go
│   │   └── docker_source.go
│   │
│   ├── parser/
│   │   └── parser.go
│   │
│   ├── metrics/
│   │   └── metrics.go
│   │
│   ├── formatter/
│   │   └── terminal.go
│   │
│   └── insights/
│       └── insights.go
│
├── sample-logs/
│   └── app.log
│
├── main.go
├── README.md
├── go.mod
└── LICENSE
```

---

# 8. Core Data Model

## File

`internal/models/log_entry.go`

```go
type LogEntry struct {
    Timestamp time.Time `json:"timestamp"`
    Method     string    `json:"method"`
    Path       string    `json:"path"`
    Status     int       `json:"status"`
    LatencyMs  int       `json:"latency_ms"`
}
```

---

# 9. Log Source Architecture

Planck must abstract log input sources.

## Interface

## File

`internal/source/source.go`

```go
type LogSource interface {
    Stream() (<-chan string, error)
}
```

---

# 10. File Source

## File

`internal/source/file_source.go`

Responsibilities:

* read file line-by-line
* stream log lines through channel
* memory efficient

---

# 11. Docker Source

## File

`internal/source/docker_source.go`

Responsibilities:

* execute:

```bash
docker logs <container>
```

* capture stdout
* stream logs line-by-line

## IMPORTANT

DO NOT USE Docker SDK in V1.1.

Use:

```go
exec.Command(...)
```

Reason:

* simpler
* fewer dependencies
* faster development

---

# 12. Parser Requirements

## File

`internal/parser/parser.go`

Responsibilities:

* parse JSON safely
* convert into LogEntry
* skip malformed lines
* count malformed entries
* continue processing

---

# 13. Metrics Engine

## File

`internal/metrics/metrics.go`

Must calculate:

## Global

* total requests

---

## Per endpoint

* request count
* error rate
* avg latency
* P95 latency

---

## Time grouping

* requests grouped by hour

---

# 14. Error Definition

HTTP status:

```text
>= 400
```

counts as error.

---

# 15. P95 Calculation

Implementation logic:

1. sort latencies ascending
2. calculate:

```text
index = ceil(0.95 * N) - 1
```

---

# 16. CLI UX

## File input

```bash
planck analyze app.log
```

---

## Docker input

```bash
planck analyze --docker invoice-api
```

---

# 17. Example Output

```text
⚛️ Planck Analysis

Source: Docker container "invoice-api"

Total requests: 12,430

🔥 Top endpoints
/invoice      42%
/login        18%
/checkout     11%

⏰ Traffic by hour
14:00   3200 requests
15:00   2900 requests

⚠️ Error rates
/checkout   3.2%
/invoice    1.1%

🐢 Slow endpoints
/invoice
  avg: 820ms
  p95: 1400ms
```

---

# 18. CLI Commands Allowed

## ONLY implement

```bash
planck analyze <file>
```

and

```bash
planck analyze --docker <container>
```

---

# 19. Commands NOT Allowed Yet

DO NOT IMPLEMENT:

```bash
planck inspect
planck doctor
planck tail
planck watch
```

---

# 20. Error Handling Requirements

## Missing Docker

If Docker command unavailable:

```text
Docker CLI not found.
Please install Docker or use file input mode.
```

---

## Invalid container

If container missing:

```text
Container "xyz" not found.
```

---

## Invalid logs

If malformed lines exist:

```text
Skipped 14 malformed log entries.
```

---

# 21. Performance Constraints

Must:

* process line-by-line
* avoid loading entire file unnecessarily
* support large logs reasonably

No premature optimization needed beyond streaming reads.

---

# 22. Suggested Dependencies

## Allowed

* cobra
* standard library

---

## Avoid

* heavy terminal UI libraries
* Docker SDK
* observability frameworks

---

# 23. Development Milestones

## Milestone 1

CLI boots successfully.

```bash
planck --help
```

---

## Milestone 2

File parsing works.

```bash
planck analyze sample-logs/app.log
```

---

## Milestone 3

Metrics output works.

---

## Milestone 4

Docker source works.

```bash
planck analyze --docker invoice-api
```

---

## Milestone 5

Pretty terminal formatting.

---

# 24. README Requirements

README must include:

* vision
* why Planck exists
* example logs
* example commands
* Docker support
* screenshots/examples
* roadmap

---

# 25. Suggested README Intro

```md
# Planck

Observe behavior at the smallest scale.

Planck is a lightweight CLI tool that transforms raw application logs into actionable operational insights — without requiring heavyweight observability infrastructure.

Built for developers running applications in Docker, VPS, and small cloud environments.
```

---

# 26. Engineering Philosophy

The implementation should prioritize:

* simplicity
* readability
* maintainability
* fast execution
* minimal setup

Avoid enterprise-level abstraction and overengineering.

---

# 27. Final Goal

Planck should feel like:

```text
Dozzle + lightweight operational intelligence
```

NOT:

* another Prometheus
* another Grafana
* another observability platform

The main value proposition is:

> meaningful insights from logs with near-zero operational overhead.
