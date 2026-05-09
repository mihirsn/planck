package parser_test

import (
	"testing"
	"time"

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

func TestParseAll_ValidLines(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"timestamp":"2026-05-08T14:00:01Z","method":"GET","path":"/login","status":200,"latency_ms":45}`,
		`{"timestamp":"2026-05-08T14:00:03Z","method":"POST","path":"/invoice","status":201,"latency_ms":130}`,
	}

	p := parser.New()
	entries, malformed := p.ParseAll(makeChannel(lines))

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

	p := parser.New()
	entries, malformed := p.ParseAll(makeChannel(lines))

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
	p := parser.New()
	// Empty strings are invalid JSON so they are malformed.
	_, malformed := p.ParseAll(makeChannel(lines))
	if malformed != 2 {
		t.Errorf("expected 2 malformed for empty/whitespace lines, got %d", malformed)
	}
}

func TestParseAll_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want int // expected valid entries
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
			p := parser.New()
			entries, _ := p.ParseAll(makeChannel([]string{tt.line}))
			if len(entries) != tt.want {
				t.Errorf("expected %d entries, got %d", tt.want, len(entries))
			}
		})
	}
}

func TestParseAll_TimestampParsed(t *testing.T) {
	t.Parallel()

	line := `{"timestamp":"2026-05-08T14:05:00Z","path":"/invoice","status":200,"latency_ms":120}`
	p := parser.New()
	entries, _ := p.ParseAll(makeChannel([]string{line}))

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

	p := parser.New()
	entries, malformed := p.ParseAll(makeChannel([]string{}))

	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty channel, got %d", len(entries))
	}
	if malformed != 0 {
		t.Errorf("expected 0 malformed for empty channel, got %d", malformed)
	}
}
