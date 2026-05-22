package parser_test

import (
	"strings"
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

	result := defaultParser().ParseAll(makeChannel(lines))

	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.Malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", result.Malformed)
	}
	if result.Entries[0].Path != "/login" {
		t.Errorf("expected path /login, got %s", result.Entries[0].Path)
	}
	if result.Entries[0].Status != 200 {
		t.Errorf("expected status 200, got %d", result.Entries[0].Status)
	}
	if result.Entries[0].LatencyMs != 45 {
		t.Errorf("expected latency_ms 45, got %d", result.Entries[0].LatencyMs)
	}
	if result.Entries[0].Method != "GET" {
		t.Errorf("expected method GET, got %s", result.Entries[0].Method)
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

	result := defaultParser().ParseAll(makeChannel(lines))

	if len(result.Entries) != 2 {
		t.Errorf("expected 2 valid entries, got %d", len(result.Entries))
	}
	if result.Malformed != 2 {
		t.Errorf("expected 2 malformed, got %d", result.Malformed)
	}
}

func TestParseAll_EmptyLine(t *testing.T) {
	t.Parallel()

	lines := []string{"", "   "}
	result := defaultParser().ParseAll(makeChannel(lines))
	if result.Malformed != 2 {
		t.Errorf("expected 2 malformed for empty/whitespace lines, got %d", result.Malformed)
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
			result := defaultParser().ParseAll(makeChannel([]string{tt.line}))
			if len(result.Entries) != tt.want {
				t.Errorf("expected %d entries, got %d", tt.want, len(result.Entries))
			}
		})
	}
}

func TestParseAll_TimestampParsed(t *testing.T) {
	t.Parallel()

	line := `{"timestamp":"2026-05-08T14:05:00Z","path":"/invoice","status":200,"latency_ms":120}`
	result := defaultParser().ParseAll(makeChannel([]string{line}))

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	expected := time.Date(2026, 5, 8, 14, 5, 0, 0, time.UTC)
	if !result.Entries[0].Timestamp.Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, result.Entries[0].Timestamp)
	}
}

func TestParseAll_EmptyChannel(t *testing.T) {
	t.Parallel()

	result := defaultParser().ParseAll(makeChannel([]string{}))

	if len(result.Entries) != 0 {
		t.Errorf("expected 0 entries for empty channel, got %d", len(result.Entries))
	}
	if result.Malformed != 0 {
		t.Errorf("expected 0 malformed for empty channel, got %d", result.Malformed)
	}
}

// --- FieldMap / preset tests ---

func TestParseAll_FastAPIPreset(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/users","status_code":200,"duration":0.120}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"POST","path":"/items","status_code":422,"duration":0.045}`,
	}

	fm, err := models.PresetFieldMap(models.PresetFastAPI)
	if err != nil {
		t.Fatalf("PresetFieldMap: %v", err)
	}
	result := parser.New(fm).ParseAll(makeChannel(lines))

	if result.Malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", result.Malformed)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].Status != 200 {
		t.Errorf("fastapi: expected status 200, got %d", result.Entries[0].Status)
	}
	// 0.120 seconds → 120ms
	if result.Entries[0].LatencyMs != 120 {
		t.Errorf("fastapi: expected latency 120ms from 0.120s, got %d", result.Entries[0].LatencyMs)
	}
	if result.Entries[1].Status != 422 {
		t.Errorf("fastapi: expected status 422, got %d", result.Entries[1].Status)
	}
	// 0.045 seconds → 45ms
	if result.Entries[1].LatencyMs != 45 {
		t.Errorf("fastapi: expected latency 45ms from 0.045s, got %d", result.Entries[1].LatencyMs)
	}
}

func TestParseAll_ExpressPreset(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","url":"/api/users","statusCode":200,"responseTime":85}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"DELETE","url":"/api/items/1","statusCode":404,"responseTime":12}`,
	}

	fm, _ := models.PresetFieldMap(models.PresetExpress)
	result := parser.New(fm).ParseAll(makeChannel(lines))

	if result.Malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", result.Malformed)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "/api/users" {
		t.Errorf("express: expected path /api/users, got %s", result.Entries[0].Path)
	}
	if result.Entries[0].Status != 200 {
		t.Errorf("express: expected status 200, got %d", result.Entries[0].Status)
	}
	if result.Entries[0].LatencyMs != 85 {
		t.Errorf("express: expected latency 85ms, got %d", result.Entries[0].LatencyMs)
	}
}

