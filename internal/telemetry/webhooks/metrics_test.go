package webhooks

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric"
	metricdata "go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNewMetricsRecordsWebhookData(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	ctx := context.Background()
	t.Cleanup(func() {
		if err := provider.Shutdown(ctx); err != nil {
			t.Fatalf("provider.Shutdown() returned error: %v", err)
		}
	})

	metrics, err := newMetrics(provider.Meter("github.com/krateoplatformops/core-provider/test"))
	if err != nil {
		t.Fatalf("newMetrics() returned error: %v", err)
	}

	metrics.RecordRequest(ctx, "mutating", "create", 125*time.Millisecond, true)
	metrics.RecordRequest(ctx, "conversion", "convert", 250*time.Millisecond, false)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("reader.Collect() returned error: %v", err)
	}

	if !hasMetric(rm, "core_provider.webhook.request.duration_seconds") {
		t.Fatal("expected webhook request duration metric to be collected")
	}
	if !hasMetric(rm, "core_provider.webhook.request.total") {
		t.Fatal("expected webhook request total metric to be collected")
	}
}

func hasMetric(rm metricdata.ResourceMetrics, name string) bool {
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == name {
				return true
			}
		}
	}

	return false
}
