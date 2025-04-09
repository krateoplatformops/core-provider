# Comprehensive Guide to Krateo Core Provider with Detailed Examples

## Introduction

Krateo Core Provider is a sophisticated system for managing Krateo PlatformOps Compositions. This expanded guide provides in-depth explanations, detailed examples with expected results, and practical insights for using the Core Provider effectively.

## Prerequisites

### 1. Installing Krateo

**Command:**
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

**Verification:**
```bash
kubectl wait krateoplatformops krateo --for condition=Ready=True --namespace krateo-system --timeout=660s
```

**Expected Result:**
- Krateo platform is installed in the `krateo-system` namespace
- All core components are running and ready
- You should see various pods in the `krateo-system` namespace when running `kubectl get pods -n krateo-system`

## Core Concepts Explained

### 1. CompositionDefinition Deep Dive

A CompositionDefinition is a custom resource that defines:
- The Helm chart to use (location and version)
- How to generate the Custom Resource Definition (CRD)
- The controller that will manage composition instances

**Key Components:**
- `spec.chart`: Defines the Helm chart details
- `metadata.annotations`: Can include special instructions like `krateo.io/connector-verbose`
- Generated CRD: Created based on the chart's values.schema.json

### 2. Composition Lifecycle

A Composition represents a deployed instance with:
- Custom resource definition based on the schema
- Managed resources tracked in the status
- Conditions showing readiness state

## Practical Examples with Detailed Explanations

### Example 1: Installing a Basic Composition

#### Step 1: Create Namespace
```bash
kubectl create namespace fireworksapp-system
```

**Expected Result:**
- New namespace `fireworksapp-system` is created
- Verify with `kubectl get ns fireworksapp-system`

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

**Expected Result:**
- CompositionDefinition resource is created
- Core provider generates:
  - Custom Resource Definition (CRD) for FireworksApp
  - Controller deployment
- Verify with:
  ```bash
  kubectl get compositiondefinition -n fireworksapp-system
  kubectl get crd fireworksapps.composition.krateo.io
  kubectl get deployments -n fireworksapp-system
  ```

#### Step 3: Wait for Readiness
```bash
kubectl wait compositiondefinition fireworksapp-cd \
  --for condition=Ready=True \
  --namespace fireworksapp-system \
  --timeout=600s
```

**Expected Result:**
- CompositionDefinition status shows `Ready: True`
- Controller deployment is available
- CRD is established

#### Step 4: Create Composition Instance
```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v1alpha1
kind: FireworksApp
metadata:
  name: fireworksapp-composition-1
  namespace: fireworksapp-system
spec:
  toRepo:
    org: your-github-org
    name: fireworks-app
EOF
```

**Expected Result:**
- New FireworksApp resource is created
- Controller begins provisioning resources
- Helm release is created (check with `helm list -n fireworksapp-system`)

#### Step 5: Verify Installation
```bash
kubectl wait fireworksapp fireworksapp-composition-1 \
  --for condition=Ready=True \
  --namespace fireworksapp-system \
  --timeout=300s
```

**Expected Result:**
- Composition status shows `Ready: True`
- All managed resources are provisioned
- GitHub repository is created (if part of the chart)

### Example 2: Managing Multiple Versions (Detailed)

#### Scenario:
You need to run two different versions of the same application simultaneously for testing purposes.

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

**Expected Result:**
- Second CompositionDefinition is created
- New CRD version is registered (verify with `kubectl get crd fireworksapps.composition.krateo.io -o yaml`)
- Second controller deployment is created (check with `kubectl get deployments -n fireworksapp-system`)

#### Step 2: Create Second Composition Instance
```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v1alpha1
kind: FireworksApp
metadata:
  name: fireworksapp-composition-v2
  namespace: fireworksapp-system
  labels:
    krateo.io/composition-version: v1-1-14
spec:
  toRepo:
    org: your-github-org
    name: fireworks-app-v2
EOF
```

**Expected Result:**
- Second instance is created with version 1.1.14
- Both versions run simultaneously in the cluster
- Verify with:
  ```bash
  helm list -n fireworksapp-system  # Should show two releases
  kubectl get fireworksapp -n fireworksapp-system  # Should show both instances
  ```

#### Step 3: Verify Isolation
```bash
kubectl get deployments -n fireworksapp-system
```

**Expected Result:**
- You should see two controller deployments:
  - `fireworksapps-v1-1-13-controller`
  - `fireworksapps-v1-1-14-controller`
