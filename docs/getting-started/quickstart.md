# Quickstart

Planck has two primary modes of operation: `analyze` for ad-hoc log parsing and `watch` for continuous monitoring.

## 1. Analyze Mode

Use `analyze` to parse a log file or a Docker container's current log stream and generate a summary report.

### Analyze a log file

```bash
planck analyze app.log
```

### Analyze Docker container logs

```bash
planck analyze --docker my-api
```

## 2. Watch Mode

Use `watch` to continuously monitor one or more Docker containers' logs and send real-time push notifications when things go wrong.

All configuration lives in `planck.yml`. Start by creating one in your working directory:

```yaml
watch:
  docker: my-api        # Container to monitor (or use containers: for multi-container)
  interval: 60s
  preset: fastapi

alerts:
  error_rate:
    threshold: 10.0

notify:
  ntfy:
    topic: my-api-alerts
```

Then run:

```bash
planck watch
```

To monitor **multiple containers**, replace `watch.docker` with a `containers:` list:

```yaml
watch:
  interval: 60s
  preset: fastapi

alerts:
  error_rate:
    threshold: 10.0

notify:
  ntfy:
    topic: my-alerts

containers:
  - name: my-api
  - name: my-worker
    alerts:
      error_rate:
        threshold: 2.0    # Stricter threshold for this container
```

For full configuration options, see the [Watch Mode Reference](../configuration/watch-mode.md).
