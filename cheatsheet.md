## Comprehensive Deployment Guide with Expected Outcomes

- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
  - [Initial Setup](#initial-setup)
  - [Core Platform Installation](#core-platform-installation)
  - [CompositionDefinition Deployment](#compositiondefinition-deployment)
  - [Creating Compositions](#creating-compositions)
  - [Advanced Operations](#advanced-operations)
    - [1. Deploying Multiple Versions](#1-deploying-multiple-versions)
    - [2. Upgrading Compositions (massive migration)](#2-upgrading-compositions-massive-migration)
    - [3. Pausing Composition Reconciliation](#3-pausing-composition-reconciliation)
- [Troubleshooting Guide](#troubleshooting-guide)
  - [Common Issues and Diagnostic Procedures](#common-issues-and-diagnostic-procedures)
  - [General Diagnostic Tools](#general-diagnostic-tools)
  - [Common Solutions](#common-solutions)


## Introduction

The Krateo V2 Template Fireworks App provides a complete solution for deploying and managing fireworks applications on Kubernetes using Krateo's Composition system. This guide covers the entire lifecycle from initial deployment to advanced management scenarios.

## Prerequisites

Before beginning, ensure you have:
- A Kubernetes cluster (v1.20+ recommended)
- Helm installed (v3.0+)
- kubectl configured to access your cluster
- A GitHub account with repository creation permissions
- A GitHub personal access token with repo scope

### Initial Setup

#### 1. Adding Krateo Helm Repository
```bash
helm repo add krateo https://charts.krateo.io
```
**What to Expect:**
- The command will register Krateo's chart repository with your local Helm installation
- Upon success, you'll see confirmation that "krateo" was added to your repositories
- This enables you to install Krateo charts using the `krateo/` prefix

#### 2. Updating Helm Repositories
```bash
helm repo update krateo
```
**What's Happening:**
- Helm contacts the repository URL to fetch the latest chart information
- It updates the local cache of available charts and versions
- The success message indicates you now have access to the most recent charts

### Core Platform Installation

#### 3. Installing Krateo Platform
```bash
helm upgrade installer installer \
  --repo https://charts.krateo.io \
  --namespace krateo-system \
  --create-namespace \
  --install \
  --version 2.4.1 \
  --wait
```
**Expected Behavior:**
- Helm will create the krateo-system namespace if it doesn't exist
- The installer chart (version 2.4.1) will be deployed
- The --wait flag ensures command completes only when resources are ready
- Output shows deployment status and namespace information

#### 4. Verifying Platform Readiness
```bash
kubectl wait krateoplatformops krateo --for condition=Ready=True --namespace krateo-system --timeout=660s
```
**What This Does:**
- Polls the Krateo platform status until "Ready" condition is True
- Times out after 660 seconds if not ready
- Successful output means core components are operational


#### 5. Install required providers:
   ```bash
   helm install github-provider krateo/github-provider --namespace krateo-system
   helm install git-provider krateo/git-provider --namespace krateo-system
   helm install argocd argo/argo-cd --namespace krateo-system --create-namespace --wait
   ```
These installations set up the necessary providers for GitHub and ArgoCD, enabling integration with your deployment process of the fireworks application.

### CompositionDefinition Deployment

#### 1. Creating the Application Namespace
```bash
kubectl create namespace fireworksapp-system
```
**Expected Outcome:**
- Creates a dedicated namespace for your Fireworks App resources
- Isolates your application resources from other deployments
- Simple confirmation message shows creation success

#### 2. Deploying the CompositionDefinition
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
**What Happens Next:**
- Krateo processes the definition and generates a Custom Resource Definition (CRD)
- A dedicated controller pod is deployed to manage compositions
- The system prepares to accept FireworksApp custom resources

#### 3. Verifying CompositionDefinition Status
```bash
kubectl wait compositiondefinition fireworksapp-cd --for condition=Ready=True --namespace fireworksapp-system --timeout=600s
```
**System Behavior:**
- Command waits until the CompositionDefinition is fully processed
- During this time, Krateo is setting up the necessary controllers
- Success means you can now create FireworksApp instances

### Creating Compositions

#### 1. Setting Up GitHub Credentials
```bash
kubectl create secret generic github-repo-creds \
  --namespace krateo-system \
  --from-literal=token=YOUR_GITHUB_TOKEN
```
**Why This Matters:**
- Stores your GitHub token securely in the cluster
- Enables the system to interact with your repositories
- The secret will be referenced by compositions for Git operations


#### 3. Creating the FireworksApp Instance


```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v1-1-13
kind: FireworksApp
metadata:
  name: fireworksapp-composition-1
  namespace: fireworksapp-system
spec:
  app:
    service:
      port: 31180
      type: NodePort
  argocd:
    application:
      destination:
        namespace: fireworks-app
        server: https://kubernetes.default.svc
      project: default
      source:
        path: chart/
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
    namespace: krateo-system
  git:
    fromRepo:
      branch: main
      credentials:
        authMethod: generic
        secretRef:
          key: token
          name: github-repo-creds
          namespace: krateo-system
      name: krateo-v2-template-fireworksapp
      org: krateoplatformops
      path: skeleton/
      scmUrl: https://github.com
    insecure: true
    toRepo:
      apiUrl: https://api.github.com
      branch: main
      credentials:
        authMethod: generic
        secretRef:
          key: token
          name: github-repo-creds
          namespace: krateo-system
      initialize: true
      org: your-organization
      name: fireworksapp-test-v2
      path: /
      private: false
      scmUrl: https://github.com
    unsupportedCapabilities: true
EOF
```

**What Occurs:**
- Krateo creates a new FireworksApp resource
- The controller begins provisioning the application
- ArgoCD is configured to manage the deployment
- A new GitHub repository is created (if specified)

#### 4. Monitoring Composition Progress
```bash
kubectl wait fireworksapp fireworksapp-composition-1 \
  --for condition=Ready=True \
  --timeout=300s \
  --namespace fireworksapp-system
```
**Expected Workflow:**
- Command waits until all resources are provisioned
- During this time, containers are pulled and started
- Services are created and become accessible
- Success means your application is fully deployed

#### 5. Check Helm Release Status
```bash
helm list -n fireworksapp-system
```

**What to Expect:**
- Helm lists all releases in the `fireworksapp-system` namespace
- You should see the `fireworksapp-composition-1` release with version 1.1.13
- This confirms that the Helm chart was successfully deployed


### Advanced Operations

#### 1. Deploying Multiple Versions

##### Scenario:

You need to run two different versions of the same application simultaneously for testing purposes.

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
**System Response:**
- A second controller is deployed for the new version
- Both versions can operate simultaneously
- Each version maintains its own CRD and controller

This will create a new `CompositionDefinition` named `fireworksapp-cd-v2` in the `fireworksapp-system` namespace, which will manage resources of version 1.1.14 of the fireworksapp chart.
You can then deploy the new version of the chart by applying the `CompositionDefinition` manifest. The `core-provider` will add a new version to the existing CRD `fireworksapps.composition.krateo.io` and deploy a new instance of the `composition-dynamic-controller` to manage resources of version 1.1.14.
The `core-provider` will leave the previous version of the chart (1.1.13) running along with its associated `composition-dynamic-controller` instance. This allows you to run multiple versions of the same application simultaneously, each managed by its own `composition-dynamic-controller`.

##### Verifying CompositionDefinition Status
```bash
kubectl wait compositiondefinition fireworksapp-cd-v2 --for condition=Ready=True --namespace fireworksapp-system --timeout=600s
```
**System Behavior:**
- Command waits until the CompositionDefinition is fully processed
- During this time, Krateo is setting up the necessary controllers
- Success means you can now create FireworksApp instances

##### Create a New FireworksApp Instance
```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v1-1-14
kind: FireworksApp
metadata:
  name: fireworksapp-composition-2
  namespace: fireworksapp-system
spec:
  app:
    service:
      port: 31180
      type: NodePort
  argocd:
    application:
      destination:
        namespace: fireworks-app
        server: https://kubernetes.default.svc
      project: default
      source:
        path: chart/
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
    namespace: krateo-system
  git:
    fromRepo:
      branch: main
      credentials:
        authMethod: generic
        secretRef:
          key: token
          name: github-repo-creds
          namespace: krateo-system
      name: krateo-v2-template-fireworksapp
      org: krateoplatformops
      path: skeleton/
      scmUrl: https://github.com
    insecure: true
    toRepo:
      apiUrl: https://api.github.com
      branch: main
      credentials:
        authMethod: generic
        secretRef:
          key: token
          name: github-repo-creds
          namespace: krateo-system
      initialize: true
      org: your-organization
      name: fireworksapp-test-v2
      path: /
      private: false
      scmUrl: https://github.com
    unsupportedCapabilities: true
EOF
```

##### Check Helm Release Status
```bash
helm list -n fireworksapp-system
```

**What to Expect:**
- You should see both `fireworksapp-composition-1` and `fireworksapp-composition-2` listed
- Each release corresponds to its respective version
- This confirms that both versions are deployed and managed independently


#### 2. Upgrading Compositions (massive migration)

##### Scenario:
You need to upgrade the existing version of the application to a newer version (1.1.14).

```bash
kubectl patch compositiondefinition fireworksapp-cd \
  -n fireworksapp-system \
  --type=merge \
  -p '{"spec":{"chart":{"version":"1.1.14"}}}'
```
**Upgrade Process:**
- The controller gradually reconciles existing resources
- New pods are rolled out using the updated version
- The system ensures zero-downtime during transition
- All components eventually reflect the new version
- The old version is marked for cleanup

##### Automatic Deletion of Unused `composition-dynamic-controller` Deployments

Notice that the previously deployed instances (pods) of `composition-dynamic-controller` that were configured to manage resources of version 1.1.14 no longer exist in the cluster.

This is due to the automatic cleanup mechanism that removes older and unused deployments along with their associated RBAC resources from the cluster:

```bash
kubectl get po -n fireworksapp-system
```

This automatic cleanup helps maintain cluster cleaniness by removing outdated controller instances when they are no longer needed.

#### 3. Pausing Composition Reconciliation

##### Use Case:
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


Here's a more comprehensive and organized troubleshooting section with clearer explanations:

## Troubleshooting Guide

### Common Issues and Diagnostic Procedures

#### 1. CompositionDefinition Not Becoming Ready

**Symptoms:**
- CompositionDefinition remains in "Not Ready" state
- No corresponding controller pod created
- CRD not generated

**Diagnostic Steps:**
```bash
# Check CompositionDefinition status details
kubectl describe compositiondefinition <name> -n fireworksapp-system

# Examine core provider logs for processing errors
kubectl logs -n krateo-system -l app=core-provider --tail=100
```

**What to Look For:**
- In the describe output: Check "Status.Conditions" for error messages
- In core provider logs: Look for chart download or CRD generation failures
- Verify network connectivity to the chart repository

#### 2. Compositions Failing to Deploy

**Symptoms:**
- FireworksApp resource stuck in "Not Ready" state
- ArgoCD application not syncing
- Missing GitHub repository

**Diagnostic Steps:**
```bash
# Get detailed status of the composition
kubectl describe fireworksapp <name> -n fireworksapp-system

# Check controller logs for reconciliation errors
kubectl logs -n fireworksapp-system -l app.kubernetes.io/name=fireworksapps-controller

# Verify ArgoCD application status
kubectl get application -n krateo-system
```

**What to Examine:**
- Events section in describe output for resource creation failures
- Controller logs for Git operations or resource deployment errors
- ArgoCD application health and sync status

#### 3. Upgrade/Rollback Failures

**Symptoms:**
- Version transition stuck
- Mixed version resources
- Controller crash loops

**Diagnostic Steps:**
```bash
# Check rollout status of controller deployment
kubectl rollout status deployment/fireworksapps-v1-1-14-controller -n fireworksapp-system

# Verify resource versions
kubectl get all -n fireworks-app -l krateo.io/composition-version

# Compare desired vs actual state
kubectl get fireworksapp -n fireworksapp-system -o yaml
```

**Investigation Areas:**
- Resource version labels consistency
- Controller pod logs for reconciliation errors
- Helm release history for the composition

### General Diagnostic Tools

```bash
# Cluster-wide event inspection
kubectl get events -A --sort-by='.metadata.creationTimestamp' | grep -i "error\|fail"

# Cross-namespace pod status check
kubectl get pods -A -o wide | grep -E "krateo-system|fireworksapp-system"

# API resource verification
kubectl api-resources | grep fireworksapp

# Network connectivity tests
kubectl run -it --rm --restart=Never network-test \
  --image=alpine -n fireworksapp-system -- \
  ping charts.krateo.io
```

### Common Solutions

1. **Authentication Issues:**
   - Regenerate Service token with correct scopes
   - Recreate the secret with proper formatting
   - Verify network policies allow outbound connections

2. **Version Conflicts:**
   - Manually clean up orphaned resources
   - Delete and recreate the CompositionDefinition
   - Verify chart compatibility between versions

3. **Resource Constraints:**
   - Check pod resource limits and node availability
   - Review pending pods with `kubectl get pods --field-selector=status.phase=Pending`
   - Increase cluster resources if needed