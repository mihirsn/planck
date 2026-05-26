package watcher_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mihirsn/planck/internal/config"
	"github.com/mihirsn/planck/internal/models"
	"github.com/mihirsn/planck/internal/source"
	"github.com/mihirsn/planck/internal/watcher"
)

// fakeSource implements source.LogSource from a slice of pre-defined log lines.
type fakeSource struct {
	lines []string
}

func (f *fakeSource) Stream() (<-chan string, error) {
	ch := make(chan string, len(f.lines))
	for _, l := range f.lines {
		ch <- l
	}
	close(ch)
	return ch, nil
}

// makeConfig builds a minimal valid Config for tests. ntfyURL should point at a test server.
func makeConfig(t *testing.T, ntfyURL string, opts ...func(*config.Config)) *config.Config {
	t.Helper()
	cfg := &config.Config{
		Watch: config.WatchConfig{
			IntervalDuration: 60 * time.Second,
			CooldownDuration: 1 * time.Millisecond, // near-zero so alerts always fire in tests
			Preset:           "",
		},
		Alerts: config.AlertConfig{
			ErrorRatePct: 10.0,
			P95LatencyMs: 500.0,
			RPS:          1000.0,
		},
		Notify: config.NotifyConfig{
			NtfyTopic:  "test-topic",
			NtfyServer: ntfyURL,
			NtfyToken:  "",
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// defaultFieldMap returns a standard field map for tests.
func defaultFieldMap() models.FieldMap {
	return models.FieldMap{
		Timestamp: "timestamp",
		Method:    "method",
		Path:      "path",
		Status:    "status",
		LatencyMs: "latency_ms",
	}
}

// injectSource wires a fake source into the watcher's NewSource function.
func injectSource(w *watcher.Watcher, lines []string) {
	w.NewSource = func(container string, tail int, since string) (source.LogSource, error) {
		return &fakeSource{lines: lines}, nil
	}
}

// injectErrorSource makes the source fail to open.
func injectErrorSource(w *watcher.Watcher) {
	w.NewSource = func(container string, tail int, since string) (source.LogSource, error) {
		return nil, fmt.Errorf("docker: container not found")
	}
}

func TestWatcher_NoEntries(t *testing.T) {
	var out bytes.Buffer
	w := watcher.New(makeConfig(t, "http://unused"), "my-api", defaultFieldMap(), &out)
	injectSource(w, []string{}) // empty log window

	stop := make(chan struct{})
	close(stop) // stop immediately after first poll

	// Give poll a moment to run before stop closes.
	go w.Run(stop)
	time.Sleep(50 * time.Millisecond)

	if !strings.Contains(out.String(), "quiet window") {
		t.Errorf("expected 'quiet window' in output, got: %s", out.String())
	}
}

func TestWatcher_SourceError(t *testing.T) {
	var out bytes.Buffer
	w := watcher.New(makeConfig(t, "http://unused"), "bad-container", defaultFieldMap(), &out)
	injectErrorSource(w)

	stop := make(chan struct{})
	go w.Run(stop)
	time.Sleep(50 * time.Millisecond)
	close(stop)
	time.Sleep(20 * time.Millisecond)

	if !strings.Contains(out.String(), "Failed to open log source") {
		t.Errorf("expected error message in output, got: %s", out.String())
	}
}

func TestWatcher_AlertOnHighErrorRate(t *testing.T) {
	var alertCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&alertCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 2 out of 2 requests are 500s → 100% error rate, threshold is 10%
	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/api/orders","status":500,"latency_ms":120}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"GET","path":"/api/orders","status":500,"latency_ms":130}`,
	}

	var out bytes.Buffer
	w := watcher.New(makeConfig(t, srv.URL), "my-api", defaultFieldMap(), &out)
	injectSource(w, lines)

	stop := make(chan struct{})
	go w.Run(stop)
	time.Sleep(150 * time.Millisecond)
	close(stop)
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&alertCount) == 0 {
		t.Errorf("expected at least one alert to be sent, got none. Output: %s", out.String())
	}
	if !strings.Contains(out.String(), "Alert sent") {
		t.Errorf("expected 'Alert sent' in output, got: %s", out.String())
	}
}

func TestWatcher_AlertOnHighP95(t *testing.T) {
	var alertCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&alertCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Requests with very high latency but 200 OK — should breach p95 threshold of 500ms
	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/api/slow","status":200,"latency_ms":2000}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"GET","path":"/api/slow","status":200,"latency_ms":2100}`,
		`{"timestamp":"2026-05-08T14:00:03Z","method":"GET","path":"/api/slow","status":200,"latency_ms":1900}`,
	}

	var out bytes.Buffer
	w := watcher.New(makeConfig(t, srv.URL), "my-api", defaultFieldMap(), &out)
	injectSource(w, lines)

	stop := make(chan struct{})
	go w.Run(stop)
	time.Sleep(150 * time.Millisecond)
	close(stop)
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&alertCount) == 0 {
		t.Errorf("expected p95 alert to be sent, got none. Output: %s", out.String())
	}
}

func TestWatcher_NoBreach_NoAlert(t *testing.T) {
	var alertCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&alertCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Healthy requests — 200 OK, fast latency
	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/api/orders","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"GET","path":"/api/orders","status":200,"latency_ms":15}`,
	}

	var out bytes.Buffer
	w := watcher.New(makeConfig(t, srv.URL), "my-api", defaultFieldMap(), &out)
	injectSource(w, lines)

	stop := make(chan struct{})
	go w.Run(stop)
	time.Sleep(150 * time.Millisecond)
	close(stop)
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&alertCount) > 0 {
		t.Errorf("expected no alerts for healthy requests, got %d. Output: %s", alertCount, out.String())
	}
}

func TestWatcher_CooldownPreventsRepeatAlert(t *testing.T) {
	var alertCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&alertCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/api/orders","status":500,"latency_ms":120}`,
	}

	var out bytes.Buffer
	// Set a long cooldown so the second poll won't alert again.
	cfg := makeConfig(t, srv.URL, func(c *config.Config) {
		c.Watch.CooldownDuration = 1 * time.Hour
		c.Watch.IntervalDuration = 30 * time.Millisecond // fast polling for this test
	})

	w := watcher.New(cfg, "my-api", defaultFieldMap(), &out)
	// Each poll gets the same failing line.
	w.NewSource = func(container string, tail int, since string) (source.LogSource, error) {
		return &fakeSource{lines: lines}, nil
	}

	stop := make(chan struct{})
	go w.Run(stop)
	time.Sleep(200 * time.Millisecond) // enough for multiple poll cycles
	close(stop)
	time.Sleep(50 * time.Millisecond)

	count := atomic.LoadInt32(&alertCount)
	if count != 1 {
		t.Errorf("expected exactly 1 alert (cooldown should suppress repeats), got %d", count)
	}
}

func TestWatcher_StopsCleanly(t *testing.T) {
	var out bytes.Buffer
	w := watcher.New(makeConfig(t, "http://unused"), "my-api", defaultFieldMap(), &out)
	injectSource(w, []string{})

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		w.Run(stop)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	close(stop)

	select {
	case <-done:
		// good — Run returned
	case <-time.After(2 * time.Second):
		t.Error("watcher did not stop within 2 seconds")
	}

	if !strings.Contains(out.String(), "stopped") {
		t.Errorf("expected 'stopped' in output, got: %s", out.String())
	}
}
