package loghandler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestNewJSONHandlerLineIsIngesterCompatible asserts that every emitted line is
// a single valid JSON object carrying the fields logs-ingester relies on:
// a `timestamp` parsable as RFC3339Nano, plus `level`, `msg` and `service`.
func TestNewJSONHandlerLineIsIngesterCompatible(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(NewJSONHandler(slog.LevelInfo, &buf))

	log.Info("order processed", "traceId", "abc-123")

	out := strings.TrimRight(buf.String(), "\n")
	if strings.Contains(out, "\n") {
		t.Fatalf("expected a single line per event, got multiple:\n%s", out)
	}

	var rec map[string]any
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("emitted line is not valid JSON: %v\nline: %s", err, out)
	}

	ts, ok := rec["timestamp"].(string)
	if !ok || ts == "" {
		t.Fatalf("missing/empty `timestamp` field: %#v", rec["timestamp"])
	}
	if _, exists := rec[slog.TimeKey]; exists {
		t.Fatalf("default %q key must be renamed to `timestamp`", slog.TimeKey)
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t.Fatalf("timestamp %q is not RFC3339Nano: %v", ts, err)
	}
	if parsed.Location() != time.UTC {
		t.Errorf("timestamp should be UTC, got %s", parsed.Location())
	}
	if !strings.HasSuffix(ts, "Z") {
		t.Errorf("timestamp should carry the UTC `Z` suffix, got %q", ts)
	}

	if got := rec["service"]; got != ServiceName {
		t.Errorf("service = %v, want %q", got, ServiceName)
	}
	if got := rec["msg"]; got != "order processed" {
		t.Errorf("msg = %v, want %q", got, "order processed")
	}
	if got := rec["level"]; got != "INFO" {
		t.Errorf("level = %v, want INFO", got)
	}
	if got := rec["traceId"]; got != "abc-123" {
		t.Errorf("traceId = %v, want abc-123", got)
	}
}

// TestNewJSONHandlerRespectsLevel ensures the configured level filters records.
func TestNewJSONHandlerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(NewJSONHandler(slog.LevelError, &buf))

	log.Info("should be filtered out")
	if buf.Len() != 0 {
		t.Fatalf("expected no output below the configured level, got: %s", buf.String())
	}

	log.Error("kept")
	if buf.Len() == 0 {
		t.Fatal("expected error-level record to be emitted")
	}
}
