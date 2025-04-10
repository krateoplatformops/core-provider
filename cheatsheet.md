# Comprehensive Guide to Krateo Core Provider with Detailed Examples

- [Comprehensive Guide to Krateo Core Provider with Detailed Examples](#comprehensive-guide-to-krateo-core-provider-with-detailed-examples)
  - [Introduction](#introduction)
  - [Prerequisites](#prerequisites)
  - [Core Concepts](#core-concepts)
  - [Practical Examples](#practical-examples)
    - [Example 1: Basic Composition Installation](#example-1-basic-composition-installation)
    - [Example 2: Managing Multiple Versions](#example-2-managing-multiple-versions)
    - [Example 3: Updating Compositions to a New Version](#example-3-updating-compositions-to-a-new-version)
    - [Example 4: Rollback Procedure](#example-4-rollback-procedure)
  - [Best Practices](#best-practices)
  - [Troubleshooting Guide](#troubleshooting-guide)
    - [Common Issues and Solutions](#common-issues-and-solutions)


## Introduction

The Krateo Core Provider is a sophisticated system for managing Krateo PlatformOps Compositions. This guide provides in-depth explanations, detailed examples with expected results, and practical insights for effectively using the Core Provider.

## Prerequisites

### 1. Installing Krateo

**Installation Command:**
```bash
helm repo add krateo https://charts.krateo.io
helm repo update krateo
helm upgrade installer installer \
  --repo https://charts.krateo.io \
  --namespace krateo-system \
  --create-namespace \
  --install \
  --version 2.4.1 \
  --wait
```

**Verification Command:**
```bash
kubectl wait krateoplatformops krateo --for condition=Ready=True --namespace krateo-system --timeout=660s
```

**Expected Results:**
- Krateo platform installed in the `krateo-system` namespace
- All core components running and ready
- Visible pods in the `krateo-system` namespace (verify with `kubectl get pods -n krateo-system`)

## Core Concepts

### 1. CompositionDefinition Overview

A CompositionDefinition is a custom resource that specifies:
- The Helm chart to use (including location and version)
- How to generate the Custom Resource Definition (CRD)
- The controller that will manage composition instances

**Key Components:**
- `spec.chart`: Defines Helm chart details
- `metadata.annotations`: Supports special instructions like `krateo.io/connector-verbose`
- Generated CRD: Created from the chart's values.schema.json

### 2. Composition Lifecycle

A Composition represents a deployed instance featuring:
- CRD based on the schema
- Managed resources tracked in the status
- Conditions indicating readiness state

## Practical Examples

### Example 1: Basic Composition Installation

#### Step 1: Create Namespace
```bash
kubectl create namespace fireworksapp-system
```

**Verification:**
```bash
kubectl get ns fireworksapp-system
```

#### Step 2: Apply CompositionDefinition
```bash
cat <<EOF | kubectl apply -f -
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: fireworksapp-cd
  namespace: fireworksapp-system
spec:
  chart:
    repo: fireworks-app
    url: https://charts.krateo.io
    version: 1.1.13
EOF
```

**Verification Commands:**
```bash
kubectl get compositiondefinition -n fireworksapp-system
kubectl get crd fireworksapps.composition.krateo.io
kubectl get deployments -n fireworksapp-system
```

#### Step 3: Verify Readiness
```bash
kubectl wait compositiondefinition fireworksapp-cd \
  --for condition=Ready=True \
  --namespace fireworksapp-system \
  --timeout=600s
```

### Example 2: Managing Multiple Versions

#### Scenario: Running v1.1.13 and v1.1.14 simultaneously

#### Step 1: Create Second CompositionDefinition
```bash
cat <<EOF | kubectl apply -f -
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: fireworksapp-cd-v2
  namespace: fireworksapp-system
spec:
  chart:
    repo: fireworks-app
    url: https://charts.krateo.io
    version: 1.1.14
EOF
```

**Verification:**
```bash
kubectl get crd fireworksapps.composition.krateo.io -o yaml
kubectl get deployments -n fireworksapp-system
```

### Example 3: Updating Compositions to a New Version

#### Scenario: 
Safely upgrading all existing compositions from v1.1.13 to v1.1.14 while maintaining service continuity.

#### Step 1: Verify Current Versions
```bash
helm list -n fireworksapp-system
kubectl get fireworksapp -n fireworksapp-system -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.labels}{"\n"}{end}'
```

**Expected Result:**
- Helm output shows current chart versions (e.g., 1.1.13)
- FireworksApp resources display their current version labels
- No errors in the output

#### Step 2: Update CompositionDefinition
```bash
kubectl patch compositiondefinition fireworksapp-cd \
  -n fireworksapp-system \
  --type=merge \
  -p '{"spec":{"chart":{"version":"1.1.14"}}}'
```

**Verification:**
```bash
kubectl get compositiondefinition fireworksapp-cd -n fireworksapp-system -o jsonpath='{.spec.chart.version}'
```

**Expected Result:**
- CompositionDefinition spec.chart.version field updates to 1.1.14
- New controller deployment begins provisioning
- Old controller remains active during transition

#### Step 3: Monitor Controller Transition
```bash
kubectl wait deployment fireworksapps-v1-1-14-controller \
  --namespace fireworksapp-system \
  --for condition=Available=True \
  --timeout=600s && \
kubectl get pods -n fireworksapp-system -l app.kubernetes.io/version=1.1.14
```

**Expected Result:**
- New v1.1.14 controller becomes Available
- Old v1.1.13 controller pods enter Terminating state
- No interruption to existing compositions

#### Step 4: Verify Composition Updates
```bash
watch 'kubectl get fireworksapp -n fireworksapp-system -o custom-columns=NAME:.metadata.name,VERSION:.metadata.labels.krateo\.io/composition-version,STATUS:.status.conditions[?(@.type=="Ready")].status'
```

**Expected Result:**
- All compositions gradually transition to v1.1.14
- Each maintains Ready=True status throughout
- Final state shows all on v1.1.14

#### Step 5: Confirm Resource Updates
```bash
helm list -n fireworksapp-system
kubectl get deployments,services -n fireworksapp-system --show-labels | grep 'krateo.io/composition-version'
```

**Expected Result:**
- Helm shows updated chart versions
- All managed resources reflect v1.1.14 labels
- No orphaned v1.1.13 resources remain

See [rollback section](#example-4-rollback-procedure) in case of errors.

### Example 4: Rollback Procedure

#### Scenario: Rolling back from v1.1.14 to v1.1.13 after a failed upgrade

#### Step 1: Patch CompositionDefinition
```bash
kubectl patch compositiondefinition fireworksapp-cd \
  -n fireworksapp-system \
  --type=merge \
  -p '{"spec":{"chart":{"version":"1.1.13"}}}'
```

**Verification:**
```bash
kubectl get pods -n fireworksapp-system
kubectl get fireworksapp -n fireworksapp-system -o wide
```

## Best Practices

1. **Version Management**
   - Implement clear version labeling
   - Test new versions in isolated environments before production deployment

2. **Resource Organization**
   - Use namespaces to separate environments (dev/stage/prod)
   - Apply consistent naming conventions

3. **Monitoring**
   - Configure alerts for `Ready=False` conditions
   - Track controller metrics and pod restarts

## Troubleshooting Guide

### Common Issues and Solutions

1. **CompositionDefinition Not Ready**
   ```bash
   kubectl describe compositiondefinition <name> -n <namespace>
   kubectl logs -n krateo-system -l app=core-provider
   ```

2. **Stuck Composition**
   ```bash
   kubectl describe fireworksapp <name> -n <namespace>
   kubectl logs -n <namespace> <controller-pod>
   ```

3. **Failed Rollbacks**
   ```bash
   helm history <release> -n <namespace>
   kubectl get events -n <namespace>
   ```

This guide provides comprehensive coverage of Krateo Core Provider operations with:
- Clear, actionable commands
- Detailed verification steps
- Expected results for each operation
- Structured troubleshooting procedures

The improved version features:
1. Consistent command formatting
2. Clear section organization
3. Proper grammar and syntax
4. Logical flow between concepts
5. Concise yet comprehensive explanations
6. Standardized verification procedures
7. Enhanced readability through formatting