# Adding Latency to Your Logs

Latency data (`latency_ms`) is optional — Planck works without it. But without it, the **Slow Endpoints** section will be empty and P95/avg latency won't be shown.

Adding latency is a one-line change in most frameworks and gives you significantly more insight. Here's how to do it in popular web frameworks:

## Go — `chi` router

```go
r.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
        next.ServeHTTP(ww, r)
        log.Printf(`{"timestamp":%q,"method":%q,"path":%q,"status":%d,"latency_ms":%d}`,
            time.Now().UTC().Format(time.RFC3339),
            r.Method, r.URL.Path,
            ww.Status(),
            time.Since(start).Milliseconds(),
        )
    })
})
```

## Python — FastAPI / Starlette

```python
import time, logging
from starlette.middleware.base import BaseHTTPMiddleware

class PlanckLoggingMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request, call_next):
        start = time.time()
        response = await call_next(request)
        latency_ms = int((time.time() - start) * 1000)
        logging.info('{"timestamp":"%s","method":"%s","path":"%s","status":%d,"latency_ms":%d}',
            __import__('datetime').datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ'),
            request.method, request.url.path,
            response.status_code, latency_ms)
        return response

app.add_middleware(PlanckLoggingMiddleware)
```

## Node.js — Express

```javascript
const morgan = require('morgan');

app.use(morgan((tokens, req, res) => {
  return JSON.stringify({
    timestamp: new Date().toISOString(),
    method: tokens.method(req, res),
    path: tokens.url(req, res).split('?')[0],
    status: Number(tokens.status(req, res)),
    latency_ms: Number(tokens['response-time'](req, res))
  });
}));
```
