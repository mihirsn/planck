# Watch Mode Reference

`planck watch` continuously monitors one or more Docker containers' logs on a configurable interval, evaluates your alert thresholds, and sends real-time push notifications via [ntfy](https://ntfy.sh/) when something goes wrong.

> **No background daemon. No database. No agent.** Just a long-running terminal process — run it in a tmux pane or as a systemd service.

---

## Quick Start

### Single container

**1. Create `planck.yml` in your working directory:**

```yaml
watch:
  docker: my-api              # Docker container name or ID
  interval: 60s               # Poll logs every 60 seconds
  alert_cooldown: 10m         # Don't repeat the same alert within 10 minutes
  preset: fastapi             # Log format preset

alerts:
  error_rate:
    threshold: 10.0           # Alert if any endpoint exceeds 10% error rate
  p95_latency:
    threshold: 2000           # Alert if any endpoint's p95 latency exceeds 2s
  rps: 200                    # Alert if requests per second exceeds 200

notify:
  ntfy:
    topic: my-api-alerts      # Your ntfy topic name
```

**2. Subscribe to your ntfy topic** on your phone or desktop at `ntfy.sh/my-api-alerts`.

**3. Start watching:**

```bash
planck watch
```

---

### Multiple containers

Drop the `watch.docker` field and use a `containers:` list instead. Each container inherits the global defaults and can optionally override individual settings.

```yaml
watch:
  interval: 60s
  alert_cooldown: 10m
  preset: fastapi             # Global default for all containers

alerts:
  error_rate:
    threshold: 10.0
    exclude_paths:
      - /health
      - /metrics
  p95_latency:
    threshold: 2000

notify:
  ntfy:
    topic: my-alerts

containers:
  - name: my-api              # Inherits all global defaults

  - name: my-worker
    preset: express           # Override preset for this container only
    alerts:
      error_rate:
        threshold: 5.0        # Stricter threshold — still inherits exclude_paths from global

  - name: my-db
    resources:
      cpu:
        threshold: 95         # DB can spike higher than the global 80% default
```

```bash
planck watch
```

**Output in multi-container mode:**

```
> Planck watch started — 3 containers, interval: 60s
   Containers: my-api, my-worker, my-db
   Alerts: error_rate≥10% | p95≥2000ms

[14:05:01] [my-api]    — quiet window
[14:05:02] [my-worker] ✓ Analysed 842 requests (14.0 req/s)
[14:05:03] [my-db]     ✓ Resources — CPU: 31.2% | MEM: 210MB / 512MB (41.0%)

   🚨 Alert sent: **Container:** my-worker
```

Each container gets its own `[label]` prefix so you always know which one fired an alert.

---

## Config File Reference

Planck searches for `planck.yml` in this order:
1. Path passed via `--config`
2. Current working directory (`./planck.yml`)
3. Home directory (`~/.planck.yml`)
4. Global config (`/etc/planck/planck.yml`)

### Global fields

