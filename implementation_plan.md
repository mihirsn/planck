# Plan: `planck watch` — Live Alerting Mode (v0.2.0)

## Overview

This plan outlines the design for a new `planck watch` command that transforms Planck from a one-shot analysis tool into a lightweight, continuous monitoring loop with real-time alerting via [ntfy](https://ntfy.sh/).

The guiding principle throughout: **no daemons, no databases, no new binary dependencies. Just a long-running terminal process.**

---

## Design Philosophy

`planck watch` is NOT a background agent. It is a foreground process the user intentionally runs in a terminal, tmux pane, or as a simple Docker container command. This means:

- ✅ Starts and stops like any other CLI command
- ✅ Logs its own activity to stdout
- ✅ Zero persistent state between runs
- ✅ Negligible idle CPU (sleeps between polls)
- ✅ Controlled memory: each poll fetches only the last N seconds of logs

---

## How It Would Work

```
planck watch --docker my-api --config planck.yml
```

1. Planck reads `planck.yml` to load alert thresholds and ntfy config.
2. Every `interval` seconds (default: 60s), Planck fetches the last `interval` seconds of logs.
3. It runs the full analysis pipeline (parse → filter → metrics).
4. It checks each computed metric against the configured thresholds.
5. If any threshold is breached, it sends an HTTP POST to ntfy with a clear message.
6. It prints a compact "heartbeat" line to stdout and sleeps until the next poll.

---

## Config File: `planck.yml` / `planck.toml`

A simple, user-friendly config file. Planck looks for it in the current directory by default, or the user specifies it with `--config`.

### Option A: YAML (`planck.yml`) — Recommended

**Binary size impact:** +200–400 KB (via `gopkg.in/yaml.v3`)

```yaml
# planck.yml

watch:
  interval: 60s          # How often to poll logs (default: 60s)
  alert_cooldown: 10m    # Don't re-alert the same threshold within this window
  preset: fastapi        # Same as --preset flag

alerts:
  error_rate_pct: 10.0   # Alert if ANY endpoint exceeds 10% error rate
  p95_latency_ms: 2000   # Alert if ANY endpoint's p95 latency exceeds 2s
  rps: 100               # Alert if requests per second exceeds 100

notify:
  ntfy_topic: "my-api-prod-alerts"    # Required: your ntfy topic name
  ntfy_server: "https://ntfy.sh"      # Optional: defaults to ntfy.sh, for self-hosted
  ntfy_token: ""                      # Optional: for protected topics
```

**Why YAML:**
- Most familiar format for cloud-native developers (Docker Compose, Kubernetes, GitHub Actions all use YAML)
- Supports comments — critical for a config file users need to understand and edit
- Feels completely at home alongside a `docker-compose.yml`
- `gopkg.in/yaml.v3` is battle-tested, used by Kubernetes and nearly every serious Go CLI

---

### Option B: TOML (`planck.toml`)

**Binary size impact:** +150–300 KB (via `github.com/BurntSushi/toml`)

```toml
# planck.toml

[watch]
interval = "60s"
alert_cooldown = "10m"
preset = "fastapi"

[alerts]
error_rate_pct = 10.0
p95_latency_ms = 2000
rps = 100

[notify]
ntfy_topic = "my-api-prod-alerts"
ntfy_server = "https://ntfy.sh"
ntfy_token = ""
```

**Why TOML:**
- Used by Rust's Cargo, Hugo, and many modern developer tools — feels very "dev tool"
- Slightly smaller binary footprint than yaml.v3
- Strong typing — no YAML indentation pitfalls

**Why it loses to YAML for Planck:**
- Less familiar to the Docker/cloud-native audience Planck targets
- `docker-compose.toml` doesn't exist — the mental model is already YAML for this user segment

> [!IMPORTANT]
> **Decision Pending:** Both YAML and TOML have been evaluated. YAML is the leading candidate due to the cloud-native audience fit. The binary size difference between the two is negligible (~100–200 KB) and should not be the deciding factor.

---

### Alert Cooldown Logic

The `alert_cooldown` config prevents notification spam when a threshold stays breached across multiple poll cycles. It is tracked entirely in-memory (a simple `map[string]time.Time` of `"endpoint+metric" → lastAlertedAt`). No persistence is needed — if the user restarts `planck watch`, the cooldown resets, which is acceptable behaviour. A reasonable default would be **10 minutes** if not configured.

---

## Example ntfy Notification

When a threshold is breached, users receive a push notification on their phone or desktop:

```
📡 Planck Alert — enterprise-billing-backend-1
⚠ /api/v1/orders/customer/filter has a high error rate: 52.3% (threshold: 10%)
Detected at 2026-05-24 17:05:00 UTC
```

---

## Proposed File Structure

```
cmd/
  watch.go               [NEW] — Cobra command for `planck watch`
internal/
  config/
    config.go            [NEW] — Reads and validates planck.yml
    config_test.go       [NEW]
  watcher/
    watcher.go           [NEW] — The polling loop and threshold checking logic
    watcher_test.go      [NEW]
  notify/
    ntfy.go              [NEW] — HTTP POST to ntfy (uses stdlib net/http only)
    ntfy_test.go         [NEW]
```

> [!NOTE]
> **Zero new external dependencies.** Config parsing uses Go's standard `gopkg.in/yaml.v3` which is already a transitive dependency in many Go projects — or we can use a simple TOML/JSON format with the stdlib `encoding/json` to stay 100% dependency-free. We can decide this during implementation.

---

## New Cobra Command: `planck watch`

```bash
# Minimal usage — config file provides everything
planck watch --docker my-api --config planck.yml

# Flag overrides for quick testing (no config file needed)
planck watch --docker my-api \
  --ntfy-topic my-api-alerts \
  --alert-error-rate 10 \
  --alert-p95 2000 \
  --interval 60s
```

---

## Resource Impact on t2.micro

| Activity | CPU | Memory |
|---|---|---|
| Sleeping between polls | ~0% | ~10MB (binary RSS) |
| Polling 60s of logs | Brief spike < 1% | Proportional to log volume in that window |
| Sending ntfy alert | ~0% (single HTTP req) | Negligible |

A 60-second polling interval pulling a busy container's logs (~500 lines) would complete in well under 1 second. The process would be idle for the other 59 seconds. **Perfectly safe for t2.micro.**

---

## What Is Deliberately Out of Scope

To protect Planck's lightweight identity, the following will **not** be part of this feature:

- ❌ Running as a systemd/launchd background service
- ❌ Storing historical alert data or a local database
- ❌ Multiple notification channels (Slack, PagerDuty, email) — ntfy covers all of these via its own integrations
- ❌ A web dashboard or status page
- ❌ Alert deduplication / cooldown periods *(could be added in a follow-up)*

---

## Open Questions

1. **Config file format:** YAML is the leading candidate (most familiar to the Docker/cloud-native audience), with TOML as the alternative. Binary size difference between the two is negligible. Both options are documented in the Config File section above.

2. **`ntfy_token` handling:** Should we support reading the token from an environment variable (e.g., `ntfy_token: $NTFY_TOKEN`) to avoid committing secrets to the config file? This is the standard 12-factor approach and especially important for EC2 deployments.

3. **`planck watch` as a dedicated top-level command** — agreed. It is a fundamentally different mode of operation from `analyze` and deserves its own command with its own documentation.

4. **Version `v0.2.0`** — agreed. Adding a new command, a config file system, and an alerting subsystem is a meaningful enough feature set to warrant a minor version bump rather than a patch release.
