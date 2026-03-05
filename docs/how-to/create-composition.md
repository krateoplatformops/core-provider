# How to: Create a Composition

> **Concepts:** [Composition](../concepts.md#glossary) · [CDC](../concepts.md#glossary)

A Composition is a live instance of a service defined by a CompositionDefinition. Creating one triggers the installation of the associated Helm chart.

---

## Prerequisites

- CompositionDefinition deployed and `Ready=True` — see [Deploy a CompositionDefinition](deploy-composition-definition.md)
- GitHub credentials secret (for the example chart used here)

---

## 1. Create the credentials secret

```bash
kubectl create secret generic github-repo-creds \
  --namespace krateo-system \
  --from-literal=token=YOUR_GITHUB_TOKEN
```

---

## 2. Create the Composition

The `apiVersion` reflects the chart version managed by your CompositionDefinition (e.g., `composition.krateo.io/v0-0-1` for chart version `0.0.1`).

```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v0-0-1
kind: GithubScaffoldingLifecycle
metadata:
  name: lifecycle-composition-1
  namespace: cheatsheet-system
spec:
  argocd:
    namespace: krateo-system
    application:
      project: default
      source:
        path: chart/
      destination:
        server: https://kubernetes.default.svc
        namespace: githubscaffolding-app
      syncEnabled: false
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
  app:
    service:
      type: NodePort
      port: 31180
  git:
    unsupportedCapabilities: true
    insecure: true
    fromRepo:
      scmUrl: https://github.com
      org: krateoplatformops-blueprints
      name: github-scaffolding-lifecycle
      branch: main
      path: skeleton/
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
    toRepo:
      scmUrl: https://github.com
      org: your-github-org        # replace with your GitHub org
      name: lifecycleapp-test-1   # customize the repository name
      branch: main
      path: /
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
      private: false
      initialize: true
      deletionPolicy: Delete
      verbose: false
      configurationRef:
        name: repo-config
        namespace: demo-system
EOF
```

---

## 3. Wait for the Composition to become ready

```bash
kubectl wait githubscaffoldinglifecycles lifecycle-composition-1 \
  --for condition=Ready=True \
  --timeout=300s \
  --namespace cheatsheet-system
```

---

## 4. Verify the Helm release

```bash
helm list -n cheatsheet-system
```

**Expected:** A release named `lifecycle-composition-1-<UUID>` at the chart version you specified (e.g., `0.0.1`).

---

## Next steps

- [Full Migration](full-migration.md) — upgrade all Compositions to a new chart version
- [Parallel Versioning](parallel-versioning.md) — run a second chart version side-by-side
- [Selective Migration](selective-migration.md) — migrate individual Compositions to a new version
- [Pause / Resume](pause-resume.md) — temporarily halt reconciliation
- [Delete Safely](delete-safely.md) — remove Compositions and CompositionDefinitions cleanly
