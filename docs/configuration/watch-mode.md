# Watch Mode Reference

`planck watch` continuously monitors a Docker container's logs on a configurable interval, evaluates your alert thresholds, and sends real-time push notifications via [ntfy](https://ntfy.sh/) when something goes wrong.

> **No background daemon. No database. No agent.** Just a long-running terminal process — run it in a tmux pane or as a Docker command.

## Quick Start

**1. Create a `planck.yml` in your working directory:**

```yaml
watch:
  docker: my-api              # Docker container name or ID to watch
  interval: 60s               # Poll logs every 60 seconds
  alert_cooldown: 10m         # Don't repeat the same alert within 10 minutes
  preset: fastapi             # Your log format preset

alerts:
  rps: 200                    # Alert if requests per second exceeds 200
  error_rate:
    threshold: 10.0           # Alert if any endpoint exceeds 10% error rate
  p95_latency:
    threshold: 2000           # Alert if any endpoint's p95 latency exceeds 2s

resources:
  interval: 30s               # Poll container stats every 30 seconds (independent of log interval)
  cpu:
    threshold: 80             # Alert if CPU usage >= 80%
  memory:
    percent: 75               # Alert if memory usage >= 75% of container limit
    absolute: 1500            # Alert if memory usage >= 1500 MB (either condition triggers)

notify:
  ntfy_topic: my-api-alerts        # Your ntfy topic name
  ntfy_server: https://ntfy.sh     # Or your self-hosted ntfy URL
  ntfy_token: ""                   # Optional: for protected topics
```

**2. Subscribe to your ntfy topic** on your phone or desktop at `ntfy.sh/my-api-alerts`.

**3. Start watching:**

```bash
# Container name is read from planck.yml
planck watch

# Or override the container name at runtime
planck watch --docker my-api
```

## Config File Reference

Planck searches for `planck.yml` in this order:
1. Path passed via `--config`
2. Current working directory (`./planck.yml`)
3. Home directory (`~/.planck.yml`)

| Field | Type | Default | Description |
|---|---|---|---|
| `watch.docker` | string | — | Docker container name or ID to watch. Can be overridden via `--docker` flag |
| `watch.interval` | duration | `60s` | How often to poll logs |
| `watch.alert_cooldown` | duration | `10m` | Minimum time between repeat alerts for the same threshold |
| `watch.preset` | string | — | Log format preset (same as `--preset` on `analyze`) |
| `alerts.rps` | float | — | Alert if requests per second exceeds this value |
| `alerts.error_rate.threshold` | float | — | Alert if any endpoint's error rate exceeds this % |
| `alerts.error_rate.exclude_paths` | []string | — | Endpoints to ignore for error alerts |
| `alerts.error_rate.include_paths` | []string | — | Restrict error alerts to only these endpoints |
| `alerts.p95_latency.threshold` | float | — | Alert if any endpoint's p95 latency exceeds this ms |
| `alerts.p95_latency.exclude_paths` | []string | — | Endpoints to ignore for latency alerts |
| `alerts.p95_latency.include_paths` | []string | — | Restrict latency alerts to only these endpoints |
| `resources.interval` | duration | `watch.interval` | How often to poll container stats (independent of log polling) |
| `resources.cpu.threshold` | float | — | Alert if container CPU usage >= this percent (0–100) |
| `resources.memory.percent` | float | — | Alert if memory usage >= this % of the container's memory limit |
| `resources.memory.absolute` | float | — | Alert if memory usage >= this value in MB |
| `notify.ntfy_topic` | string | **required** | Your ntfy topic name (letters, digits, `-`, `_` only) |
| `notify.ntfy_server` | string | `https://ntfy.sh` | ntfy server URL (must be http/https) |
| `notify.ntfy_token` | string | — | Bearer token for protected topics |

> All `alerts` and `resources` fields are optional. Planck only checks thresholds you configure — omitted fields are never alerted on.

## Endpoint Filtering

You can optionally filter which API endpoints trigger alerts on a per-alert basis. This allows you to ignore noisy internal endpoints or allow slow polling endpoints without sacrificing error visibility.

```yaml
alerts:
  error_rate:
    threshold: 10.0
    # Ignore alerts for these paths (e.g. /health)
    exclude_paths:
      - "/health"
    # Only trigger alerts for these paths. 
    # If omitted, all paths not excluded are allowed.
    include_paths:
      - "/api/v1"

  p95_latency:
    threshold: 2000
    # Ignore slow long-polling endpoints from latency alerts
    exclude_paths:
      - "/api/upload"
```

**Behavior Rules:**
- Excludes act as a hard override. An endpoint **MUST NOT** match any `exclude_paths` to trigger an alert.
- If `include_paths` is provided, the endpoint **MUST** match at least one of them.
- Filtering only suppresses alerts; the global Requests Per Second (RPS) calculation and terminal output remain accurate for all traffic.

