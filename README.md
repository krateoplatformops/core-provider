# Krateo Core Provider

The Krateo Core Provider is the foundational component of Krateo Composable Operations (KCO), enabling the management of Helm charts as Kubernetes-native resources.

## Key Features

- **Dynamic CRD Generation**: Automatically creates and manages versioned CRDs from a chart's `values.schema.json`.
- **Schema-Driven Validation**: Leverages JSON Schema to enforce strict input validation at the API level, preventing invalid configurations before they are applied.
- **Secure Credential Management**: Integrates with Kubernetes secrets for seamless authentication against private OCI and Helm repositories.
- **Isolated RBAC Policies**: Generates and manages fine-grained RBAC policies for each composition, ensuring controllers have the minimum necessary permissions.
- **Multi-Version Chart Support**: Supports three distinct upgrade patterns (full migration, parallel versioning, selective migration) for controlled, zero-downtime chart upgrades.


## Requirements

- Kubernetes 1.30+ (compatible with [Validating Admission Policies](https://kubernetes.io/blog/2024/04/24/validating-admission-policy-ga/))
- Helm 3.0+

> As of version 0.24.2, the core-provider no longer requires Snowplow for resource discovery — this functionality is now built-in.

## Install

```sh
helm repo add krateo https://charts.krateo.io
helm repo update
helm install krateo-core-provider krateo/core-provider --namespace krateo-system --create-namespace
```

## Documentation

| Document | Purpose |
|:---------|:--------|
| [Concepts](docs/concepts.md) | Architecture, glossary, version management model, security design |
| [Install the platform](docs/how-to/install.md) | Set up Krateo and required providers from scratch |
| [Deploy a CompositionDefinition](docs/how-to/deploy-composition-definition.md) | Register a Helm chart as a Kubernetes API, including chart sources and authentication |
| [Create a Composition](docs/how-to/create-composition.md) | Instantiate and monitor a deployed service |
| [Full Migration](docs/how-to/full-migration.md) | Upgrade all Compositions to a new chart version at once |
| [Parallel Versioning](docs/how-to/parallel-versioning.md) | Run two chart versions side-by-side without migrating existing instances |
| [Selective Migration](docs/how-to/selective-migration.md) | Migrate individual Compositions to a new version on your own schedule |
| [Pause / Resume](docs/how-to/pause-resume.md) | Temporarily halt reconciliation for maintenance or debugging |
| [Delete Safely](docs/how-to/delete-safely.md) | Remove Compositions and CompositionDefinitions without leaving orphaned resources |
| [Troubleshooting](docs/troubleshooting.md) | Diagnose and fix common issues |

## CRD Specification

Full CRD configuration: [doc.crds.dev/github.com/krateoplatformops/core-provider](https://doc.crds.dev/github.com/krateoplatformops/core-provider)

## Environment Variables and Flags

| Name | Description | Default | Notes |
|:-----|:------------|:--------|:------|
| `HELM_REGISTRY_CONFIG_PATH` | Path to Helm registry configuration file | `/tmp` | Used for OCI registries |
| `CORE_PROVIDER_DEBUG` | Enables debug logging | `false` | Use `--debug` flag |
| `CORE_PROVIDER_SYNC` | Sync period for controller manager | `1h` | Duration |
| `CORE_PROVIDER_POLL_INTERVAL` | Poll interval for resource drift checks | `5m` | Duration |
| `CORE_PROVIDER_MAX_RECONCILE_RATE` | Maximum reconcile rate per second | `5` | Integer |
| `CORE_PROVIDER_LEADER_ELECTION` | Enables leader election for controller manager | `false` | Use `--leader-election` flag |
| `CORE_PROVIDER_WEBHOOK_SERVICE_NAME` | Name of the webhook service | `core-provider-webhook-service` | String |
| `CORE_PROVIDER_WEBHOOK_SERVICE_NAMESPACE` | Namespace of the webhook service | `demo-system` | String |
| `CORE_PROVIDER_MAX_ERROR_RETRY_INTERVAL` | Maximum retry interval on errors | `1m` | Duration |
| `CORE_PROVIDER_MIN_ERROR_RETRY_INTERVAL` | Minimum retry interval on errors | `1s` | Duration |
| `CORE_PROVIDER_TLS_CERTIFICATE_DURATION` | Duration of the TLS certificate. Must be at least 10 minutes and 3× the poll interval. | `24h` | Duration |
| `CORE_PROVIDER_TLS_CERTIFICATE_LEASE_EXPIRATION_MARGIN` | Time before certificate expiry when the lease is renewed. Must be less than the certificate duration; recommended ≤ 2/3 of it. | `16h` | Duration |
| `URL_PLURALS` | **DEPRECATED** (≥ 0.24.2) — URL to krateo pluraliser service | `http://snowplow.krateo-system.svc.cluster.local:8081/api-info/names` | String |


