# Krateo Core Provider — Concepts

This document explains the foundational concepts behind the Krateo Core Provider. Read this before consulting the how-to guides.

---

## Table of Contents

- [Glossary](#glossary)
- [Architecture](#architecture)
- [Lifecycle Workflow](#lifecycle-workflow)
- [Security by Design](#security-by-design)
- [Chart Requirements](#chart-requirements)
- [Version Management](#version-management)
  - [The Model](#the-model)
  - [Pattern 1: Full Migration](#pattern-1-full-migration)
  - [Pattern 2: Parallel Versioning](#pattern-2-parallel-versioning)
  - [Pattern 3: Selective Migration](#pattern-3-selective-migration)
  - [Tracking Versions](#tracking-versions)
  - [Multi-Version Constraints](#multi-version-constraints)
- [Best Practices](#best-practices)

---

## Glossary

- **CRD (Custom Resource Definition):** A Kubernetes resource that defines custom objects and their schemas, enabling users to extend the Kubernetes API.
- **CompositionDefinition:** A declarative Krateo resource that acts as a master blueprint for a deployable service. It consumes a Helm chart and uses it to dynamically generate a versioned CRD, effectively registering the application as a new API within the cluster.
- **Composition:** A Custom Resource representing a single live instance of a service defined by a CompositionDefinition. Its schema is generated from the `values.schema.json` of the associated Helm chart. Creating a Composition triggers the installation of the chart; its `spec` allows per-instance configuration overrides.
- **CDC (Composition Dynamic Controller):** A dedicated controller deployed by the Core Provider for each CompositionDefinition. It manages the full lifecycle (create, update, delete) of Helm releases based on Composition resources.
- **Helm Chart:** A package of pre-configured Kubernetes resources used to deploy applications.
- **OCI Registry:** A container registry supporting the Open Container Initiative image format, used for storing and distributing Helm charts.
- **RBAC Policy:** A set of rules defining Kubernetes resource permissions. Composed of Roles, RoleBindings, ClusterRoles, and ClusterRoleBindings assigned to service accounts.
- **values.schema.json:** A JSON Schema file included in Helm charts to define and validate the structure of `values.yaml`.

---

## Architecture

![core-provider Architecture](../_diagrams/core-provider.png "core-provider Architecture")

The Core Provider orchestrates three main actors:

- **Core Provider**: Watches CompositionDefinition resources. For each one it generates a CRD (from `values.schema.json`), creates RBAC policies, and deploys a CDC.
- **CDC (Composition Dynamic Controller)**: Watches Composition resources of a specific version, manages Helm releases, and requests resource metadata from the Chart Inspector.
- **Chart Inspector**: A sidecar that introspects Helm chart content to provide the CDC with the list of resources it manages, enabling precise RBAC generation.

---

## Lifecycle Workflow

![core-provider State Diagram](../_diagrams/core-provider-flow.png "core-provider State Diagram")

When a CompositionDefinition is created or updated, the Core Provider:

1. Downloads the specified Helm chart from the configured source (Helm repo, OCI registry, or TGZ URL).
2. Validates the presence of `values.schema.json`.
3. Generates or updates a versioned CRD from the schema.
4. Creates RBAC resources (ServiceAccount, Role, ClusterRole, RoleBinding, ClusterRoleBinding) scoped to the minimum permissions required by that chart.
5. Deploys a CDC instance configured to watch Compositions of that version.

When a CompositionDefinition is deleted, the reverse happens automatically: the CRD version, CDC deployment, and all RBAC resources are removed.

---

## Security by Design

- **Schema-Driven Security**: CRDs are generated from `values.schema.json`, so the Kubernetes API server validates all inputs before they reach the Core Provider, rejecting malformed or invalid configurations at the gate.
- **Principle of Least Privilege**: Each CDC is granted only the minimal RBAC permissions required to manage the resources defined in its specific chart.
- **Automatic Cleanup**: Deleting a CompositionDefinition removes the CRD, CDC, and all associated RBAC — no orphaned resources.
- **Secure Webhooks**: Mutating and conversion webhooks handle schema validation and safe multi-version API management, with TLS certificates managed and auto-rotated by the Core Provider.

---

## Chart Requirements

Every Helm chart used with the Core Provider must include a `values.schema.json` file at the chart root. This file:

- Defines the structure, types, and validation rules for `values.yaml`.
- Is used to generate the Composition CRD.
- Enables Kubernetes-native input validation.

Use [`krateoctl gen-schema`](https://github.com/krateoplatformops/krateoctl/blob/main/README.md) to generate a compliant `values.schema.json` from your `values.yaml`:

```bash
krateoctl gen-schema path/to/your/values.yaml -output path/to/output/values.schema.json
```

See the [krateoctl testdata examples](https://github.com/krateoplatformops/krateoctl/tree/main/testdata) for reference schemas.

---

## Version Management

### The Model

![CompositionDefinition Implicit Upgrade Strategy](../_diagrams/compositiondefinition-decision-tree-implicit-strategy.png "CompositionDefinition: Implicit Upgrade Strategy")

The Core Provider's versioning model has one central rule: **each chart version maps to exactly one CompositionDefinition**, which in turn maps to one CRD version and one CDC deployment.

This means there is no concept of an "in-place version bump" on a Composition. Moving a Composition to a new chart version always involves a deliberate action — either by the Platform Operator (patching the CompositionDefinition) or by the Application Owner (patching version labels on individual Compositions).

There are three distinct patterns depending on your migration goals:

| Pattern | Schema change? | Who moves? | How |
|:--------|:---------------|:-----------|:----|
| Full Migration | Non-breaking | All instances, automatically | Patch the chart version on the existing CompositionDefinition |
| Parallel Versioning | Breaking or opt-in | None (new instances only) | Create a new CompositionDefinition with a different name |
| Selective Migration | Non-breaking or breaking | Chosen instances, manually | Create new CompositionDefinition + patch version labels per Composition |

---

### Pattern 1: Full Migration

**Goal:** Move all existing Compositions to a new chart version at once.

The Platform Operator patches the `spec.chart.version` on the existing CompositionDefinition. The Core Provider detects the change, adds a new CRD version, deploys a new CDC, and reconciles all Compositions to the new version. Once all Compositions have migrated, the old CDC is automatically removed.

**When to use:** Non-breaking schema update where uniform, immediate migration is acceptable.

**Key constraint:** The new `values.schema.json` must not add required fields or remove existing ones. If it does, existing Compositions will fail schema validation. Use Parallel Versioning instead.

See: [How to: Full Migration](how-to/full-migration.md)

---

### Pattern 2: Parallel Versioning

**Goal:** Deploy a new chart version without touching any existing Compositions.

The Platform Operator creates a **new CompositionDefinition with a distinct name** pointing to the new chart version. The Core Provider registers the new version on the shared CRD and deploys a new CDC for it. The original CompositionDefinition and all its Compositions are completely unaffected.

Both versions coexist indefinitely. When the Platform Operator decides to retire the old version, deleting the old CompositionDefinition triggers automatic cleanup of its CDC and RBAC.

**When to use:** Breaking schema changes, or when separate teams own different Composition instances and cannot be migrated on a shared schedule.

**Key constraint:** Each active CompositionDefinition for the same chart registers a new CRD version. Keep the number of parallel versions low — see [Multi-Version Constraints](#multi-version-constraints).

See: [How to: Parallel Versioning](how-to/parallel-versioning.md)

---

### Pattern 3: Selective Migration

**Goal:** Migrate specific Compositions to a new version while others remain on the old version.

This combines patterns 1 and 2. A new CompositionDefinition is created (as in pattern 2). Individual Composition owners then patch two labels on their resource to hand it off to the new CDC:

| Label | Old value | New value |
|:------|:----------|:----------|
| `krateo.io/composition-definition-name` | `my-cd` | `my-cd-v2` |
| `krateo.io/composition-version` | `v1-0-0` | `v1-0-1` |

Once patched, the new CDC takes immediate ownership. For a period of time, both versions coexist. When all instances are migrated, the Platform Operator deletes the old CompositionDefinition, triggering CDC and RBAC cleanup.

**When to use:** Gradual rollout, canary testing, or when different teams migrate on their own schedule.

See: [How to: Selective Migration](how-to/selective-migration.md)


---

### Multi-Version Constraints

> **Important:** Kubernetes requires a specific API version in every request. When multiple CRD versions are registered simultaneously, `kubectl` defaults to the first stored version. Addressing a resource at an older version requires explicitly specifying the `apiVersion` in the resource manifest or API call.
>
> To limit operational complexity, keep the number of concurrently active CompositionDefinitions for the same chart as low as possible. Retire old CompositionDefinitions promptly after migration is complete.

---

## Best Practices

- **Always define a schema**: Every chart must include a comprehensive `values.schema.json`. This is the foundation of reliable Composition management.
- **Use `krateo.io/paused: "true"`** to halt reconciliation on a Composition during debugging or maintenance without deleting it.
- **Choose the right pattern before acting**: Review the pattern table above before patching a CompositionDefinition. A wrong choice (e.g., full migration when there are breaking changes) will put all Compositions into a failed state.
- **Keep parallel versions minimal**: Every active CompositionDefinition for the same chart adds a CRD version. More versions = more `kubectl` confusion. Retire them promptly.
- **Audit before retiring**: Before deleting an old CompositionDefinition, confirm zero Compositions remain on its version:
  ```bash
  kubectl get <kind> -A -l krateo.io/composition-version=<old-version>
  ```
