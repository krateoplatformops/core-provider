# Krateo Core Provider

The [core-provider](https://github.com/krateoplatformops/core-provider) is the first function of Krateo Composable Operations (KCO) and is vital in the Krateo PlatformOps product.

## Summary

- [Architecture](#architecture)
- [Overview](#overview)
- [Examples](#examples)
- [Configuration](#configuration)

## Architecture

![core-provider Architecture Image](_diagrams/core-provider.png "core-provider Architecture")


## Overview

The product assumes that Kubernetes is the standard for IT process orchestration, bringing together business application developers and infrastructure automation developers.

Helm is the most widely used tool for composing multiple Kubernetes manifests.

However, it's important to note that Helm is not a resource natively recognized by Kubernetes. To install a Helm chart, you need to use the relevant CLI or tools configured to acknowledge Helm charts and install them on the cluster. For example, ArgoCD or FluxCD are two well-known options, but many others are available.

The `core-provider` is a Kubernetes operator that downloads a Helm chart using one of three possible methods (via tgz, Helm repo, or OCI image). It checks for the existence of values.schema.json for linting the Helm chart and uses it to generate a Custom Resource Definition in Kubernetes. This definition accurately represents the possible values that can be expressed for the installation of the chart.

However, Kubernetes is designed to validate resource inputs before applying them to the cluster, and we adhere to this hard requirement throughout Krateo development.

That's why we created the `core-provider`, which provides input validation and ensures that incorrect inputs are not even accepted in the first place.

### What is a Composition?

A Composition is an Helm Chart archive (.tgz) with a [JSON Schema](https://json-schema.org/) for the [`values.yaml`](https://helm.sh/docs/chart_template_guide/values_files/) file.

This [JSON Schema](https://json-schema.org/) file must be named: `values.schema.json`.

There are many online tools to generate automatically [JSON Schema](https://json-schema.org/) from YAML, here a few:

- https://jsonformatter.org/yaml-to-jsonschema
- https://codebeautify.org/yaml-to-json-schema-generator

Here some online tools useful to verify the [JSON Schema](https://json-schema.org/) before building the Composition:

- https://www.jsonschemavalidator.net/
- https://json-schema.hyperjump.io/

## Examples

### How to install

```sh
$ helm repo add krateo https://charts.krateo.io
$ helm repo update krateo
$ helm install krateo-core-provider krateo/core-provider 
```

### Apply the CompositionDefinition Manifest

```yaml
apiVersion: core.krateo.io/v1alpha1
kind: Definition
metadata:
  annotations:
     "krateo.io/connector-verbose": "true"
  name: postgresql
  namespace: krateo-system
spec:
  chart:
    url: oci://registry-1.docker.io/bitnamicharts/postgresql
    version: "12.8.3"
```
When the `CompositionDefinition` manifest is applied in the cluster, the `core-provider` generates the CRDs from the schema defined in the `values.schema.json` file included in the chart. It then deploys an instance of the [`composition-dynamic-controller`](https://github.com/krateoplatformops/composition-dynamic-controller), instructing it to manage resources of the type defined by the CRD. The deployment applies the most restrictive RBAC policy possible, ensuring that the `composition-dynamic-controller` instance can only manage the resources contained within the chart.

Upon deleting the CR, the `composition-dynamic-controller` instance is undeployed, and the RBAC policy is removed.

## Configuration
To view the CRD configuration visit [this link](https://doc.crds.dev/github.com/krateoplatformops/core-provider).


