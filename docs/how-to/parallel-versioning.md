# How to: Parallel Versioning

> **Concepts:** [Pattern 2 — Parallel Versioning](../concepts.md#pattern-2-parallel-versioning) · [Multi-Version Constraints](../concepts.md#multi-version-constraints)

Deploy a new chart version by creating a second CompositionDefinition with a distinct name. All existing Compositions remain on the old version untouched. New Compositions are created against the new version. Both coexist until the old CompositionDefinition is explicitly deleted.

**Use this when:** The new chart version has breaking schema changes, or when different teams own Composition instances and need to migrate independently.

---

## Prerequisites

- An existing CompositionDefinition — see [Deploy a CompositionDefinition](deploy-composition-definition.md)
- The new chart version (may have breaking schema changes)

---

## 1. Create a new CompositionDefinition for the new version

Give it a **distinct name** from the existing one:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: lifecycleapp-cd-v2        # distinct name — NOT the same as lifecycleapp-cd-v1
  namespace: cheatsheet-system
spec:
  chart:
    repo: github-scaffolding-lifecycle
    url: https://marketplace.krateo.io
    version: 0.0.2
EOF
```

---

## 2. Wait for it to become ready

```bash
kubectl wait compositiondefinition lifecycleapp-cd-v2 \
  --for condition=Ready=True \
  --namespace cheatsheet-system \
  --timeout=600s
```

**What happens:** The Core Provider adds `v0-0-2` as a new version on the shared CRD and deploys a second CDC (`githubscaffoldinglifecycles-v0-0-2-controller`). The old CDC for `v0-0-1` continues running unchanged.

---

## 3. Create new Compositions against the new version

Use `apiVersion: composition.krateo.io/v0-0-2`:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v0-0-2
kind: GithubScaffoldingLifecycle
metadata:
  name: lifecycle-composition-new
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

## 4. (Optional) Migrate an existing Composition with breaking schema changes

### Step 1: Export the current manifest

```bash
kubectl get githubscaffoldinglifecycles.v0-0-1.composition.krateo.io lifecycle-composition-1 \
  -n cheatsheet-system -o yaml > composition.yaml
```

### Step 2: Edit the manifest

In `composition.yaml`, make three changes:

1. Update `apiVersion` to the new version:
   ```yaml
   apiVersion: composition.krateo.io/v0-0-2   # was v0-0-1
   ```

2. Update the version label:
   ```yaml
   labels:
     krateo.io/composition-version: v0-0-2     # was v0-0-1
   ```

3. Adapt `spec` for any breaking schema changes introduced in `0.0.2`.

### Step 3: Apply

```bash
kubectl apply -f composition.yaml
```

The new CDC (`githubscaffoldinglifecycles-v0-0-2-controller`) detects the updated label and takes ownership. The old CDC stops reconciling this Composition.

---

## 5. Retire the old CompositionDefinition (when ready)

Once all Compositions have been migrated or replaced, delete the old CompositionDefinition:

```bash
# Confirm nothing is left on the old version
kubectl get githubscaffoldinglifecycles -A -l krateo.io/composition-version=v0-0-1

# Delete
kubectl delete compositiondefinition lifecycleapp-cd-v1 -n cheatsheet-system
```

**Expected:** The `githubscaffoldinglifecycles-v0-0-1-controller` CDC and all its RBAC resources are automatically removed. 

> **Note on CRD versions:** While both CompositionDefinitions are active, two CRD versions (`v0-0-1` and `v0-0-2`) are registered. `kubectl` will default to the first stored version when no version is specified. Retire the old CompositionDefinition as soon as possible. See [Multi-Version Constraints](../concepts.md#multi-version-constraints).