func TestParseAll_CustomFieldMap(t *testing.T) {
	t.Parallel()

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
	result := parser.New(fm).ParseAll(makeChannel(lines))

	if result.Malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", result.Malformed)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "/health" {
		t.Errorf("custom: expected path /health, got %s", result.Entries[0].Path)
	}
	if result.Entries[0].LatencyMs != 33 {
		t.Errorf("custom: expected latency 33ms, got %d", result.Entries[0].LatencyMs)
	}
}

func TestParseAll_FloatLatency_ConvertedFromSeconds(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/api","status":200,"latency_ms":0.250}`,
	}

	result := defaultParser().ParseAll(makeChannel(lines))
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	// 0.250 seconds → 250ms
	if result.Entries[0].LatencyMs != 250 {
		t.Errorf("expected latency 250ms from 0.250s, got %d", result.Entries[0].LatencyMs)
	}
}

func TestParseAll_IntegerLatency_UsedDirectly(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/api","status":200,"latency_ms":500}`,
	}

	result := defaultParser().ParseAll(makeChannel(lines))
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].LatencyMs != 500 {
		t.Errorf("expected latency 500ms, got %d", result.Entries[0].LatencyMs)
	}
}

func TestParseAll_GinPreset(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"time":"2026-05-08T14:00:01Z","method":"GET","path":"/ping","status":200,"latency":5}`,
	}

	fm, _ := models.PresetFieldMap(models.PresetGin)
	result := parser.New(fm).ParseAll(makeChannel(lines))

	if result.Malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", result.Malformed)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].LatencyMs != 5 {
		t.Errorf("gin: expected latency 5ms, got %d", result.Entries[0].LatencyMs)
	}
}

func TestParseAll_SpringPreset(t *testing.T) {
	t.Parallel()

	// Spring Boot with logstash-logback-encoder uses @timestamp.
	lines := []string{
		`{"@timestamp":"2026-05-08T14:00:01Z","method":"GET","uri":"/actuator/health","status":200,"duration":12}`,
	}

	fm, _ := models.PresetFieldMap(models.PresetSpring)
	result := parser.New(fm).ParseAll(makeChannel(lines))

	if result.Malformed != 0 {
		t.Errorf("expected 0 malformed, got %d", result.Malformed)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "/actuator/health" {
		t.Errorf("spring: expected path /actuator/health, got %s", result.Entries[0].Path)
	}
}

// --- scan-json tests ---

func TestParseAll_ScanJSON_StripsPythonPrefix(t *testing.T) {
	t.Parallel()

	// Python default logging format: "LEVEL:logger_name:{json}"
	lines := []string{
		`INFO:api.access:{"timestamp":"2026-05-08T14:00:01Z","path":"/users","status":200,"latency_ms":45}`,
		`WARNING:api.access:{"timestamp":"2026-05-08T14:00:02Z","path":"/items","status":500,"latency_ms":980}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetScanJSON(true)
	result := p.ParseAll(makeChannel(lines))

	if result.Malformed != 0 {
		t.Errorf("scan-json: expected 0 malformed, got %d", result.Malformed)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("scan-json: expected 2 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "/users" {
		t.Errorf("scan-json: expected path /users, got %s", result.Entries[0].Path)
	}
	if result.Entries[1].Status != 500 {
		t.Errorf("scan-json: expected status 500, got %d", result.Entries[1].Status)
	}
}

func TestParseAll_ScanJSON_PureJSONStillWorks(t *testing.T) {
	t.Parallel()

	// Pure JSON lines (no prefix) must continue to work when --scan-json is enabled.
	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/api","status":200,"latency_ms":10}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetScanJSON(true)
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Fatalf("scan-json pure JSON: expected 1 entry, got %d", len(result.Entries))
	}
}

func TestParseAll_ScanJSON_NoBraceIsStillMalformed(t *testing.T) {
	t.Parallel()

	// Lines with no '{' at all should still be counted as malformed.
	lines := []string{
		`INFO:app: Server started on port 8080`,
		`plain text log without any json`,
	}

	p := parser.New(models.DefaultFieldMap()).SetScanJSON(true)
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 0 {
		t.Errorf("scan-json no-brace: expected 0 entries, got %d", len(result.Entries))
	}
	if result.Malformed != 2 {
		t.Errorf("scan-json no-brace: expected 2 malformed, got %d", result.Malformed)
	}
}