| Field | Type | Default | Description |
|---|---|---|---|
| `watch.docker` | string | — | **Legacy single-container field.** Use `containers:` instead for new setups. Mutually exclusive with `containers:`. |
| `watch.interval` | duration | `60s` | How often to poll logs (applies to all containers) |
| `watch.alert_cooldown` | duration | `10m` | Minimum time between repeat alerts for the same threshold |
| `watch.preset` | string | — | Default log format preset for all containers. Can be overridden per-container. |
| `alerts.rps` | float | — | Alert if requests per second exceeds this value |
| `alerts.error_rate.threshold` | float | — | Alert if any endpoint's error rate exceeds this % |
| `alerts.error_rate.exclude_paths` | []string | — | Endpoints to ignore for error alerts (global default) |
| `alerts.error_rate.include_paths` | []string | — | Restrict error alerts to only these endpoints (global default) |
| `alerts.p95_latency.threshold` | float | — | Alert if any endpoint's p95 latency exceeds this ms |
| `alerts.p95_latency.exclude_paths` | []string | — | Endpoints to ignore for latency alerts (global default) |
| `alerts.p95_latency.include_paths` | []string | — | Restrict latency alerts to only these endpoints (global default) |
| `resources.interval` | duration | `watch.interval` | How often to poll container stats. Global-only — ignored per-container. |
| `resources.cpu.threshold` | float | — | Alert if CPU usage >= this percent (0–100) |
| `resources.memory.percent` | float | — | Alert if memory usage >= this % of the container's limit |
| `resources.memory.absolute` | float | — | Alert if memory usage >= this value in MB |
| `notify.ntfy.topic` | string | **required** | Your ntfy topic name (letters, digits, `-`, `_` only) |
| `notify.ntfy.server` | string | `https://ntfy.sh` | ntfy server URL |
| `notify.ntfy.token` | string | — | Bearer token for protected topics |
| `notify.webhook.url` | string | **required** | Webhook destination URL |
| `notify.webhook.headers` | map | — | Optional HTTP headers (supports `${ENV_VAR}` expansion) |

> All `alerts` and `resources` fields are optional. Planck only checks thresholds you configure — omitted fields are never alerted on.

### Per-container fields (`containers:`)

| Field | Type | Default | Description |
|---|---|---|---|
| `containers[].name` | string | **required** | Docker container name or ID |
| `containers[].preset` | string | `watch.preset` | Override the log format preset for this container |
| `containers[].alerts.error_rate.threshold` | float | global value | Override the error rate threshold for this container |
| `containers[].alerts.error_rate.exclude_paths` | []string | global value | Override exclude paths for this container. Replaces (not appends) the global list. |
| `containers[].alerts.error_rate.include_paths` | []string | global value | Override include paths for this container |
| `containers[].alerts.p95_latency.threshold` | float | global value | Override the p95 latency threshold for this container |
| `containers[].alerts.p95_latency.exclude_paths` | []string | global value | Override latency exclude paths for this container |
| `containers[].alerts.p95_latency.include_paths` | []string | global value | Override latency include paths for this container |
| `containers[].alerts.rps` | float | global value | Override the RPS threshold for this container |
| `containers[].resources.cpu.threshold` | float | global value | Override CPU threshold for this container |
| `containers[].resources.memory.percent` | float | global value | Override memory percent threshold for this container |
| `containers[].resources.memory.absolute` | float | global value | Override memory absolute threshold for this container |

> `resources.interval` is a **global-only** field — it cannot be overridden per-container since all containers share a single resource poll cadence.

### Merge rules

When a container specifies overrides, Planck applies **field-level merging** — you only need to specify what's different:

```yaml
alerts:
  error_rate:
    threshold: 10.0
    exclude_paths:
      - /health       # ← defined globally

containers:
  - name: my-worker
    alerts:
      error_rate:
        threshold: 5.0   # ← overrides threshold only
                          # exclude_paths: [/health] is still inherited automatically
```

If you also set `exclude_paths` on the container, it **replaces** (not appends to) the global list.

---

## Multi-Container Examples

### Minimal: all containers share global config

All containers use the same preset, thresholds, and notify destination. This is the simplest multi-container setup.

```yaml
watch:
  interval: 60s
  preset: fastapi

alerts:
  error_rate:
    threshold: 10.0
  p95_latency:
    threshold: 2000

notify:
  ntfy:
    topic: my-alerts

containers:
  - name: my-api
  - name: my-worker
  - name: my-scheduler
```

---

### Per-container threshold overrides

Different services have different traffic patterns. Workers typically have lower error tolerance than APIs.

