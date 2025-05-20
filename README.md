# Krateo Core Provider

The Krateo Core Provider is the foundational component of Krateo Composable Operations (KCO), enabling the management of Helm charts as Kubernetes-native resources. It provides:

- Schema validation through JSON Schema
- Automated CRD generation
- Versioned composition management
- Secure authentication mechanisms

## Summary

- [Summary](#summary)
- [Glossary](#glossary)
- [Architecture](#architecture)
- [Workflow](#workflow)
- [Requirements](#requirements)
- [CompositionDefinition specifications and examples](#compositiondefinition-specifications-and-examples)
  - [Authentication](#authentication)
    - [OCI Registry](#oci-registry)
      - [GCP Artifact Registry](#gcp-artifact-registry)
    - [Helm Repository](#helm-repository)
  - [Composition Definition](#composition-definition)
    - [CRD Specification](#crd-specification)
- [How to Install](#how-to-install)
- [Examples and Troubleshooting](#examples-and-troubleshooting)
- [Environment Variables and Flags](#environment-variables-and-flags)
- [Security Features](#security-features)
- [Best Practices](#best-practices)


## Glossary

- **CRD (Custom Resource Definition):** A Kubernetes resource that defines custom objects and their schemas, enabling users to extend Kubernetes functionality.
- **CompositionDefinition:** A custom resource in the `core-provider` that defines how Helm charts are managed and deployed in Kubernetes.
- **CDC (Composition Dynamic Controller):** A controller deployed by the `core-provider` to manage resources defined by a `CompositionDefinition`. This controller is responsible to create, update, and delete helm releases and their associated resources based on the values defined in the `composition`
- **Helm Chart:** A package of pre-configured Kubernetes resources used to deploy applications.
- **OCI Registry:** A container registry that supports the Open Container Initiative (OCI) image format, used for storing and distributing Helm charts.
- **RBAC Policy:** A set of rules that define permissions for accessing Kubernetes resources. Typically composed of roles, role bindings, cluster roles, and cluster role bindings assigned to service accounts.
- **values.schema.json:** A JSON Schema file included in Helm charts to define and validate the structure of `values.yaml`.

## Architecture

![core-provider Architecture Image](_diagrams/core-provider.png "core-provider Architecture")

This diagram outlines the high-level architecture and interactions within the Core Provider, responsible for managing CompositionDefinitions and related resources. It illustrates the relationships between key components such as the Core Provider itself, the Composition Dynamic Controller (CDC), the Chart Inspector, and various Kubernetes resources.

The Core Provider generates CRDs, creates RBAC policies, and deploys the CDC. The CDC manages Helm chart releases, requests resource information from the Chart Inspector, and generates RBAC policies based on those resources. The diagram highlights the flow of definitions and resources between these components, showcasing how the Core Provider orchestrates the deployment and management of composed applications within a Kubernetes cluster.

## Workflow

![core-provider State Diagram](_diagrams/core-provider-flow.png "core-provider State Diagram")

This diagram illustrates the Core Provider's workflow for managing CompositionDefinitions, which define how resources are composed and managed in a Kubernetes environment. It encompasses the lifecycle of Helm releases and associated resources, involving the creation and updating of CRDs (Custom Resource Definitions), RBAC (Role-Based Access Control), and CDC (Composition Dynamic Controller) deployments. These actions are conditional, based on chart versions and the current state of the cluster.

**Note:** The `kubectl` commands within the notes serve as illustrative examples of the operations performed by the Core Provider and are not intended for direct user execution. They provide insights into the resource management processes undertaken by the system.

## Requirements

The core-provider does not requires Snowplow anymore to be installed in the cluster. The core-provider is now able to retrieve resource plurals from the cluster without Snowplow. This change was introduced in version 0.24.2 of the core-provider.

## CompositionDefinition specifications and examples

The `core-provider` is a Kubernetes operator that downloads and manages Helm charts. It checks for the existence of `values.schema.json` and uses it to generate a Custom Resource Definition (CRD) in Kubernetes, accurately representing the possible values that can be expressed for the installation of the chart.

Kubernetes is designed to validate resource inputs before applying them to the cluster, and the `core-provider` provides input validation to ensure that incorrect inputs are not accepted.

### Authentication

The `core-provider` also handles authentication to private OCI registries and Helm repositories. Users can provide their credentials through Kubernetes secrets, and the `core-provider` will use them to download the necessary chart resources.

#### OCI Registry

Create the Kubernetes secret:

```bash
kubectl create secret generic docker-hub --from-literal=token=your_token -n krateo-system
```

```yaml
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  annotations:
     "krateo.io/connector-verbose": "true"
  name: fireworks-private
  namespace: krateo-system
spec:
  chart:
    url: oci://registry-1.docker.io/yourusername/fireworks-app
    version: "0.1.0"
    credentials:
      username: yourusername
      passwordRef:
        key: token
        name: docker-hub
        namespace: krateo-system
```

##### GCP Artifact Registry

Follow [this guide](https://cloud.google.com/iam/docs/keys-create-delete) to create a service account key. You will need to download the .json file containing the key and create the Kubernetes secret from it. Note that the service account should have permissions to download from the Google Artifact Registry.

Now, create a secret from the JSON file containing your service account key:

```bash
kubectl create secret generic gcp-sa-secret -n demo \
 --from-file=secret-access-credentials=/path/to/file/krateoregistry-3d546566ae4a.json
```

```yaml
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: fireworks-private
  namespace: krateo-system
spec:
  chart:
    url: oci://europe-west12-docker.pkg.dev/krateoregistry/krateotest/fireworks-app
    version: "0.0.1"
    credentials:
      username: json_key
      passwordRef: # reference to a secret
        key: secret-access-credentials
        name: gcp-sa-secret
        namespace: demo
```

**Note:** The `spec.chart.credentials.username` should be set to `json_key` as explained in [this documentation](https://cloud.google.com/artifact-registry/docs/helm/authentication#linux-macos_1).

#### Helm Repository

```bash
kubectl create secret generic helm-repo --from-literal=token=your_token -n krateo-system
```

```yaml
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  annotations:
     "krateo.io/connector-verbose": "true"
  name: fireworks-private
  namespace: krateo-system
spec:
  chart:
    repo: fireworks-app
    url: https://theurltoyourhelmrepo
    version: 0.3.0
    credentials:
      username: yourusername
      passwordRef:
        key: token
        name: helm-repo
        namespace: krateo-system
```

### Composition Definition

A Composition is a Helm Chart archive (.tgz) with a JSON Schema for the `values.yaml` file. The JSON Schema file must be named `values.schema.json`.

Here are some online tools to generate and validate JSON Schemas:

- https://jsonformatter.org/yaml-to-jsonschema
- https://codebeautify.org/yaml-to-json-schema-generator
- https://www.jsonschemavalidator.net/
- https://json-schema.hyperjump.io/

#### CRD Specification

To view the CRD configuration, visit [this link](https://doc.crds.dev/github.com/krateoplatformops/core-provider).

## How to Install

You can install the `core-provider` with the following commands:

```sh
helm repo add krateo https://charts.krateo.io
helm repo update
helm install krateo-core-provider krateo/core-provider --namespace krateo-system --create-namespace
```

## Examples and Troubleshooting

You can see a more practical guide on `core-provider` usage at [this link](cheatsheet.md).

## Environment Variables and Flags

| Name                                   | Description                | Default Value | Notes         |
|:---------------------------------------|:---------------------------|:--------------|:--------------|
| `HELM_REGISTRY_CONFIG_PATH`           | Path to Helm registry configuration file | `/tmp/.config/helm/registry/config.json` | Used for OCI registries |
| `CORE_PROVIDER_DEBUG`                 | Enables debug logging      | `false`       | Use `--debug` flag |
| `CORE_PROVIDER_SYNC`                  | Sync period for controller manager | `1h`          | Duration |
| `CORE_PROVIDER_POLL_INTERVAL`         | Poll interval for resource drift checks | `5m`          | Duration |
| `CORE_PROVIDER_MAX_RECONCILE_RATE`    | Maximum reconcile rate per second | `3`           | Integer |
| `CORE_PROVIDER_LEADER_ELECTION`       | Enables leader election for controller manager | `false`      | Use `--leader-election` flag |
| `CORE_PROVIDER_MAX_ERROR_RETRY_INTERVAL` | Maximum retry interval on errors | `1m`          | Duration |
| `CORE_PROVIDER_MIN_ERROR_RETRY_INTERVAL` | Minimum retry interval on errors | `1s`          | Duration |
| `URL_PLURALS`                          | NOT USED from version 0.24.2 - URL to krateo pluraliser service | `http://snowplow.krateo-system.svc.cluster.local:8081/api-info/names` | String |

## Security Features

- Generates CRDs based on the chart's schema, preventing invalid configurations
- Deploys `composition-dynamic-controller` with minimal necessary permissions
- Removes RBAC policies upon deletion of the CR
- Implements mutating webhook and conversion webhook for enhanced security and flexibility

## Best Practices

1. Always include a `values.schema.json` file in your Helm charts
2. Use the `krateo.io/paused` annotation to manage composition lifecycle
3. Leverage the multi-version support for smooth upgrades and rollbacks

By implementing these improvements and best practices, the Krateo Core Provider offers enhanced flexibility, security, and version management capabilities for Kubernetes-based applications.