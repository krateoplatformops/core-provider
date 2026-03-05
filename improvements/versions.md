# CompositionDefinition Version Management Enhancement Proposal

**Document Version:** 1.0  
**Date:** March 3, 2026  
**Status:** Proposed  

---

## Executive Summary

This proposal introduces non-breaking enhancements to the CompositionDefinition resource to support manual version upgrades and efficient multi-version management in high-velocity environments (10+ versions per day).

## Problem Statement

Current limitations:
1. **Forced upgrades**: Changing `spec.chart.version` automatically upgrades all compositions
2. **No version control**: Cannot maintain compositions at different versions simultaneously
3. **Scalability concerns**: Managing multiple versions requires creating separate CompositionDefinition resources
4. **Testing constraints**: Cannot test new versions on subset of compositions before full rollout

## Proposed Solution

Introduce optional, backward-compatible fields to enable:
- **Manual upgrade control** via annotations on individual compositions
- **Active-only CDC strategy** to minimize resource overhead
- **Parallel version support** for gradual migration scenarios

### API Enhancements

```yaml
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
spec:
  chart:
    version: "1.0.150"  # Latest version
  
  # NEW: Optional version management policy (default: ActiveOnly)
  versionManagement:
    strategy: ActiveOnly  # Deploy CDCs only for versions in use
    retention:
      minRetentionDays: 7
      maxRetentionDays: 90
  
  # NEW: Optional upgrade policy (default: Automatic for backward compatibility)
  upgradePolicy: Manual  # Automatic | Manual | Paused
```

### Composition Upgrade Mechanism

Users control upgrades via annotations:

```yaml
apiVersion: core.krateo.io/githubscaffoldinglifecycles/v1-0-1
kind: GithubScaffoldingLifecycle
metadata:
  annotations:
    krateo.io/upgrade-to-version: "1.0.150"  # Request upgrade
```

## Technical Approach

### 1. Vacuum Pattern Compatibility

The existing vacuum storage pattern (served=false, storage=true) enables:
- Schema validation per version at API server level
- Lossless field preservation across version changes
- Zero storage migrations required
- True API version changes without data loss

### 2. CDC Lifecycle Management (ActiveOnly Strategy)

Controller behavior:
- Deploy CDC for latest version (always)
- Deploy CDC for any version with active compositions (count > 0)
- Cleanup CDC after grace period (default: 7 days) when count = 0
- On-demand CDC deployment when upgrade requested

### 3. Enhanced Status Tracking

```go
type VersionDetail struct {
    Version      string       `json:"version"`
    Count        int          `json:"count,omitempty"`       // Active compositions
    CDCDeployed  bool         `json:"cdcDeployed,omitempty"` // Controller status
    LastUsedAt   *metav1.Time `json:"lastUsedAt,omitempty"`  // Usage tracking
}
```

## Implementation Strategy

### Phase 1: API Changes (Week 1)
- Add optional fields to CompositionDefinitionSpec
- Enhance VersionDetail status structure
- Generate updated CRD
- Default `upgradePolicy: Automatic` preserves current behavior

### Phase 2: Controller Logic (Week 2)
- Implement composition counting per version
- Add ActiveOnly CDC lifecycle management
- Implement manual upgrade processing
- Add grace period for CDC cleanup

### Phase 3: Testing & Validation (Week 3)
- Unit tests for version management logic
- Integration tests for upgrade scenarios
- Race condition testing
- Backward compatibility validation

## Benefits

1. **Resource Efficiency**: O(active_versions) instead of O(total_versions) CDC pods
2. **Controlled Rollouts**: Test new versions on subset before full deployment  
3. **Zero Breaking Changes**: Backward compatible with existing resources
4. **Multi-Version Support**: Compositions at different versions coexist safely
5. **Operational Flexibility**: Manual or automatic upgrade modes per use case

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| CDC cleanup race conditions | Medium | Grace period + finalizers + count re-check |
| Backward compatibility | Low | Optional fields, default to current behavior |
| Increased controller complexity | Medium | Phased implementation, comprehensive testing |
| On-demand CDC deployment latency | Low | Pre-deploy latest version, cache checks |

## Resource Impact (High-Velocity Scenario)

**Current approach** (10 versions/day × 30 days):
- 300 CRD versions
- 300 CDC deployments
- High etcd pressure

**Proposed approach** (ActiveOnly):
- 300 CRD versions (required for API validation)
- 5-10 CDC deployments (only active versions)
- 95%+ resource reduction

## Recommendation

**APPROVED FOR IMPLEMENTATION** with the following conditions:

1. Default `upgradePolicy: Automatic` to preserve current behavior
2. Implement comprehensive unit and integration tests
3. Add observability metrics for CDC lifecycle events
4. Document migration guide for users adopting manual upgrades
5. Feature flag for gradual production rollout

**Estimated Timeline:** 3 weeks  
**Risk Level:** Medium (acceptable with proper testing)  

---

**Prepared by:** Core Provider Engineering Team  
**Review Required:** Architecture Team, Platform Team