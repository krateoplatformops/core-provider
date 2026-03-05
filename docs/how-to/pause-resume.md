# How to: Pause and Resume a Composition

> **Concepts:** [Composition](../concepts.md#glossary) · [CDC](../concepts.md#glossary)

Pausing a Composition stops the CDC from reconciling it. Use this during maintenance, debugging, or when performing manual changes you don't want overwritten.

---

## Standard Pause

### Pause

```bash
kubectl annotate githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  "krateo.io/paused=true"
```

**Expected:** The CDC stops reconciling this Composition. Existing resources are left as-is. Verify:

```bash
kubectl get githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  -o jsonpath='{.metadata.annotations}'
```

You can also watch for the absence of new reconciliation events:

```bash
kubectl get events -n cheatsheet-system \
  --sort-by='.metadata.creationTimestamp' | grep "lifecycle-composition-1"
```

### Resume

```bash
kubectl annotate githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  "krateo.io/paused-"
```

**Expected:** The annotation is removed and the CDC immediately resumes reconciliation, converging the resource to its desired state.

---

## Graceful Pause

Available from `composition-dynamic-controller` ≥ 0.19.3 (released with `core-provider-chart` ≥ 0.33.4).

A graceful pause first pauses **all resources managed by the Composition** before pausing the Composition itself. This prevents child resources from being reconciled independently during the pause window.

To use this feature, your Helm chart must be updated to support it. See the [`composition-dynamic-controller` documentation](https://github.com/krateoplatformops/composition-dynamic-controller?tab=readme-ov-file#about-the-gracefullypaused-value) for the required chart changes.

> This feature is not backward compatible with charts that haven't been updated.
