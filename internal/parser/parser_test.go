package parser_test

import (
	"testing"
	"time"

	"github.com/mihirsn/planck/internal/models"
	"github.com/mihirsn/planck/internal/parser"
)

// makeChannel creates a channel pre-loaded with the given strings.
func makeChannel(lines []string) <-chan string {
	ch := make(chan string, len(lines))
	for _, l := range lines {
		ch <- l
	}
	close(ch)
	return ch
}

// defaultParser returns a parser using the default (native Planck) field map.
func defaultParser() *parser.Parser {
	return parser.New(models.DefaultFieldMap())
}

func TestParseAll_ValidLines(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/login","status":200,"latency_ms":45}`,
		`{"timestamp":"2026-05-08T14:00:03Z","method":"POST","path":"/invoice","status":201,"latency_ms":130}`,
	}

	entries, malformed := defaultParser().ParseAll(makeChannel(lines))

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", malformed)
	}
	if entries[0].Path != "/login" {
		t.Errorf("expected path /login, got %s", entries[0].Path)
	}
	if entries[0].Status != 200 {
		t.Errorf("expected status 200, got %d", entries[0].Status)
	}
	if entries[0].LatencyMs != 45 {
		t.Errorf("expected latency_ms 45, got %d", entries[0].LatencyMs)
	}
	if entries[0].Method != "GET" {
		t.Errorf("expected method GET, got %s", entries[0].Method)
	}
}

func TestParseAll_MalformedLines(t *testing.T) {
	t.Parallel()

	lines := []string{
		`not json at all`,
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/login","status":200,"latency_ms":45}`,
		`{broken`,
		`{"timestamp":"2026-05-08T14:00:03Z","method":"POST","path":"/invoice","status":201}`,
	}

	entries, malformed := defaultParser().ParseAll(makeChannel(lines))

	if len(entries) != 2 {
		t.Errorf("expected 2 valid entries, got %d", len(entries))
	}
	if malformed != 2 {
		t.Errorf("expected 2 malformed, got %d", malformed)
	}
}

func TestParseAll_EmptyLine(t *testing.T) {
	t.Parallel()

	lines := []string{"", "   "}
	_, malformed := defaultParser().ParseAll(makeChannel(lines))
	if malformed != 2 {
		t.Errorf("expected 2 malformed for empty/whitespace lines, got %d", malformed)
	}
}

func TestParseAll_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want int
	}{
		{
			name: "missing path",
			line: `{"timestamp":"2026-05-08T14:00:01Z","status":200,"latency_ms":10}`,
			want: 0,
		},
		{
			name: "missing status",
			line: `{"timestamp":"2026-05-08T14:00:01Z","path":"/login","latency_ms":10}`,
			want: 0,
		},
		{
			name: "all required present",
			line: `{"timestamp":"2026-05-08T14:00:01Z","path":"/login","status":200}`,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entries, _ := defaultParser().ParseAll(makeChannel([]string{tt.line}))
			if len(entries) != tt.want {
				t.Errorf("expected %d entries, got %d", tt.want, len(entries))
			}
		})
	}
}

func TestParseAll_TimestampParsed(t *testing.T) {
	t.Parallel()

	line := `{"timestamp":"2026-05-08T14:05:00Z","path":"/invoice","status":200,"latency_ms":120}`
	entries, _ := defaultParser().ParseAll(makeChannel([]string{line}))

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	expected := time.Date(2026, 5, 8, 14, 5, 0, 0, time.UTC)
	if !entries[0].Timestamp.Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, entries[0].Timestamp)
	}
}

func TestParseAll_EmptyChannel(t *testing.T) {
	t.Parallel()

	entries, malformed := defaultParser().ParseAll(makeChannel([]string{}))

	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty channel, got %d", len(entries))
	}
	if malformed != 0 {
		t.Errorf("expected 0 malformed for empty channel, got %d", malformed)
	}
}

// --- FieldMap / preset tests ---

