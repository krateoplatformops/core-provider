# How to: Selective Migration

> **Concepts:** [Pattern 3 — Selective Migration](../concepts.md#pattern-3-selective-migration) · [Tracking Versions](../concepts.md#tracking-versions)

Migrate individual Compositions to a new chart version by patching two version labels. Other Compositions remain on the old version until their owners choose to migrate them.

**Use this when:** You want a gradual rollout — test the new version on dev/staging Compositions before moving production ones.

---

## Prerequisites

- An existing CompositionDefinition with multiple Compositions — see [Deploy a CompositionDefinition](deploy-composition-definition.md) and [Create a Composition](create-composition.md)
- The new chart version is deployed as a second CompositionDefinition (either already exists or created in step 1 below)

---

## 1. Create the new CompositionDefinition

If it doesn't exist yet, create it with a distinct name:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: lifecycleapp-cd-v2
  namespace: cheatsheet-system
spec:
  chart:
    repo: github-scaffolding-lifecycle
    url: https://marketplace.krateo.io
    version: 0.0.2
EOF

kubectl wait compositiondefinition lifecycleapp-cd-v2 \
  --for condition=Ready=True \
  --namespace cheatsheet-system \
  --timeout=600s
```

---

## 2. Inspect current version assignments

```bash
kubectl get githubscaffoldinglifecycles -A
```

The `VERSION` column shows the current version label for each Composition.

---

## 3. Migrate a specific Composition

Patch the two version labels on the Composition you want to upgrade. Replace `composition-2` with the actual name:

```bash
kubectl label githubscaffoldinglifecycles composition-2 \
  -n cheatsheet-system \
  krateo.io/composition-definition-name=lifecycleapp-cd-v2 \
  krateo.io/composition-version=v0-0-2 \
  --overwrite
```

**What happens immediately:**
- The new CDC (`githubscaffoldinglifecycles-v0-0-2-controller`) detects the label change and takes ownership of `composition-2`
- The old CDC stops reconciling `composition-2`
- All other Compositions (e.g., `composition-1`) continue to be managed by the old CDC

---

## 4. Verify the migration

Check that the new version field appears on the migrated Composition:

```bash
kubectl get githubscaffoldinglifecycles.v0-0-2.composition.krateo.io composition-2 \
  -n cheatsheet-system -o yaml
```

Check that the old Composition is still managed by the old CDC:

```bash
kubectl get githubscaffoldinglifecycles.v0-0-1.composition.krateo.io composition-1 \
  -n cheatsheet-system -o yaml
```

To confirm which CDC is reconciling each resource, check the CDC logs:

```bash
kubectl logs -n cheatsheet-system \
  -l app.kubernetes.io/name=githubscaffoldinglifecycles-v0-0-2-controller
```

---

## 5. Repeat for remaining Compositions

Run step 3 for each Composition you want to migrate, at whatever pace suits your team.

---

## 6. Retire the old CompositionDefinition (when all are migrated)

Once no Compositions remain on `v0-0-1`:

```bash
# Confirm
kubectl get githubscaffoldinglifecycles -A -l krateo.io/composition-version=v0-0-1

# Delete
kubectl delete compositiondefinition lifecycleapp-cd-v1 -n cheatsheet-system
```

**Expected:** The `githubscaffoldinglifecycles-v0-0-1-controller` CDC and all its RBAC resources are automatically removed. The `v0-0-1` version entry is cleaned up from the shared CRD.
