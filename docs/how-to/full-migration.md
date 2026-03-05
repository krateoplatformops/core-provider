# How to: Full Migration

> **Concepts:** [Pattern 1 — Full Migration](../concepts.md#pattern-1-full-migration)

Patch the chart version on an existing CompositionDefinition to automatically migrate **all** associated Compositions to the new version. The old CDC is removed once no Compositions reference it.

**Use this when:** The new chart version has no breaking schema changes (no new required fields, no removed fields, no type changes) and you want all instances to migrate immediately.

---

## Prerequisites

- An existing CompositionDefinition with at least one Composition — see [Deploy a CompositionDefinition](deploy-composition-definition.md) and [Create a Composition](create-composition.md)
- The new chart version's `values.schema.json` is backward compatible with all existing Compositions

---

## 1. Patch the chart version

```bash
kubectl patch compositiondefinition lifecycleapp-cd-v1 \
  -n cheatsheet-system \
  --type=merge \
  -p '{"spec":{"chart":{"version":"0.0.2"}}}'
```

---

## 2. Watch the rollout

The Core Provider detects the version change and starts reconciliation. Watch its logs:

```bash
kubectl logs -n krateo-system -l app=core-provider -f
```

And monitor CDC rollout:

```bash
kubectl rollout status deployment/githubscaffoldinglifecycles-v0-0-2-controller \
  -n cheatsheet-system
```

---

## 3. Verify all Compositions are on the new version

```bash
kubectl get githubscaffoldinglifecycles -A \
  -o custom-columns=\
"NAME:.metadata.name,NAMESPACE:.metadata.namespace,VERSION:.metadata.labels.krateo\.io/composition-version"
```

All instances should show `v0-0-2` in the `VERSION` column.

> **Note:** A dedicated `VERSION` column in `kubectl get` output is a planned feature and is not yet available.

---

## 4. Verify old CDC is removed

Once all Compositions have migrated, the Core Provider automatically removes the old CDC:

```bash
kubectl get pods -n cheatsheet-system
# The githubscaffoldinglifecycles-v0-0-1-controller pod should no longer exist
```

---

## 5. Verify Helm release versions

```bash
helm list -n cheatsheet-system
```

All releases should now show version `0.0.2`.

---

## If the migration stalls

Check for schema validation errors on stuck Compositions:

```bash
kubectl describe githubscaffoldinglifecycles <name> -n cheatsheet-system
```

If the new schema added required fields that existing Compositions don't have, those Compositions will fail validation and block migration. In that case, use [Parallel Versioning](parallel-versioning.md) instead.
