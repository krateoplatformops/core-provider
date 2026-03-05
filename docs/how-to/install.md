# How to: Install the Krateo Platform

> **Concepts:** [Architecture](../concepts.md#architecture) · [Glossary](../concepts.md#glossary)

This guide walks through installing the full Krateo platform from scratch, including the core-provider and the providers required to work with the GitHub Scaffolding Lifecycle chart used in the other how-to guides.

---

## Prerequisites

- A Kubernetes cluster (v1.30+ recommended)
- `helm` v3.0+
- `kubectl` configured to access your cluster
- A GitHub account and personal access token with `repo` scope

---

## 1. Add the Krateo Helm repository

```bash
helm repo add krateo https://charts.krateo.io
helm repo update krateo
```

**Expected:** `"krateo" has been added to your repositories`

---

## 2. Install the Krateo platform

```bash
helm upgrade installer-crd installer-crd \
  --repo https://charts.krateo.io \
  --namespace krateo-system \
  --create-namespace \
  --install \
  --version 2.7.0 \
  --wait

helm upgrade installer installer \
  --repo https://charts.krateo.io \
  --namespace krateo-system \
  --create-namespace \
  --install \
  --version 2.7.0 \
  --wait
```

**Expected:** Both commands complete with no errors. The `--wait` flag blocks until all pods are ready.

---

## 3. Wait for the platform to become ready

```bash
kubectl wait krateoplatformops krateo \
  --for condition=Ready=True \
  --namespace krateo-system \
  --timeout=660s
```

**Expected:** `krateoplatformops.installer.krateo.io/krateo condition met`

---

## 4. Install required providers

```bash
helm repo add marketplace https://marketplace.krateo.io
helm repo update marketplace

helm install github-provider-kog-repo marketplace/github-provider-kog-repo \
  --namespace krateo-system --create-namespace --wait --version 1.0.0

helm install git-provider krateo/git-provider \
  --namespace krateo-system --create-namespace --wait --version 0.10.1

helm repo add argo https://argoproj.github.io/argo-helm
helm repo update argo

helm install argocd argo/argo-cd \
  --namespace krateo-system --create-namespace --wait --version 8.0.17
```

**Expected:** All three providers are installed and running in `krateo-system`.

---

## Next steps

- [Deploy a CompositionDefinition](deploy-composition-definition.md)
