# Core Provider Metrics Reference

This document describes the OpenTelemetry metrics emitted by `core-provider` and `provider-runtime`.

## Naming note

Metric names in code use dots. Prometheus usually normalizes them with underscores, and counters may appear with a `_total` suffix.

## Metrics

| Metric | Type | Unit | Description | Emitted from | PromQL example |
|---|---|---|---|---|---|
| `provider_runtime.startup.success` | Counter | count | Provider started successfully. | `core-provider/main.go` | `sum(increase(provider_runtime_startup_success_total[1h]))` |
| `provider_runtime.startup.failure` | Counter | count | Provider startup failed. | `core-provider/main.go` | `sum(increase(provider_runtime_startup_failure_total[1h]))` |
| `provider_runtime.reconcile.duration_seconds` | Histogram | seconds | Total reconcile duration. | `provider-runtime/pkg/telemetry/metrics.go` | `histogram_quantile(0.95, sum by (le) (rate(provider_runtime_reconcile_duration_seconds_bucket[5m])))` |
| `provider_runtime.reconcile.queue.depth` | UpDownCounter | count | Current queued requests for the controller. | `provider-runtime/pkg/controller/queue_wait.go` | `max(provider_runtime_reconcile_queue_depth)` |
| `provider_runtime.reconcile.queue.wait.duration_seconds` | Histogram | seconds | Time spent waiting in queue before processing. | `provider-runtime/pkg/controller/queue_wait.go` | `histogram_quantile(0.95, sum by (le) (rate(provider_runtime_reconcile_queue_wait_duration_seconds_bucket[5m])))` |
| `provider_runtime.reconcile.queue.oldest_item_age_seconds` | Histogram | seconds | Age of the oldest queued item observed at enqueue/dequeue time. | `provider-runtime/pkg/controller/queue_wait.go` | `histogram_quantile(0.95, sum by (le) (rate(provider_runtime_reconcile_queue_oldest_item_age_seconds_bucket[5m])))` |
| `provider_runtime.reconcile.queue.work.duration_seconds` | Histogram | seconds | Time spent processing a dequeued item before `Done()`. | `provider-runtime/pkg/controller/queue_wait.go` | `histogram_quantile(0.95, sum by (le) (rate(provider_runtime_reconcile_queue_work_duration_seconds_bucket[5m])))` |
| `provider_runtime.reconcile.queue.requeues` | Counter | count | Total queue requeues grouped by reason. | `provider-runtime/pkg/telemetry/metrics.go` | `sum(increase(provider_runtime_reconcile_queue_requeues_total[1h]))` |
| `provider_runtime.external.connect.duration_seconds` | Histogram | seconds | Time spent reading external references. | `provider-runtime/pkg/telemetry/metrics.go` | `histogram_quantile(0.95, sum by (le) (rate(provider_runtime_external_connect_duration_seconds_bucket[5m])))` |
| `provider_runtime.external.observe.duration_seconds` | Histogram | seconds | Time spent observing external resources. | `provider-runtime/pkg/telemetry/metrics.go` | `histogram_quantile(0.95, sum by (le) (rate(provider_runtime_external_observe_duration_seconds_bucket[5m])))` |
| `provider_runtime.finalizer.add.duration_seconds` | Histogram | seconds | Time spent adding finalizers. | `provider-runtime/pkg/telemetry/metrics.go` | `histogram_quantile(0.95, sum by (le) (rate(provider_runtime_finalizer_add_duration_seconds_bucket[5m])))` |
| `provider_runtime.reconcile.requeue.after` | Counter | count | Reconcile returned `RequeueAfter`. | `provider-runtime/pkg/telemetry/metrics.go` | `sum(increase(provider_runtime_reconcile_requeue_after_total[1h]))` |
| `provider_runtime.reconcile.requeue.immediate` | Counter | count | Reconcile returned immediate `Requeue`. | `provider-runtime/pkg/telemetry/metrics.go` | `sum(increase(provider_runtime_reconcile_requeue_immediate_total[1h]))` |
| `provider_runtime.reconcile.requeue.error` | Counter | count | Reconcile returned an error and will be requeued. | `provider-runtime/pkg/telemetry/metrics.go` | `sum(increase(provider_runtime_reconcile_requeue_error_total[1h]))` |
| `provider_runtime.reconcile.in_flight` | Gauge | count | Number of reconcile operations currently running. | `provider-runtime/pkg/telemetry/metrics.go` | `max(provider_runtime_reconcile_in_flight)` |

## Notes

- The manager metrics endpoint on `:8080` still exposes controller-runtime defaults.
- The custom provider-runtime metrics are exported via OTLP when `--otel-enabled` is set.
- Avoid high-cardinality labels for queue metrics.
