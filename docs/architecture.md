# Architecture & Philosophy

## Philosophy

> Meaningful insights from logs with near-zero operational overhead.

Planck is intentionally **not**:
- Another Prometheus
- Another Grafana  
- Another observability platform

It's a sharp, focused tool for developers who want answers quickly. It runs completely locally without external dependencies. 

## High-Level Architecture

Currently, Planck works on a buffered-read architecture:
1. **Source Layer:** Connects to the log source (Docker daemon or local file) and streams lines as strings.
2. **Parser Layer:** Uses `github.com/valyala/fastjson` to rapidly parse lines. It buffers all parsed `LogEntry` structs in memory.
3. **Metrics Layer:** Iterates over the slice of entries in memory to calculate percentiles, RPS, and error rates.
4. **Formatter Layer:** Renders the computed `Report` struct to the terminal or JSON.

## Roadmap & Streaming Accumulator

The current architecture means that **memory usage scales linearly with the number of log lines**.

> **Workaround (today):** Use `--since` and `--until` to analyze a bounded time window instead of the entire log history. For example, `--since 1h` restricts analysis to the last hour of traffic, keeping memory usage low and execution fast. This is the recommended approach for large containers until the streaming engine is implemented.
>
> See the [Analyze Flags reference](configuration/analyze-flags.md) for the full `--since` / `--until` usage.

### Why the Accumulator Engine is Needed
If you analyze 10 million log lines, Planck will attempt to allocate 10 million structs in RAM. On a tiny VM (like an AWS `t2.micro` with 1GB RAM), this will cause Planck to crash with an Out Of Memory (OOM) error.

### The Solution: V2 Streaming Architecture
Planning to rewrite the core engine to use a streaming accumulator:

Instead of buffering entries:
1. The parser reads a line.
2. The aggregator updates running counters (e.g. `Endpoints["/login"].TotalRequests++`).
3. For latency percentiles (like p95), we can switch to using a space-efficient streaming quantile sketch algorithm (like t-digest or HDR Histogram).
4. The line is immediately discarded to garbage collection.

This will cap Planck's memory footprint to a flat ~10-20MB regardless of whether it analyzes 10 thousand or 10 billion log lines.