// --- PrefixedJSON hint detection tests ---

func TestParseAll_PrefixedJSON_DetectedWhenScanDisabled(t *testing.T) {
	t.Parallel()

	// When --scan-json is OFF, prefixed lines should populate PrefixedJSON counter.
	lines := []string{
		`INFO:api.access:{"timestamp":"2026-05-08T14:00:01Z","path":"/users","status":200,"latency_ms":45}`,
		`{"timestamp":"2026-05-08T14:00:02Z","path":"/items","status":201,"latency_ms":90}`,
	}

	result := defaultParser().ParseAll(makeChannel(lines))

	// The prefixed line should be malformed (not parsed)...
	if len(result.Entries) != 1 {
		t.Errorf("expected 1 valid entry (the pure JSON line), got %d", len(result.Entries))
	}
	if result.Malformed != 1 {
		t.Errorf("expected 1 malformed, got %d", result.Malformed)
	}
	// ...but should be detected as prefixed JSON for the hint.
	if result.PrefixedJSON != 1 {
		t.Errorf("expected PrefixedJSON=1 for the hint, got %d", result.PrefixedJSON)
	}
}

func TestParseAll_PrefixedJSON_NotSetWhenScanEnabled(t *testing.T) {
	t.Parallel()

	// When --scan-json is ON, prefixed lines are parsed successfully, so
	// PrefixedJSON should remain 0.
	lines := []string{
		`INFO:api.access:{"timestamp":"2026-05-08T14:00:01Z","path":"/users","status":200,"latency_ms":45}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetScanJSON(true)
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry with scan-json on, got %d", len(result.Entries))
	}
	if result.PrefixedJSON != 0 {
		t.Errorf("expected PrefixedJSON=0 when scan-json is on, got %d", result.PrefixedJSON)
	}
}

// --- exclude-path tests ---

func TestParseAll_ExcludePath_PrefixMatch(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/health","status":200,"latency_ms":1}`,
		`{"timestamp":"2026-05-08T14:00:02Z","path":"/health/check","status":200,"latency_ms":1}`,
		`{"timestamp":"2026-05-08T14:00:03Z","path":"/api/orders","status":200,"latency_ms":45}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetExcludePaths([]string{"/health"})
	result := p.ParseAll(makeChannel(lines))

	// /health and /health/check should be excluded, /api/orders should remain.
	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry after exclusion, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "/api/orders" {
		t.Errorf("expected /api/orders, got %s", result.Entries[0].Path)
	}
	if result.Excluded != 2 {
		t.Errorf("expected Excluded=2, got %d", result.Excluded)
	}
}

func TestParseAll_ExcludePath_MultiplePatterns(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/health","status":200,"latency_ms":1}`,
		`{"timestamp":"2026-05-08T14:00:02Z","path":"/metrics","status":200,"latency_ms":1}`,
		`{"timestamp":"2026-05-08T14:00:03Z","path":"/api/v1/internal/cleanup","status":200,"latency_ms":5}`,
		`{"timestamp":"2026-05-08T14:00:04Z","path":"/api/orders","status":200,"latency_ms":45}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetExcludePaths([]string{"/health", "/metrics", "/api/v1/internal"})
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry after exclusion, got %d", len(result.Entries))
	}
	if result.Excluded != 3 {
		t.Errorf("expected Excluded=3, got %d", result.Excluded)
	}
}

func TestParseAll_ExcludePath_NotCountedAsMalformed(t *testing.T) {
	t.Parallel()

	// Excluded entries must NOT increment the Malformed counter.
	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/health","status":200,"latency_ms":1}`,
		`not json`,
	}

	p := parser.New(models.DefaultFieldMap()).SetExcludePaths([]string{"/health"})
	result := p.ParseAll(makeChannel(lines))

	if result.Excluded != 1 {
		t.Errorf("expected Excluded=1, got %d", result.Excluded)
	}
	if result.Malformed != 1 {
		t.Errorf("expected Malformed=1 (only for the non-JSON line), got %d", result.Malformed)
	}
}

