package webhooks

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/krateoplatformops/core-provider"

// Metrics captures low-cardinality webhook telemetry for core-provider.
type Metrics struct {
	requestDuration metric.Float64Histogram
	requestTotal    metric.Int64Counter
}

// NewMetrics creates the webhook metric instruments.
func NewMetrics() (*Metrics, error) {
	return newMetrics(otel.Meter(meterName))
}

func newMetrics(meter metric.Meter) (*Metrics, error) {
	var err error
	m := &Metrics{}

	if m.requestDuration, err = meter.Float64Histogram("core_provider.webhook.request.duration_seconds"); err != nil {
		return nil, err
	}
	if m.requestTotal, err = meter.Int64Counter("core_provider.webhook.request.total"); err != nil {
		return nil, err
	}

	return m, nil
}

// RecordRequest captures a single webhook request outcome and latency.
func (m *Metrics) RecordRequest(ctx context.Context, webhook string, operation string, d time.Duration, success bool) {
	if m == nil {
		return
	}

	outcome := "success"
	if !success {
		outcome = "error"
	}

	labels := []attribute.KeyValue{
		attribute.String("webhook", webhook),
		attribute.String("operation", operation),
	}

	m.requestDuration.Record(ctx, d.Seconds(), metric.WithAttributes(labels...))
	m.requestTotal.Add(ctx, 1, metric.WithAttributes(append(labels, attribute.String("outcome", outcome))...))
}
