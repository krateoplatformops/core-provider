# Extending core-provider

Where to make the common changes. This is a map of the seams, not a line-by-line index.

> Before changing anything that affects what gets deployed per composition, keep this **deployment invariant** in mind: the CDC bundle templates are mounted by `core-provider-chart` at runtime, not embedded in the binary. Changing the shape of the bundle means changing both the committed template and the chart that mounts it.

## Change the reconcile behavior

The reconciler's logic lives in the four operations of the CompositionDefinition controller — `Observe`, `Create`, `Update`, `Delete`. This is where you change what the operator does when a definition appears, changes, or is removed. New per-reconcile dependencies are added where the controller is wired up at startup.

## Add a chart source or change how charts are fetched

All chart fetching funnels through a single path (the shared Helm getter), so adding a source or an option is a two-part change: extend the chart portion of the `CompositionDefinition` spec to carry the new field, and thread that field into the fetch options. Because you changed an API type, regenerate the deepcopy code and the static CRD afterward.

## Customize the CDC bundle (image, args, RBAC, ConfigMap, Service)

The bundle is rendered from **template files**, not Go literals. To change the CDC image or arguments, the inspector URL or other environment, or the RBAC the controller gets, edit the corresponding template. Remember that in production these templates are mounted by `core-provider-chart`, so any change must also land in that chart.

If you add a *new* object to the bundle, handle it in three places so drift detection stays consistent: when the bundle is rendered/applied, when it is torn down, and when it is read back for the digest comparison. Miss one and `Observe` will report the composition as perpetually out of date (or fail to clean up).

## Modify CRD generation

CRD generation owns the schema decisions (the fixed status schema, the served/storage version handling) and the apply/version logic (including conversion-webhook injection). Change it here if you need different version semantics or a different generated shape.

## Change the webhook logic

- **Mutation** handles default population and the composition-version label on create.
- **Conversion** copies metadata/spec/status verbatim. If you need real cross-version field migration, this is where it would have to be implemented — today there is none, and versions are assumed field-compatible.

## Add a metric

The webhook instruments are defined together; add a new instrument there, record it from the handler, and document it in the telemetry metrics reference. Reconcile and queue metrics come from provider-runtime — see the **provider-runtime** developer guide for those.

## Add a background loop

Implement a manager runnable and register it, mirroring the certificate refresher: an initial action on start, then work on an interval, with a per-iteration timeout and log-and-continue on errors so one bad tick doesn't kill the loop.

## Worked example: an image-override flag

Suppose you want operators to override the CDC image without editing the mounted template. The shape of the change:

1. **Add a flag/env** for the override.
2. **Thread it through** the controller's options so each reconcile can read it.
3. **Inject it as a template variable** when the Deployment is rendered, falling back to the template default when unset.
4. **Use it on the dry-run path too**, so the value the operator compares against in `Observe` matches what `Create`/`Update` actually deploy — otherwise the composition looks perpetually out of date.
5. **Update the template** in both the committed copy and `core-provider-chart`, so production mounts a template that understands the new variable.

No CRD regeneration is needed here, because no API type changed. Run the test script to validate.
