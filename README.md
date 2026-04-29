# Krateo Core Provider

The **Krateo Core Provider** is the foundation of Krateo Composable Operations (KCO). It enables the management of Helm charts as Kubernetes-native resources by automating versioned CRD generation, strict JSON schema validation, and fine-grained RBAC isolation.

## Quick Start

```sh

helm repo add krateo https://charts.krateo.io
helm repo update
helm install krateo-core-provider krateo/core-provider --namespace krateo-system --create-namespace
```

## Documentation

For detailed guides, architecture diagrams, and full reference, visit the official documentation:

👉 **[https://docs.krateo.io](https://docs.krateo.io/key-concepts/kco/core-provider/)**