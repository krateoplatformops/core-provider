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
  # ... spec values conforming to the new schema ...
EOF
```

---

## 4. (Optional) Migrate existing Compositions with breaking schema changes

If you want to move an existing Composition to the new version (with manual schema adaptation):

### Step 1: Pause the old Composition

```bash
kubectl annotate githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  "krateo.io/paused=true"
```

### Step 2: Get the Helm release name of the old Composition

```bash
kubectl get githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  -o jsonpath='{.metadata.labels.krateo\.io/release-name}'
```

### Step 3: Create a new Composition at the new version, reusing the release name

Set `krateo.io/release-name` to the value retrieved above so the new Composition takes over the existing Helm release:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v0-0-2
kind: GithubScaffoldingLifecycle
metadata:
  name: lifecycle-composition-1-v2
  namespace: cheatsheet-system
  labels:
    krateo.io/release-name: <RELEASE_NAME_FROM_OLD_COMPOSITION>
spec:
  # ... spec values updated to match the new schema ...
EOF
```

### Step 4: Orphan and delete the old Composition

Prevent the old Composition's deletion from removing the Helm release:

```bash
kubectl annotate githubscaffoldinglifecycle lifecycle-composition-1 \
  -n cheatsheet-system \
  krateo.io/management-policy=orphan

kubectl patch githubscaffoldinglifecycle lifecycle-composition-1 \
  -n cheatsheet-system \
  --type=merge \
  -p '{"metadata":{"finalizers":null}}'

kubectl delete githubscaffoldinglifecycle lifecycle-composition-1 \
  -n cheatsheet-system
```

---

## 5. Retire the old CompositionDefinition (when ready)

Once all Compositions have been migrated or replaced, delete the old CompositionDefinition:

```bash
# Confirm nothing is left on the old version
kubectl get githubscaffoldinglifecycles -A -l krateo.io/composition-version=v0-0-1

# Delete
kubectl delete compositiondefinition lifecycleapp-cd-v1 -n cheatsheet-system
```

**Expected:** The `githubscaffoldinglifecycles-v0-0-1-controller` CDC and all its RBAC resources are automatically removed. The `v0-0-1` version entry is removed from the shared CRD.

> **Note on CRD versions:** While both CompositionDefinitions are active, two CRD versions (`v0-0-1` and `v0-0-2`) are registered. `kubectl` will default to the first stored version when no version is specified. Retire the old CompositionDefinition as soon as possible. See [Multi-Version Constraints](../concepts.md#multi-version-constraints).
