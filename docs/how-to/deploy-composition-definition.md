# How to: Deploy a CompositionDefinition

> **Concepts:** [CompositionDefinition](../concepts.md#glossary) · [Chart Requirements](../concepts.md#chart-requirements) · [Lifecycle Workflow](../concepts.md#lifecycle-workflow)

A CompositionDefinition registers a Helm chart as a versioned Kubernetes API. This guide covers creating one, all supported chart sources, and authenticating against private registries.

---

## Prerequisites

- Krateo platform installed — see [Install](install.md)
- A Helm chart with a `values.schema.json` at its root — see [Chart Requirements](../concepts.md#chart-requirements)

---

## 1. Create a namespace

```bash
kubectl create namespace cheatsheet-system
```

---

## 2. Deploy the CompositionDefinition

Choose the appropriate source format for your chart.

### Helm Repository

```bash
cat <<EOF | kubectl apply -f -
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: lifecycleapp-cd-v1
  namespace: cheatsheet-system
spec:
  chart:
    repo: github-scaffolding-lifecycle
    url: https://marketplace.krateo.io
    version: 0.0.1
EOF
```

See also the [PostgreSQL Helm repo example](../../testdata/examples/compositiondefinition-postgresql-repo.yaml).

### OCI Registry

```bash
cat <<EOF | kubectl apply -f -
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: lifecycleapp-cd-v1
  namespace: cheatsheet-system
spec:
  chart:
    url: oci://registry-1.docker.io/yourusername/my-chart
    version: "0.1.0"
EOF
```

Examples: [OCI with repo field](../../testdata/examples/compositiondefinition-postgresql-oci-repo.yaml) · [OCI without repo field](../../testdata/examples/compositiondefinition-postgresql-oci-no-repo.yaml)

### TGZ Archive (direct URL)

```bash
cat <<EOF | kubectl apply -f -
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: lifecycleapp-cd-v1
  namespace: cheatsheet-system
spec:
  chart:
    url: https://example.com/charts/my-chart-0.1.0.tgz
    version: "0.1.0"
EOF
```

Example: [TGZ archive example](../../testdata/examples/compositiondefinition-postgresql-tgz.yaml)

---

## 3. Wait for it to become ready

```bash
kubectl wait compositiondefinition lifecycleapp-cd-v1 \
  --for condition=Ready=True \
  --namespace cheatsheet-system \
  --timeout=600s
```

**What happens:** The Core Provider downloads the chart, validates `values.schema.json`, generates the CRD, creates RBAC resources, and deploys the CDC. Once all steps complete, the condition flips to `Ready=True`.

---

## Authentication

If your chart source is private, add a `credentials` block to `spec.chart`.

### OCI Registry

```bash
kubectl create secret generic my-registry-secret \
  --from-literal=token=YOUR_TOKEN \
  -n cheatsheet-system
```

```yaml
spec:
  chart:
    url: oci://registry-1.docker.io/yourusername/my-chart
    version: "0.1.0"
    credentials:
      username: yourusername
      passwordRef:
        key: token
        name: my-registry-secret
        namespace: cheatsheet-system
```

### GCP Artifact Registry

Create the secret from your service account key JSON file (the service account needs Artifact Registry Reader permissions):

```bash
kubectl create secret generic gcp-sa-secret -n cheatsheet-system \
  --from-file=secret-access-credentials=/path/to/krateoregistry-key.json
```

```yaml
spec:
  chart:
    url: oci://europe-west12-docker.pkg.dev/myproject/myrepo/my-chart
    version: "0.0.1"
    credentials:
      username: json_key          # required literal value for GCP
      passwordRef:
        key: secret-access-credentials
        name: gcp-sa-secret
        namespace: cheatsheet-system
```

> The `username` must be `json_key` for GCP Artifact Registry. See the [GCP documentation](https://cloud.google.com/artifact-registry/docs/helm/authentication#linux-macos_1).

### Helm Repository

```bash
kubectl create secret generic helm-repo-secret \
  --from-literal=token=YOUR_TOKEN \
  -n cheatsheet-system
```

```yaml
spec:
  chart:
    repo: my-chart
    url: https://charts.example.com
    version: "0.3.0"
    credentials:
      username: yourusername
      passwordRef:
        key: token
        name: helm-repo-secret
        namespace: cheatsheet-system
```

---

## Next steps

- [Create a Composition](create-composition.md)
