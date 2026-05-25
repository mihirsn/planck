# Analyze Flags Reference

The `analyze` command provides a variety of flags to filter, slice, and format your logs.

## Time Filtering

### Fetch only the last N lines from Docker

```bash
planck analyze --docker my-api --tail 1000
```

### Filter Docker logs by time

```bash
planck analyze --docker my-api --since 1h
planck analyze --docker my-api --since 3d  # Days are supported!
```

### Filter file logs by time

```bash
planck analyze app.log --since 12h
planck analyze app.log --since 2026-05-10T08:00:00Z --until 2026-05-10T18:00:00Z
```

## Allowlist Filtering (HTTP Method / Status)

```bash
planck analyze app.log --filter-status 5xx                 # Only server errors
planck analyze app.log --filter-method POST                # Only POST requests
planck analyze app.log --filter-status 5xx --filter-method POST  # Combine filters
```

## Blocklist Filtering (Exclude Noisy Traffic)

```bash
planck analyze --docker my-api --exclude-path /health --exclude-path /metrics
planck analyze app.log --exclude-status 404 --exclude-status 401
planck analyze app.log --exclude-method OPTIONS --exclude-method HEAD
```

## Output Formatting

### Machine-readable JSON output

Useful for piping results into `jq` or integrating Planck into CI pipelines.

```bash
planck analyze app.log --format json
planck analyze app.log --format json | jq '.top_endpoints'
```

### Control how many endpoints are shown

```bash
planck analyze app.log --top 10     # show top 10 endpoints (default: 5)
planck analyze app.log --slow 3     # show 3 slowest endpoints (default: 5)
```