---

## Resource Alerts

Planck can monitor your container's CPU and memory usage in real time, independently of log polling. Resource stats are collected via `docker stats --no-stream` — no agent, no SDK, zero extra dependencies.

> Resource polling runs in a **separate goroutine** with its own interval. You can poll resources more frequently than logs (e.g. every 30s) without changing your log analysis cadence.

### Threshold Behavior

- **CPU threshold** — fires when `CPUPercent >= threshold`
- **Memory percent** — fires when container memory usage >= this % of its configured limit
- **Memory absolute** — fires when memory usage >= this value in MB
- If **both** `memory.percent` and `memory.absolute` are set, each is evaluated independently with its own cooldown key — either condition can trigger an alert without affecting the other
- All resource alerts respect the same `watch.alert_cooldown` as app-level alerts

### Examples

**CPU only:**
```yaml
resources:
  interval: 30s
  cpu:
    threshold: 85   # Alert when container CPU >= 85%
```

**Memory only (both conditions):**
```yaml
resources:
  memory:
    percent: 80     # Alert when memory >= 80% of container limit
    absolute: 3000  # OR when memory >= 3000 MB — whichever triggers first
```

**CPU + memory combined:**
```yaml
resources:
  interval: 20s     # Poll stats every 20 seconds
  cpu:
    threshold: 75
  memory:
    percent: 70
    absolute: 1500
```

**Omit `resources:` entirely** to disable all resource monitoring — zero behavior change from previous versions.

### Alert Notification Format

When a resource threshold is breached, Planck sends a notification via ntfy:

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
[01:30:00] ✓ Resources — CPU: 12.3% | MEM: 512MB / 2048MB (25.0%)
```

### Memory & CPU Footprint

Resource monitoring is designed to stay true to Planck's lightweight philosophy. Here's what it actually costs:

- **Planck process RAM**: unchanged — the resource polling goroutine adds ~4 KB of stack, which is negligible. In practice, `planck watch` continues to run at ~7–10 MB RSS.
- **Transient subprocess**: each poll spawns `docker stats --no-stream` as a **separate child process** (~1–2 MB), which exits in under a second. This memory belongs to the child process, not Planck, and the OS reclaims it immediately.
- **CPU**: the resource goroutine sleeps between polls. It only wakes, runs a single CLI command, parses ~100 bytes of JSON, compares three floats, then sleeps again.

> **Recommendation for low-resource machines (e.g. t2.micro / 1 GB RAM):** Use `interval: 30s` or longer. Polling every 5–10 seconds spawns a new `docker stats` subprocess that frequently — it works fine, but 30s is more than sufficient to catch sustained resource spikes and keeps overhead minimal.


```bash
# Useful for managing multiple environments
planck watch --docker my-api --config /etc/planck/planck-prod.yml
```

## Self-Hosted ntfy

[ntfy](https://ntfy.sh/) can be self-hosted on your own server. Simply point `ntfy_server` at your instance:

```yaml
notify:
  ntfy_topic: my-alerts
  ntfy_server: https://ntfy.yourdomain.com
  ntfy_token: your-access-token
```

## Running in Production

Because `planck watch` is a standalone binary and not a background daemon, it will stop if your SSH session disconnects (due to Linux sending a `SIGHUP` signal). 

To keep Planck running continuously in the background, choose one of these two options:

### Option 1: The Quick Way (`tmux`)
Use `tmux` to create a virtual terminal session that survives SSH disconnects. This is great if you still want to easily view the live terminal output.

```bash
# Start a new tmux session
tmux

# Run your watcher
planck watch --docker my-api

# To safely detach and leave it running in the background:
# Press Ctrl+b, then press d

# When you SSH back in later, reattach with:
tmux attach

# You will immediately see the live terminal output and all the
# analysis history that Planck printed while you were away!
```

### Option 2: The Production Way (`systemd`)
For a true "set and forget" deployment that automatically restarts if your server reboots, configure Planck as a `systemd` service.

#### If you installed via `.deb` or `.rpm`

The systemd service file is already installed. The `ExecStart` is simply `planck watch` — no container name is hardcoded. 

To start the daemon, you must place your config file in the global directory:
1. Create your config file: `sudo mkdir -p /etc/planck && sudo nano /etc/planck/planck.yml`
2. Make sure the `docker` field is set with your container name under `watch`

```bash
# Enable and start the background daemon
sudo systemctl daemon-reload
sudo systemctl enable planck
sudo systemctl start planck
```

#### If you installed via `curl` / `tar.gz`

You need to create the service file manually first:

1. Create the file: `sudo nano /etc/systemd/system/planck.service`
2. Paste the following:

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

3. Enable and start the daemon:

```bash
sudo systemctl daemon-reload
sudo systemctl enable planck
sudo systemctl start planck
```

You can view the background logs at any time using `sudo journalctl -u planck -f`.


