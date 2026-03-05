# How to: Delete Safely

> **Concepts:** [Lifecycle Workflow](../concepts.md#lifecycle-workflow) · [Automatic Cleanup](../concepts.md#security-by-design)

Deleting a CompositionDefinition triggers a cascaded cleanup: the CRD version, all associated Compositions, all Helm releases, and all RBAC resources are removed in order. Do not bypass this process by force-deleting the CRD.

---

## Delete a CompositionDefinition

```bash
kubectl delete compositiondefinition lifecycleapp-cd-v1 \
  -n cheatsheet-system
```

**What happens:**
1. A `deletionTimestamp` is set on the CRD and on all Compositions associated with it.
2. The CDC reconciles the deletion of each Helm release (this may take several minutes).
3. Once all releases are removed, the Compositions are deleted.
4. The Core Provider removes the CDC deployment, RBAC resources, and CRD version.

> This command may take a few minutes to complete. You can watch progress with:
> ```bash
> kubectl get compositiondefinition lifecycleapp-cd-v1 -n cheatsheet-system -w
> ```

---

## Delete an individual Composition

```bash
kubectl delete githubscaffoldinglifecycle lifecycle-composition-1 \
  -n cheatsheet-system
```

The CDC manages the deletion of the associated Helm release before removing the resource.

---

## If deletion is stuck

**Do not remove the finalizer on the CRD.** Removing a CRD finalizer while etcd still holds objects of that type can corrupt etcd state.

Instead:

1. Check the CDC logs for the cause of the stuck deletion:
   ```bash
   kubectl logs -n cheatsheet-system \
     -l app.kubernetes.io/name=githubscaffoldinglifecycles-v0-0-1-controller
   ```

2. If a specific Composition is stuck, you can safely remove **its** finalizer (not the CRD's) to unblock deletion:
   ```bash
   kubectl patch githubscaffoldinglifecycle <name> \
     -n cheatsheet-system \
     --type=merge \
     -p '{"metadata":{"finalizers":null}}'
   ```
   The Core Provider will then safely proceed with CRD cleanup.

3. See [Troubleshooting](../troubleshooting.md) for additional error patterns.

---

## Orphan a Helm release (prevent deletion on Composition delete)

If you want to delete a Composition without deleting its Helm release:

```bash
kubectl annotate githubscaffoldinglifecycle lifecycle-composition-1 \
  -n cheatsheet-system \
  krateo.io/management-policy=orphan
```

Then delete the Composition normally. The Helm release remains in the cluster.