```yaml
watch:
  interval: 60s
  preset: fastapi

alerts:
  error_rate:
    threshold: 10.0         # Default for all containers
    exclude_paths:
      - /health
      - /readyz
  p95_latency:
    threshold: 2000

notify:
  ntfy:
    topic: my-alerts

containers:
  - name: my-api            # Inherits error_rate=10%, exclude_paths global

  - name: my-worker
    alerts:
      error_rate:
        threshold: 1.0      # Workers must be very reliable — alert at 1%
                            # Still inherits exclude_paths: [/health, /readyz]

  - name: my-realtime-ws
    alerts:
      p95_latency:
        threshold: 500      # Realtime service — much stricter latency SLA
```

---

### Mixed log formats (different presets per container)

Use when your containers use different frameworks or logging libraries.

```yaml
watch:
  interval: 60s

alerts:
  error_rate:
    threshold: 10.0

notify:
  ntfy:
    topic: my-alerts

containers:
  - name: my-fastapi-service
    preset: fastapi

  - name: my-express-service
    preset: express

  - name: my-gin-service
    preset: gin

  - name: my-custom-service   # Uses planck's default field map (no preset)
```

---

### With per-container resource overrides

Good for mixed workloads — e.g., a database container is allowed to use more CPU than an API.

```yaml
watch:
  interval: 60s
  preset: fastapi

alerts:
  error_rate:
    threshold: 10.0

resources:
  interval: 30s
  cpu:
    threshold: 80           # Default: alert at 80% CPU
  memory:
    percent: 85             # Default: alert at 85% memory

notify:
  ntfy:
    topic: my-alerts

containers:
  - name: my-api            # Inherits: cpu≥80%, mem≥85%

  - name: my-db
    resources:
      cpu:
        threshold: 95       # DB is expected to spike — only alert at 95%
                            # memory.percent still inherited from global (85%)

  - name: my-cache
    resources:
      memory:
        percent: 70         # Cache is memory-sensitive — alert earlier
        absolute: 800       # Also alert if over 800 MB absolute
                            # cpu.threshold still inherited (80%)
```

---

### Full example: two environments in one config

You can also use separate `planck.yml` files per environment and select them with `--config`.

```bash
# Production
planck watch --config /etc/planck/planck-prod.yml

# Staging (in a separate tmux pane)
planck watch --config /etc/planck/planck-staging.yml
```

---

## Endpoint Filtering

You can filter which endpoints trigger alerts on a per-alert basis. Filters apply globally or can be overridden per-container.

```yaml
alerts:
  error_rate:
    threshold: 10.0
    exclude_paths:
      - /health         # Never alert on health checks
      - /metrics        # Or Prometheus scrapes
    include_paths:
      - /api/v1         # Only consider paths under /api/v1 (optional)

  p95_latency:
    threshold: 2000
    exclude_paths:
      - /api/upload     # Long uploads are expected to be slow
```

**Behavior Rules:**
- Excludes act as a hard override. An endpoint **MUST NOT** match any `exclude_paths` to trigger an alert.
- If `include_paths` is provided, the endpoint **MUST** match at least one of them.
- Filtering only suppresses alerts; the global RPS calculation and terminal output remain accurate for all traffic.

---

## Resource Alerts

Planck can monitor your containers' CPU and memory usage in real time, independently of log polling. Stats are collected via `docker stats --no-stream` — no agent, no SDK, zero extra dependencies.

> Resource polling runs in a **separate goroutine per container** with its own interval. You can poll resources more frequently than logs (e.g. every 30s) without changing your log analysis cadence.

### Threshold Behavior

- **CPU threshold** — fires when `CPUPercent >= threshold`
- **Memory percent** — fires when container memory usage >= this % of its configured limit
- **Memory absolute** — fires when memory usage >= this value in MB
- If **both** `memory.percent` and `memory.absolute` are set, each is evaluated independently with its own cooldown key — either condition can trigger without affecting the other
- All resource alerts respect the same `watch.alert_cooldown` as app-level alerts

### Alert Notification Format

```
Title:  Planck – High CPU Usage
Body:   **Container:** my-api
        **CPU:** 87.3% (Threshold: 80%)

Title:  Planck – High Memory Usage
Body:   **Container:** my-api
        **Memory:** 1600MB / 2048MB (78.1%) (Threshold: 75%)
```