- Each manages its own version of the composition

### Example 3: Pausing Composition Reconciliation (Expanded)

#### Use Case:
Temporarily stop automatic reconciliation during maintenance or troubleshooting.

#### Step 1: Pause Composition
```bash
kubectl annotate fireworksapp fireworksapp-composition-1 \
  -n fireworksapp-system \
  "krateo.io/paused=true"
```

**Expected Result:**
- Composition is annotated with `krateo.io/paused=true`
- Controller stops reconciling this instance
- Verify with:
  ```bash
  kubectl get fireworksapp fireworksapp-composition-1 -n fireworksapp-system -o jsonpath='{.metadata.annotations}'
  ```

#### Step 2: Make Manual Changes
During paused state, you can:
- Manually modify resources
- Troubleshoot issues
- Perform maintenance

#### Step 3: Resume Reconciliation
```bash
kubectl annotate fireworksapp fireworksapp-composition-1 \
  -n fireworksapp-system \
  "krateo.io/paused-"
```

**Expected Result:**
- Pause annotation is removed
- Controller resumes normal operation
- Changes are reconciled according to desired state

### Example 4: Updating a Composition (Detailed Workflow)

#### Step 1: Check Current Version
```bash
helm list -n fireworksapp-system
```

**Expected Result:**
- Shows current version (e.g., 1.1.13)

#### Step 2: Update CompositionDefinition
```bash
kubectl patch compositiondefinition fireworksapp-cd \
  -n fireworksapp-system \
  --type=merge \
  -p '{"spec":{"chart":{"version":"1.1.14"}}}'
```

**Expected Result:**
- CompositionDefinition spec is updated
- New controller deployment is created
- Old controller is terminated after graceful shutdown

#### Step 3: Verify Update
```bash
kubectl wait deployment fireworksapps-v1-1-14-controller \
  --namespace fireworksapp-system \
  --for condition=Available=True \
  --timeout=600s
```

**Expected Result:**
- New controller is running and available
- Old controller pod is terminated (verify with `kubectl get pods -n fireworksapp-system`)

#### Step 4: Check Composition Status
```bash
kubectl get fireworksapp -n fireworksapp-system -o wide
```

**Expected Result:**
- All compositions now show version 1.1.14 in their status
- Resources are updated to new version

## Advanced Operations

### 1. Inspecting Managed Resources

```bash
kubectl get fireworksapp fireworksapp-composition-1 \
  -n fireworksapp-system \
  -o jsonpath='{.status.managed}' | jq .
```

**Expected Result:**
- Detailed list of all resources managed by the composition
- Includes API versions, kinds, names, and namespaces

### 2. Accessing Controller Logs

```bash
kubectl logs -n fireworksapp-system \
  -l app.kubernetes.io/name=fireworksapps-v1-1-14-controller \
  --tail=50
```

**Expected Result:**
- Log output showing reconciliation activities
- Helpful for troubleshooting

### 3. Monitoring Composition Events

```bash
kubectl get events --sort-by='.lastTimestamp' \
  -n fireworksapp-system \
  --field-selector involvedObject.kind=FireworksApp
```

**Expected Result:**
- Chronological list of events related to compositions
- Shows creation, updates, and errors

## Best Practices (Expanded)

1. **Version Management:**
   - Use clear version labeling
   - Test new versions with isolated CompositionDefinitions before rolling out

2. **Resource Organization:**
   - Use namespaces to separate environments
   - Prefix resource names with composition version

3. **Monitoring:**
   - Set up alerts for composition `Ready` condition
   - Monitor controller pod restarts

4. **RBAC:**
   - Regularly review generated RBAC rules
   - Audit permissions with `kubectl auth can-i` commands

## Troubleshooting Guide

### Common Issues and Solutions

1. **Composition Stuck in "Not Ready" State**
   - Check controller logs
   - Verify dependent resources (secrets, configmaps)
   - Examine events for errors

2. **RBAC Permission Errors**
   - Inspect generated ClusterRoles
   - Verify ServiceAccount associations
   - Check for namespace restrictions

3. **Version Upgrade Failures**
   - Verify schema compatibility between versions
   - Check Helm release history (`helm history`)
   - Review rollback options

4. **Resource Cleanup Issues**
   - Verify finalizers on compositions
   - Check for orphaned resources
   - Review controller termination behavior

This comprehensive guide provides detailed examples with expected results for all major operations with Krateo Core Provider. The expanded explanations and troubleshooting sections will help you effectively manage compositions in production environments.