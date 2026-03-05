# Version Management Enhancement for CompositionDefinition

## Problem

Currently, when you update the chart version in a CompositionDefinition, **all compositions are automatically upgraded** to the new version. To run multiple versions in parallel, you must create separate CompositionDefinition resources. This creates several challenges:

- **No granular control**: Cannot upgrade individual compositions on-demand within a single CompositionDefinition
- **Resource explosion**: Managing 10+ versions per day requires separate CompositionDefinitions (300+ CDC pods for 30 days of releases)
- **Operational overhead**: Manual cleanup of old CompositionDefinitions and their resources
- **Complex workflows**: Testing new versions requires creating temporary CompositionDefinitions

## Proposed Solution

Introduce **optional, backward-compatible** fields to enable manual upgrade control and efficient multi-version management.

### New CompositionDefinition Spec

```yaml
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: lifecycleapp-cd
  namespace: cheatsheet-system
spec:
  chart:
    repo: github-scaffolding-lifecycle
    url: https://marketplace.krateo.io
    version: "1.0.150"  # Latest version
  
  # NEW: Version management policy (optional)
  versionManagement:
    strategy: ActiveOnly  # Only deploy CDCs for versions in use + latest
    retention:
      minRetentionDays: 7    # Keep CDC for 7 days after last composition
      maxRetentionDays: 90   # Force cleanup after 90 days
  
  # NEW: Upgrade policy (optional, defaults to "Automatic" for backward compatibility)
  upgradePolicy: Manual  # Automatic | Manual | Paused
```

### Upgrading Individual Compositions

```yaml
apiVersion: core.krateo.io/githubscaffoldinglifecycles/v1-0-1
kind: GithubScaffoldingLifecycle
metadata:
  name: my-composition
  annotations:
    # Request upgrade to specific version
    krateo.io/upgrade-to-version: "1.0.150"
```

## Benefits

### 1. **Resource Efficiency**
- **Before**: 10 versions/day × 30 days = 300 CDC deployments
- **After**: Only 5-10 CDC deployments (versions actively in use)
- **Savings**: 95%+ reduction in controller pods and resource consumption

### 2. **Controlled Rollouts**
- Test new versions on development compositions first
- Gradually upgrade production compositions
- Pause upgrades during critical periods
- Rollback individual compositions if needed

### 3. **Multi-Version Support**
- Run compositions at different versions simultaneously
- Support teams with different upgrade schedules
- Maintain old versions for legacy integrations
- Automatic cleanup when versions are no longer used

### 4. **High-Velocity Friendly**
- Sustainable for environments with frequent releases
- No manual cleanup of old CompositionDefinitions
- Automatic CDC lifecycle management
- Focus on active versions only

## How It Works

### ActiveOnly Strategy

The controller automatically:
1. Deploys CDC for the **latest version** (always)
**Note**: All new fields are optional and backward compatible. Existing CompositionDefinitions work unchanged with default behavior preserved.

2. Deploys CDC for **any version with active compositions** (count > 0)
3. Cleans up CDC after grace period (7 days) when no compositions remain
4. Deploys CDC **on-demand** when composition requests upgrade

### Example Workflow

```bash
# 1. Update CompositionDefinition to new version
kubectl patch compositiondefinition lifecycleapp-cd \
  --type=merge \
  -p '{"spec":{"chart":{"version":"1.0.150"},"upgradePolicy":"Manual"}}'

# 2. Test on development composition
kubectl annotate githubscaffoldinglifecycles dev-comp \
  krateo.io/upgrade-to-version="1.0.150"

# 3. After validation, upgrade production compositions
kubectl annotate githubscaffoldinglifecycles prod-comp-1 \
  krateo.io/upgrade-to-version="1.0.150"

# 4. Old version CDC automatically cleaned up after retention period
```

### Other Notes
Having multiple versions of the CRD deployed is a problem for the kubectl CLI, which only recognizes the latest version. K8s needs to specify a version in the API request, which can lead to confusion and errors when multiple versions are present. By limiting the number of active versions, we can mitigate this issue and ensure smoother operations.

In particular we can include the version of the CR, stored in the label `krateo.io/composition-version`, in the an additional column to show when listing compositions with kubectl. This would allow users to easily identify which version of the CRD a composition is using, and avoid confusion when multiple versions are present.

To get the version of the CRD a composition is using, you first need to get the composition and then check the labels for the `krateo.io/composition-version` key. For example:

```bash
kubectl get githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  -o jsonpath='{.metadata.labels.krateo\.io/composition-version}'
```

Then you can use this information to filter compositions by version or to display the version in the output when listing compositions. For example, to get the manifest of a composition along with its version, you can use:

```bash
kubectl get githubscaffoldinglifecycles.v0-0-1.composition.krateo.io -n cheatsheet-system lifecycle-composition-1 -o yaml
```