func TestParseAll_ExcludePath_EmptyPaths_NoEffect(t *testing.T) {
	t.Parallel()

	// Empty exclude list should not affect any entries.
	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/health","status":200,"latency_ms":1}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetExcludePaths([]string{})
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry with empty exclude list, got %d", len(result.Entries))
	}
	if result.Excluded != 0 {
		t.Errorf("expected Excluded=0, got %d", result.Excluded)
	}
}

// --- filter-status tests ---

func TestParseAll_FilterStatus_ClassPattern_5xx(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/api","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","path":"/api","status":500,"latency_ms":80}`,
		`{"timestamp":"2026-05-08T14:00:03Z","path":"/api","status":503,"latency_ms":90}`,
		`{"timestamp":"2026-05-08T14:00:04Z","path":"/api","status":404,"latency_ms":5}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetStatusFilter("5xx")
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 2 {
		t.Errorf("expected 2 5xx entries, got %d", len(result.Entries))
	}
	if result.Filtered != 2 {
		t.Errorf("expected Filtered=2 (200 and 404), got %d", result.Filtered)
	}
	for _, e := range result.Entries {
		if e.Status < 500 || e.Status >= 600 {
			t.Errorf("unexpected non-5xx status %d in filtered result", e.Status)
		}
	}
}

func TestParseAll_FilterStatus_ClassPattern_4xx(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/a","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","path":"/b","status":400,"latency_ms":5}`,
		`{"timestamp":"2026-05-08T14:00:03Z","path":"/c","status":404,"latency_ms":5}`,
		`{"timestamp":"2026-05-08T14:00:04Z","path":"/d","status":500,"latency_ms":90}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetStatusFilter("4xx")
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 2 {
		t.Errorf("expected 2 4xx entries, got %d", len(result.Entries))
	}
	if result.Filtered != 2 {
		t.Errorf("expected Filtered=2, got %d", result.Filtered)
	}
}

func TestParseAll_FilterStatus_ExactCode(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/a","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","path":"/b","status":201,"latency_ms":20}`,
		`{"timestamp":"2026-05-08T14:00:03Z","path":"/c","status":404,"latency_ms":5}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetStatusFilter("201")
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry with status 201, got %d", len(result.Entries))
	}
	if result.Filtered != 2 {
		t.Errorf("expected Filtered=2 (200 and 404), got %d", result.Filtered)
	}
}

func TestParseAll_FilterStatus_EmptyFilter_NoEffect(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/a","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","path":"/b","status":500,"latency_ms":80}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetStatusFilter("")
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries with empty filter, got %d", len(result.Entries))
	}
	if result.Filtered != 0 {
		t.Errorf("expected Filtered=0, got %d", result.Filtered)
	}
}

// --- time range (--since / --until) tests ---

func TestParseAll_TimeRange_SinceFiltersOldEntries(t *testing.T) {
	t.Parallel()

	// sinceTime is between the two entries.
	sinceTime := time.Date(2026, 5, 8, 14, 0, 30, 0, time.UTC)

	lines := []string{
		// Before sinceTime — should be filtered.
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/old","status":200,"latency_ms":10}`,
		// After sinceTime — should be included.
		`{"timestamp":"2026-05-08T14:01:00Z","path":"/new","status":200,"latency_ms":20}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetTimeRange(sinceTime, time.Time{})
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry after --since filter, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "/new" {
		t.Errorf("expected /new, got %s", result.Entries[0].Path)
	}
	if result.Filtered != 1 {
		t.Errorf("expected Filtered=1, got %d", result.Filtered)
	}
}

func TestParseAll_TimeRange_UntilFiltersNewEntries(t *testing.T) {
	t.Parallel()

	// untilTime is between the two entries.
	untilTime := time.Date(2026, 5, 8, 14, 0, 30, 0, time.UTC)

	lines := []string{
		// Before untilTime — should be included.
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/old","status":200,"latency_ms":10}`,
		// After untilTime — should be filtered.
		`{"timestamp":"2026-05-08T14:01:00Z","path":"/new","status":200,"latency_ms":20}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetTimeRange(time.Time{}, untilTime)
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry after --until filter, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "/old" {
		t.Errorf("expected /old, got %s", result.Entries[0].Path)
	}
	if result.Filtered != 1 {
		t.Errorf("expected Filtered=1, got %d", result.Filtered)
	}
}

func TestParseAll_TimeRange_ZeroTimesNoEffect(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","path":"/a","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T15:00:00Z","path":"/b","status":200,"latency_ms":20}`,
	}

	// Both zero — no filtering should happen.
	p := parser.New(models.DefaultFieldMap()).SetTimeRange(time.Time{}, time.Time{})
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries with zero time range, got %d", len(result.Entries))
	}
	if result.Filtered != 0 {
		t.Errorf("expected Filtered=0, got %d", result.Filtered)
	}
}

