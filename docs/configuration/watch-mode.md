# Watch Mode Reference

`planck watch` continuously monitors a Docker container's logs on a configurable interval, evaluates your alert thresholds, and sends real-time push notifications via [ntfy](https://ntfy.sh/) when something goes wrong.

> **No background daemon. No database. No agent.** Just a long-running terminal process — run it in a tmux pane or as a Docker command.

## Quick Start

**1. Create a `planck.yml` in your working directory:**

```yaml
watch:
  docker: my-api              # Docker container name or ID to watch
  interval: 60s          # Poll every 60 seconds
  alert_cooldown: 10m    # Don't repeat the same alert within 10 minutes
  preset: fastapi        # Your log format preset

alerts:
  error_rate_pct: 10.0   # Alert if any endpoint exceeds 10% error rate
  p95_latency_ms: 2000   # Alert if any endpoint's p95 latency exceeds 2s
  rps: 200               # Alert if requests per second exceeds 200

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
| `alerts.error_rate_pct` | float | — | Alert if any endpoint's error rate exceeds this % |
| `alerts.p95_latency_ms` | float | — | Alert if any endpoint's p95 latency exceeds this ms |
| `alerts.rps` | float | — | Alert if requests per second exceeds this value |
| `notify.ntfy_topic` | string | **required** | Your ntfy topic name (letters, digits, `-`, `_` only) |
| `notify.ntfy_server` | string | `https://ntfy.sh` | ntfy server URL (must be http/https) |
| `notify.ntfy_token` | string | — | Bearer token for protected topics |

> All `alerts` fields are optional. Planck only checks thresholds you configure — omitted fields are never alerted on.

## Using a Custom Config Path

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


