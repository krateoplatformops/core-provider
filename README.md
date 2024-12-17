# Krateo Core Provider

The [core-provider](https://github.com/krateoplatformops/core-provider) is the first component of Krateo Composable Operations (KCO) and is vital to the Krateo PlatformOps product.

## Summary

- [Krateo Core Provider](#krateo-core-provider)
  - [Summary](#summary)
  - [Architecture](#architecture)
  - [Overview](#overview)
    - [Authentication](#authentication)
    - [RBAC Generation](#rbac-generation)
    - [Multi-Version Support](#multi-version-support)
    - [Composition Definition](#composition-definition)
  - [Examples](#examples)
    - [How to Install](#how-to-install)
    - [Apply the CompositionDefinition Manifest](#apply-the-compositiondefinition-manifest)
  - [Upgrade Chart Version](#upgrade-chart-version)
    - [Update the Fireworksapp Chart in the `CompositionDefinition`](#update-the-fireworksapp-chart-in-the-compositiondefinition)
    - [Automatic Deletion of Unused `composition-dynamic-controller` Deployments](#automatic-deletion-of-unused-composition-dynamic-controller-deployments)
  - [Multi-Version Management](#multi-version-management)
  - [Authentication](#authentication-1)
    - [OCI Registry](#oci-registry)
    - [Helm Repository](#helm-repository)
    - [Configuration](#configuration)
  - [Environment Variables and Flags](#environment-variables-and-flags)
    - [`HELM_REGISTRY_CONFIG_PATH`](#helm_registry_config_path)
    - [`CORE_PROVIDER_DEBUG`](#core_provider_debug)
    - [`CORE_PROVIDER_SYNC`](#core_provider_sync)
    - [`CORE_PROVIDER_POLL_INTERVAL`](#core_provider_poll_interval)
    - [`CORE_PROVIDER_MAX_RECONCILE_RATE`](#core_provider_max_reconcile_rate)
    - [`CORE_PROVIDER_LEADER_ELECTION`](#core_provider_leader_election)
  - [Security Features](#security-features)
  - [Best Practices](#best-practices)


## Architecture

![core-provider Architecture Image](_diagrams/core-provider.png "core-provider Architecture")

Here's the updated Overview section with more details about authentication and RBAC generation:

## Overview

The `core-provider` is a Kubernetes operator that downloads and manages Helm charts. It checks for the existence of `values.schema.json` and uses it to generate a Custom Resource Definition (CRD) in Kubernetes, accurately representing the possible values that can be expressed for the installation of the chart.

Kubernetes is designed to validate resource inputs before applying them to the cluster, and the `core-provider` provides input validation to ensure that incorrect inputs are not accepted.

### Authentication

The `core-provider` also handles authentication to private OCI registries and Helm repositories. Users can provide their credentials through Kubernetes secrets, and the `core-provider` will use them to download the necessary chart resources.

### RBAC Generation

When the `CompositionDefinition` manifest is applied to the cluster, the `core-provider` not only generates the CRDs but also deploys an instance of the `composition-dynamic-controller`. This controller is instructed to manage resources of the type defined by the CRD, and the deployment applies the most restrictive RBAC policy possible to ensure that the `composition-dynamic-controller` instance can only manage the resources contained within the chart. Upon deleting the CR, the `composition-dynamic-controller` instance is undeployed, and the RBAC policy is removed.

### Multi-Version Support

The core provider supports managing multiple versions of the same chart, including different values for each version. This feature addresses the common requirement of maintaining multiple versions of a composition.

### Composition Definition

A Composition is a Helm Chart archive (.tgz) with a JSON Schema for the `values.yaml` file. The JSON Schema file must be named `values.schema.json`.

Here are some online tools to generate and validate JSON Schemas:

- https://jsonformatter.org/yaml-to-jsonschema
- https://codebeautify.org/yaml-to-json-schema-generator
- https://www.jsonschemavalidator.net/
- https://json-schema.hyperjump.io/

## Requirements

In order to work, `core-provider` needs `krateo/bff` installed in the `krateo-system` namespace. Refer to [how to install](#how-to-install).

## Examples

### How to Install

The `core-provider` requires Krateo BFF to be installed in your cluster to perform some actions. If Krateo BFF is not already installed, you can install it with the following commands (note that it should be installed in the `krateo-system` namespace):

```sh
helm repo add krateo https://charts.krateo.io
helm repo update
helm install bff krateo/bff --namespace krateo-system --create-namespace
```

After installing Krateo BFF, you can install the `core-provider` with the following commands:

```sh
helm repo add krateo https://charts.krateo.io
helm repo update
helm install krateo-core-provider krateo/core-provider --namespace krateo-system --create-namespace
```

### Apply the CompositionDefinition Manifest

```yaml
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  annotations:
     "krateo.io/connector-verbose": "true"
     "krateo.io/paused": "false"
     "krateo.io/release-name": "my-composition-release"
  name: postgresql
  namespace: krateo-system
spec:
  chart:
    url: oci://registry-1.docker.io/bitnamicharts/postgresql
    version: "12.8.3"
```

When the `CompositionDefinition` manifest is applied to the cluster, the `core-provider` generates the CRDs from the schema defined in the `values.schema.json` file included in the chart. It then deploys an instance of the [`composition-dynamic-controller`](https://github.com/krateoplatformops/composition-dynamic-controller), instructing it to manage resources of the type defined by the CRD. The deployment applies the most restrictive RBAC policy possible, ensuring that the `composition-dynamic-controller` instance can only manage the resources contained within the chart.

Upon deleting the CR, the `composition-dynamic-controller` instance is undeployed, and the RBAC policy is removed.


## Upgrade Chart Version

<details><summary>CompositionDefinition fireworksapp source Manifest</summary>

```yaml
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  annotations:
    krateo.io/connector-verbose: "true"
  name: fireworksapp-1-1-5
  namespace: fireworksapp-system
spec:
  chart:
    repo: fireworks-app
    url: https://charts.krateo.io
    version: 1.1.5
```

</details>

### Update the Fireworksapp Chart in the `CompositionDefinition`

The requirement for this step is that the `values.schem.json` cointained in the chart should not change between versions of the chart. If your `values.schem.json` go to section [Multi version Management](#multi-version-management).

1. To update them chart version of existing CompositionDefinition called fireworksapp-1-1-5 in the fireworksapp-system namespace to change the spec.chart.version to 1.1.6:
```bash
kubectl patch compositiondefinition fireworksapp-1-1-5 -n fireworksapp-system --type=merge -p '{"spec":{"chart":{"version":"1.1.6"}}}'
```

2. Wait for the `fireworksapp-1-1-5` CompositionDefinition to be in the `Ready=True` condition in the `fireworksapp-system` namespace:

```bash
kubectl wait compositiondefinition fireworksapp-1-1-5 --for condition=Ready=True --namespace fireworksapp-system
```

3. Inspect the CustomResourceDefinition `fireworksapps.composition.krateo.io` to see the added version:

```bash
kubectl get crd fireworksapps.composition.krateo.io -o yaml
```

4. Check if the `fireworksapps-v1-1-6-controller` deployment to be ready in the `fireworksapp-system` namespace:

```bash
kubectl wait deployment fireworksapps-v1-1-6-controller --namespace fireworksapp-system --for condition=Available=True
```

5. Check that the previously installed chart have the expected version: 

```bash
helm list -n fireworksapp-system
```
This procedure update the existing fireworksapp installations to use the new version `1.1.6` of the chart, since the `values.schema.json` does not change between the two versions.


### Automatic Deletion of Unused `composition-dynamic-controller` Deployments

Notice that the previously deployed instances (pods) of `composition-dynamic-controller` that were configured to manage resources of version 1.1.5 no longer exist in the cluster.

This is due to the automatic cleanup mechanism that removes older and unused deployments along with their associated RBAC resources from the cluster:

```bash
kubectl get po -n fireworksapp-system
```

This automatic cleanup helps maintain cluster cleaniness by removing outdated controller instances when they are no longer needed.

## Multi-Version Management

The core provider now supports managing multiple versions of the same composition.

To perform a version upgrade or rollback:

1. Pause the current version:
   Add the `"krateo.io/paused": "true"` annotation to the CompositionDefinition for the current version.

2. Install the new version:
   Create a new CompositionDefinition for the new version without the `krateo.io/paused` annotation or set it to `"false"`.
   Include the `"krateo.io/release-name": "my-composition-release"` annotation, using the same name as the release for the previous version. This ensures that the upgrade is performed correctly.

Note: If omitted, the CDC will use the composition's `metadata.name` as the release name.

This process allows for seamless upgrades and rollbacks between different versions of the same composition, maintaining consistency in release names and ensuring proper management of multiple versions.

## Authentication

### OCI Registry

Create the Kubernetes secret:

```bash
kubectl create secret generic docker-hub --from-literal=token=your_token -n krateo-system
```

Apply the Manifest:
<details><summary>Check the Manifest</summary>

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

</details>

### Helm Repository

```bash
kubectl create secret generic helm-repo --from-literal=token=your_token -n krateo-system
```

Apply the Manifest:

<details><summary>Check the Manifest</summary>

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

</details>

### Configuration

To view the CRD configuration, visit [this link](https://doc.crds.dev/github.com/krateoplatformops/core-provider).

## Environment Variables and Flags

### `HELM_REGISTRY_CONFIG_PATH`
- **Description**: This environment variable specifies the path to the Helm registry configuration file. It is used to configure the Helm client to interact with OCI registries.
- **Example**: `/tmp/.config/helm/registry/config.json`

### `CORE_PROVIDER_DEBUG`
- **Description**: Enables debug logging for the application.
- **Flag**: `--debug` or `-d`
- **Type**: `Bool`
- **Default**: `false`

### `CORE_PROVIDER_SYNC`
- **Description**: Specifies the sync period for the controller manager. This determines how often the controller manager will resync resources.
- **Flag**: `--sync` or `-s`
- **Type**: `Duration`
- **Default**: `1h`

### `CORE_PROVIDER_POLL_INTERVAL`
- **Description**: Controls how often an individual resource should be checked for drift from the desired state.
- **Flag**: `--poll`
- **Type**: `Duration`
- **Default**: `5m`

### `CORE_PROVIDER_MAX_RECONCILE_RATE`
- **Description**: The global maximum rate per second at which resources may be checked for drift from the desired state.
- **Flag**: `--max-reconcile-rate`
- **Type**: `Int`
- **Default**: `3`

### `CORE_PROVIDER_LEADER_ELECTION`
- **Description**: Enables leader election for the controller manager. This ensures that only one instance of the controller manager is active at a time.
- **Flag**: `--leader-election` or `-l`
- **Type**: `Bool`
- **Default**: `false`



## Security Features

- Generates CRDs based on the chart's schema, preventing invalid configurations
- Deploys `composition-dynamic-controller` with minimal necessary permissions
- Removes RBAC policies upon deletion of the CR
- Implements mutating webhook and conversion webhook for enhanced security and flexibility

## Best Practices

1. Always include a `values.schema.json` file in your Helm charts
2. Use the `krateo.io/paused` annotation to manage composition lifecycle
3. Utilize the `krateo.io/release-name` annotation for consistent naming across versions
4. Leverage the multi-version support for smooth upgrades and rollbacks

By implementing these improvements and best practices, the Krateo Core Provider offers enhanced flexibility, security, and version management capabilities for Kubernetes-based applications.