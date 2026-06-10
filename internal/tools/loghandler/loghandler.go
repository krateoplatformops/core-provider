// Package loghandler provides the slog handler used across the core-provider.
//
// It emits logs as one JSON object per line on the given writer, with the time
// field rendered as `timestamp` in RFC3339Nano UTC and a persistent `service`
// attribute. This format is required by logs-ingester, which discards any line
// that is not a valid JSON object and parses `timestamp` with
// time.Parse(time.RFC3339Nano, ...). See docs/log-ingester-compatibility.md.
package loghandler

import (
	"io"
	"log/slog"
	"os"
	"time"
)

// ServiceName is emitted as the `service` attribute on every log line and is
// mapped to the service_name column by logs-ingester.
const ServiceName = "core-provider"

// NewJSONHandler returns a slog.Handler that writes one JSON object per line to
// w (defaulting to os.Stderr when nil). The standard time field is renamed to
// `timestamp` and formatted as RFC3339Nano in UTC, and every record carries the
// `service` attribute.
func NewJSONHandler(level slog.Leveler, w io.Writer) slog.Handler {
	if w == nil {
		w = os.Stderr
	}

	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Only rewrite the top-level time field, not nested attributes that
			// happen to share the key.
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.String("timestamp", a.Value.Time().UTC().Format(time.RFC3339Nano))
			}
			return a
		},
	})

	return h.WithAttrs([]slog.Attr{slog.String("service", ServiceName)})
}