func TestParseAll_FastAPIPreset(t *testing.T) {
	t.Parallel()

	// FastAPI log: status_code instead of status, duration (float seconds) instead of latency_ms.
	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/users","status_code":200,"duration":0.120}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"POST","path":"/items","status_code":422,"duration":0.045}`,
	}

	fm, err := models.PresetFieldMap(models.PresetFastAPI)
	if err != nil {
		t.Fatalf("PresetFieldMap: %v", err)
	}
	p := parser.New(fm)
	entries, malformed := p.ParseAll(makeChannel(lines))

	if malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", malformed)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Status != 200 {
		t.Errorf("fastapi: expected status 200, got %d", entries[0].Status)
	}
	// 0.120 seconds → 120ms
	if entries[0].LatencyMs != 120 {
		t.Errorf("fastapi: expected latency 120ms from 0.120s, got %d", entries[0].LatencyMs)
	}
	if entries[1].Status != 422 {
		t.Errorf("fastapi: expected status 422, got %d", entries[1].Status)
	}
	// 0.045 seconds → 45ms
	if entries[1].LatencyMs != 45 {
		t.Errorf("fastapi: expected latency 45ms from 0.045s, got %d", entries[1].LatencyMs)
	}
}

func TestParseAll_ExpressPreset(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","url":"/api/users","statusCode":200,"responseTime":85}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"DELETE","url":"/api/items/1","statusCode":404,"responseTime":12}`,
	}

	fm, _ := models.PresetFieldMap(models.PresetExpress)
	p := parser.New(fm)
	entries, malformed := p.ParseAll(makeChannel(lines))

	if malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", malformed)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Path != "/api/users" {
		t.Errorf("express: expected path /api/users, got %s", entries[0].Path)
	}
	if entries[0].Status != 200 {
		t.Errorf("express: expected status 200, got %d", entries[0].Status)
	}
	if entries[0].LatencyMs != 85 {
		t.Errorf("express: expected latency 85ms, got %d", entries[0].LatencyMs)
	}
}

func TestParseAll_CustomFieldMap(t *testing.T) {
	t.Parallel()

	// Fully custom schema.
	lines := []string{
		`{"ts":"2026-05-08T14:00:01Z","verb":"GET","endpoint":"/health","code":200,"dur":33}`,
	}

	fm := models.FieldMap{
		Timestamp: "ts",
		Method:    "verb",
		Path:      "endpoint",
		Status:    "code",
		LatencyMs: "dur",
	}
	p := parser.New(fm)
	entries, malformed := p.ParseAll(makeChannel(lines))

	if malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", malformed)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "/health" {
		t.Errorf("custom: expected path /health, got %s", entries[0].Path)
	}
	if entries[0].Status != 200 {
		t.Errorf("custom: expected status 200, got %d", entries[0].Status)
	}
	if entries[0].LatencyMs != 33 {
		t.Errorf("custom: expected latency 33ms, got %d", entries[0].LatencyMs)
	}
}

func TestParseAll_FloatLatency_ConvertedFromSeconds(t *testing.T) {
	t.Parallel()

	// Float latency is treated as seconds and converted to ms.
	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/api","status":200,"latency_ms":0.250}`,
	}

	entries, _ := defaultParser().ParseAll(makeChannel(lines))
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// 0.250 seconds → 250ms
	if entries[0].LatencyMs != 250 {
		t.Errorf("expected latency 250ms from 0.250s, got %d", entries[0].LatencyMs)
	}
}

func TestParseAll_IntegerLatency_UsedDirectly(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/api","status":200,"latency_ms":500}`,
	}

	entries, _ := defaultParser().ParseAll(makeChannel(lines))
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].LatencyMs != 500 {
		t.Errorf("expected latency 500ms, got %d", entries[0].LatencyMs)
	}
}

func TestParseAll_GinPreset(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"time":"2026-05-08T14:00:01Z","method":"GET","path":"/ping","status":200,"latency":5}`,
	}

	fm, _ := models.PresetFieldMap(models.PresetGin)
	p := parser.New(fm)
	entries, malformed := p.ParseAll(makeChannel(lines))

	if malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", malformed)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].LatencyMs != 5 {
		t.Errorf("gin: expected latency 5ms, got %d", entries[0].LatencyMs)
	}
}

func TestParseAll_SpringPreset(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","uri":"/actuator/health","status":200,"duration":12}`,
	}

	fm, _ := models.PresetFieldMap(models.PresetSpring)
	p := parser.New(fm)
	entries, malformed := p.ParseAll(makeChannel(lines))

	if malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", malformed)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "/actuator/health" {
		t.Errorf("spring: expected path /actuator/health, got %s", entries[0].Path)
	}
}
