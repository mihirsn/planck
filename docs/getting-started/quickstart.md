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

Use `watch` to continuously monitor a Docker container's logs and send real-time push notifications when things go wrong.

**Start watching:**

```bash
planck watch --docker my-api
```

For more details on configuring watch mode with alert thresholds, see the [Watch Mode Configuration](../configuration/watch-mode.md).
