# Telemetry Assets

This folder contains ready-to-use telemetry assets for `core-provider`.

- `dashboards/core-provider-overview.dashboard.json`: Grafana dashboard with example panels
- `collector/otel-collector-config.yaml`: minimal OpenTelemetry Collector config (OTLP HTTP -> Prometheus endpoint)
- `metrics-reference.md`: metric catalog with example queries

## Prerequisites

1. Run `core-provider` with OpenTelemetry enabled:

```yaml
OTEL_ENABLED: "true"
OTEL_EXPORT_INTERVAL: "30s"
OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector.monitoring.svc.cluster.local:4318"
```

2. Make the OpenTelemetry Collector reachable from `core-provider`.
3. Make sure Prometheus scrapes the Collector Prometheus exporter.
4. Connect Grafana to Prometheus as a data source.

## Import The Dashboard

1. Open Grafana.
2. Go to `Dashboards` -> `New` -> `Import`.
3. Upload `dashboards/core-provider-overview.dashboard.json`.
4. Select your Prometheus data source.
5. Save.

## Example Panels Included

- Queue depth and in-flight reconciles
- Startup success/failure counters
- Requeue after / immediate / error totals
- Queue wait latency (p95)
- Queue work duration (p95)
- Queue oldest item age (p95)
- Reconcile duration (p95)
- External connect latency (p95)
- External observe latency (p95)
- Finalizer add latency (p95)
- Requeue totals by reason
- Reconcile success/failure rate

## Metric Naming Notes

Depending on your OTel -> Prometheus conversion rules:

- counters may appear as `<metric>_total`
- histograms usually appear as `<metric>_bucket`, `<metric>_sum`, `<metric>_count`

The dashboard uses Prometheus-style queries for the normalized metric names.
If your environment differs, edit the panel queries accordingly.

## Collector Example

Use [otelcol-values.yaml](otelcol-values.yaml) as the Helm values file.

The file already includes the Collector config, so you usually only need to adjust the image tag or the target environment.

If you need the raw Collector pipeline on its own, start from [collector/otel-collector-config.yaml](collector/otel-collector-config.yaml) and adapt it to your deployment.

Current pipeline in the example:

- Receiver: OTLP HTTP on `4318`
- Processor: `batch`
- Exporter: Prometheus endpoint on `9464`

## Deploying OpenTelemetry Collector (Helm)

You can deploy a shared Collector in-cluster with the official Helm chart.

1. Add Helm repo:

```bash
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update
```

2. Use [otelcol-values.yaml](otelcol-values.yaml) as the Helm values file.

The values file exposes these ports:

```yaml
ports:
  otlp-http:
    enabled: true
    containerPort: 4318
    servicePort: 4318
    protocol: TCP
  prom-metrics:
    enabled: true
    containerPort: 9464
    servicePort: 9464
    protocol: TCP
```

3. Install Collector:

```bash
helm upgrade --install otel-collector open-telemetry/opentelemetry-collector \
	-n monitoring --create-namespace \
	-f otelcol-values.yaml
```

4. The chart creates the ServiceMonitor automatically, so Prometheus can scrape the Collector metrics endpoint.

5. Point `core-provider` to the Collector service:

```yaml
OTEL_ENABLED: "true"
OTEL_EXPORT_INTERVAL: "30s"
OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector-opentelemetry-collector.monitoring.svc.cluster.local:4318"
```

6. Quick checks:

```bash
kubectl -n monitoring get pods
kubectl -n monitoring get svc
```

Then ensure Prometheus scrapes the Collector's `:9464` endpoint.