The terminal also prints a heartbeat line on each resource poll:

```
[14:05:30] [my-api] ✓ Resources — CPU: 12.3% | MEM: 512MB / 2048MB (25.0%)
```

---

## Memory & CPU Footprint

Planck is designed to stay true to its lightweight philosophy — including in multi-container mode.

### Single container

- **Planck process RAM**: ~8–10 MB RSS
- **Goroutines**: 1 log poller + 1 resource poller (optional), both sleeping ~99% of the time at 60s intervals
- **Subprocess overhead**: each poll spawns a child process (`docker logs` or `docker stats --no-stream`) that exits in <1s. Memory (~1–2 MB) belongs to the child process and is reclaimed immediately by the OS.

### Multiple containers

- **Per-container goroutine stack**: ~2–3 KB per sleeping goroutine
- **3 containers with resource polling**: ~6 goroutines × ~3 KB = ~18 KB additional stack RAM
- **Per-watcher heap**: cooldowns map + config pointers (shared) + field map ≈ ~1 KB per container
- **In practice**: 3 containers adds roughly 1–3 MB to the RSS, landing at **~10–13 MB** total

The dominant cost remains the transient subprocess (`docker logs`, `docker stats`) — not Planck's process memory. Goroutine stacks grow only when actually executing; while sleeping they consume minimal memory.

> **Recommendation for low-resource machines (t2.micro / 1 GB RAM):**
>
> - Use `interval: 60s` or longer — goroutines sleep 99% of the time and only briefly allocate memory during parse
> - Use `resources.interval: 30s` or longer to avoid frequent `docker stats` subprocesses
> - 2–5 containers on a t2.micro is well within budget. The parse-time memory spike is transient and the Go GC reclaims it between polls.

---

## Self-Hosted ntfy

[ntfy](https://ntfy.sh/) can be self-hosted on your own server. Simply point `server` at your instance:

```yaml
notify:
  ntfy:
    topic: my-alerts
    server: https://ntfy.yourdomain.com
    token: your-access-token
```

## Webhook Notifications

Planck can also send a structured JSON `POST` request to any webhook endpoint when an alert fires.

```yaml
notify:
  webhook:
    url: https://api.pagerduty.com/alerts
    headers:
      Authorization: Bearer ${PAGERDUTY_TOKEN}
```

*Planck automatically expands environment variables in headers, preventing you from committing secrets to your repository.*

---

## Running in Production

Because `planck watch` is a standalone binary and not a background daemon, it will stop if your SSH session disconnects. To keep Planck running continuously in the background, choose one of these options:

### Option 1: The Quick Way (`tmux`)

Use `tmux` to create a virtual terminal session that survives SSH disconnects. Great if you want to view live output.

```bash
# Start a new tmux session
tmux

# Run your watcher (reads containers from planck.yml)
planck watch

# To safely detach and leave it running in the background:
# Press Ctrl+b, then press d

# When you SSH back in later, reattach with:
tmux attach
```

### Option 2: The Production Way (`systemd`)

For a true "set and forget" deployment that automatically restarts after reboots.

#### If you installed via `.deb` or `.rpm`

The systemd service file is already installed. Place your config at `/etc/planck/planck.yml` with a `containers:` list (or the legacy `watch.docker` field), then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable planck
sudo systemctl start planck
```

#### If you installed via `curl` / `tar.gz`

Create the service file manually:

```bash
sudo nano /etc/systemd/system/planck.service
```

```ini
[Unit]
Description=Planck Watcher
After=docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/planck watch --config /etc/planck/planck.yml
Restart=on-failure
RestartSec=5s
User=root
StandardOutput=journal
StandardError=journal
SyslogIdentifier=planck

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable planck
sudo systemctl start planck
```

View live logs with: `sudo journalctl -u planck -f`