// --- filter-method tests ---

func TestParseAll_FilterMethod_ExactMatch(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/a","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"POST","path":"/b","status":200,"latency_ms":20}`,
		`{"timestamp":"2026-05-08T14:00:03Z","method":"PUT","path":"/c","status":200,"latency_ms":30}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetMethodFilter("POST")
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Method != "POST" {
		t.Errorf("expected POST, got %s", result.Entries[0].Method)
	}
	if result.Filtered != 2 {
		t.Errorf("expected Filtered=2, got %d", result.Filtered)
	}
}

func TestParseAll_FilterMethod_CaseInsensitive(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/a","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"post","path":"/b","status":200,"latency_ms":20}`,
	}

	// Filter with lowercase "post"
	p := parser.New(models.DefaultFieldMap()).SetMethodFilter("post")
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if !strings.EqualFold(result.Entries[0].Method, "POST") {
		t.Errorf("expected POST, got %s", result.Entries[0].Method)
	}
	if result.Filtered != 1 {
		t.Errorf("expected Filtered=1, got %d", result.Filtered)
	}
}

func TestParseAll_FilterMethod_EmptyFilter_NoEffect(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/a","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"POST","path":"/b","status":200,"latency_ms":20}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetMethodFilter("")
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries with empty method filter, got %d", len(result.Entries))
	}
	if result.Filtered != 0 {
		t.Errorf("expected Filtered=0, got %d", result.Filtered)
	}
}

// --- exclude blocklist tests ---

func TestParseAll_ExcludeStatuses(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/a","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"GET","path":"/b","status":401,"latency_ms":20}`,
		`{"timestamp":"2026-05-08T14:00:03Z","method":"GET","path":"/c","status":404,"latency_ms":30}`,
		`{"timestamp":"2026-05-08T14:00:04Z","method":"GET","path":"/d","status":500,"latency_ms":40}`,
	}

	// Exclude 401 exactly, and all 5xx
	p := parser.New(models.DefaultFieldMap()).SetExcludeStatuses([]string{"401", "5xx"})
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].Status != 200 || result.Entries[1].Status != 404 {
		t.Errorf("expected 200 and 404 to remain, got %d and %d", result.Entries[0].Status, result.Entries[1].Status)
	}
	if result.Filtered != 2 {
		t.Errorf("expected Filtered=2, got %d", result.Filtered)
	}
}

func TestParseAll_ExcludeMethods(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/a","status":200,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"OPTIONS","path":"/b","status":200,"latency_ms":20}`,
		`{"timestamp":"2026-05-08T14:00:03Z","method":"POST","path":"/c","status":200,"latency_ms":30}`,
	}

	p := parser.New(models.DefaultFieldMap()).SetExcludeMethods([]string{"options", "head"})
	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].Method != "GET" || result.Entries[1].Method != "POST" {
		t.Errorf("expected GET and POST, got %s and %s", result.Entries[0].Method, result.Entries[1].Method)
	}
	if result.Filtered != 1 {
		t.Errorf("expected Filtered=1, got %d", result.Filtered)
	}
}

func TestParseAll_MixedAllowlistAndBlocklist(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/a","status":500,"latency_ms":10}`,
		`{"timestamp":"2026-05-08T14:00:02Z","method":"GET","path":"/b","status":502,"latency_ms":20}`,
		`{"timestamp":"2026-05-08T14:00:03Z","method":"GET","path":"/c","status":404,"latency_ms":30}`,
	}

	// Allow only 5xx, but explicitly block 502
	p := parser.New(models.DefaultFieldMap()).
		SetStatusFilter("5xx").
		SetExcludeStatuses([]string{"502"})

	result := p.ParseAll(makeChannel(lines))

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Status != 500 {
		t.Errorf("expected 500 to remain, got %d", result.Entries[0].Status)
	}
	if result.Filtered != 2 {
		t.Errorf("expected Filtered=2, got %d", result.Filtered)
	}
}
