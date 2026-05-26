# Custom Fields & Log Formats

Planck reads **one JSON object per line** ([JSON Lines](https://jsonlines.org/) format).

> **Note on log format**: Planck currently requires a specific JSON schema (described below). Use `--preset` or `--field-*` flags to adapt Planck to your existing log format.

## The Standard Schema

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

## Field Mapping

If your JSON logs don't use the exact keys `timestamp`, `path`, `status`, `method`, and `latency_ms`, you can tell Planck how to read your format.

### Option A: Built-in Presets

The easiest way is to use a `--preset`. This automatically configures Planck to understand the default JSON log format for popular web frameworks.

```bash
planck analyze --docker my-api --preset fastapi
```

**Supported presets:**
- `fastapi` — Standard FastAPI / Uvicorn JSON logs
- `express` — Express.js + Morgan JSON logs
- `gin` — Go Gin default JSON logger
- `echo` — Go Echo default JSON logger
- `spring-boot` — Spring Boot `logstash-logback-encoder`

### Option B: Custom Field Flags

If you use a custom schema, use the `--field-*` flags to map your keys manually.

```bash
planck analyze app.log \
  --field-timestamp "@timestamp" \
  --field-path "req.url" \
  --field-status "res.status_code" \
  --field-method "req.method" \
  --field-latency "duration_ms"
```

*Note: Nested fields are supported using dot notation (e.g., `req.url`).*

## Stripping Log Prefixes

Some frameworks wrap JSON logs in a text prefix, which makes them invalid JSON. For example, Python's logging module might output:

`INFO:uvicorn.access:{"timestamp":"2026-05-08...`

Use the `--scan-json` flag to tell Planck to find the `{` character and extract only the JSON payload:

```bash
planck analyze app.log --scan-json
```
