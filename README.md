# Krateo Core Provider

Manage Krateo PlatformOps Compositions.

## What is a Composition?

A Composition is an Helm Chart archive (.tgz) with a [JSON Schema](https://json-schema.org/) for the [`values.yaml`](https://helm.sh/docs/chart_template_guide/values_files/) file.

This [JSON Schema](https://json-schema.org/) file must be named: `values.schema.json`.

There are many online tools to generate automatically [JSON Schema](https://json-schema.org/) from YAML, here a few:

- https://jsonformatter.org/yaml-to-jsonschema
- https://codebeautify.org/yaml-to-json-schema-generator

Here some online tools useful to verify the [JSON Schema](https://json-schema.org/) before building the Composition:

- https://www.jsonschemavalidator.net/
- https://json-schema.hyperjump.io/

## How to install

```sh
$ helm repo add krateo https://charts.krateo.io
$ helm repo update krateo
$ helm install krateo-core-provider krateo/core-provider 
```

## Resources (specs)

## core.krateo.io/v1alpha1, Definition

A `Definition` defines a Krateo Composition Archive URL.

| Spec               | Description                                     | Required |
|:-------------------|:------------------------------------------------|:---------|
| chart.url          | krateo composition url                          | true     |
| chart.version      | krateo composition version                      | false    |
| chart.repo         | helm chart repo name (only for helm repos urls) | false    |

example:

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

