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
    - [4. Safely Deleting Compositions](#4-safely-deleting-compositions)
- [Troubleshooting Guide](#troubleshooting-guide)
  - [Common Issues and Diagnostic Procedures](#common-issues-and-diagnostic-procedures)
    - [1. CompositionDefinition Not Becoming Ready](#1-compositiondefinition-not-becoming-ready)
    - [2. Compositions Failing to Deploy](#2-compositions-failing-to-deploy)
    - [3. Upgrade/Rollback Failures](#3-upgraderollback-failures)
    - [4. Certificate Issues: Mutating Webhook Configuration](#4-certificate-issues-mutating-webhook-configuration)
    - [4.1 Certificate Issues: Conversion Webhook Configuration](#41-certificate-issues-conversion-webhook-configuration)
    - [5. Error after creating CompositionDefinition](#5-error-after-creating-compositiondefinition)
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
  --version 2.4.2 \
  --wait
```
**Expected Behavior:**
- Helm will create the krateo-system namespace if it doesn't exist
- The installer chart (version 2.4.2) will be deployed
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
You need to upgrade the existing version of the application to a newer version (1.1.14). To be sure the new version can be deployed, the `values.schema.json` should not add any new required fields or remove any existing required fields. If you need to add new required fields, you should create a new `CompositionDefinition` with the new version of the chart. Refer to [the next section](#3-upgrading-compositions-with-changes-in-the-valuesschemajson) for more details. 

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

##### Check Helm Release Status
```bash
helm list -n fireworksapp-system
```

**What to Expect:**
- You should see both `fireworksapp-composition-1` and `fireworksapp-composition-2` listed
- Each release corresponds to its respective version but now `fireworksapp-composition-1` will be updated to version 1.1.14


#### 3. Upgrading Compositions with changes in the `values.schema.json`
##### Scenario:
This can be useful for upgrading a chart where you have changed the `values.schema.json` (in particular adding required fields or modifying existing fields) and you want to ensure that the new version can be deployed without issues.

Suppose you have installed composition `fireworksapp-cd` with version 1.1.13 and you want to upgrade it to version 1.1.14, but the new version has changes in the `values.schema.json` that require you to create a new `CompositionDefinition` with the new version of the chart.

The first step is to create a new `CompositionDefinition` with the new version of the chart. You can do this by applying the following manifest as done in [the previous section](#1-deploying-multiple-versions):

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

At this point, you have two `CompositionDefinition` resources in the cluster: `fireworksapp-cd` with version 1.1.13 and `fireworksapp-cd-v2` with version 1.1.14.

At this point, suppose you need to upgrade an existing release of a composition to the new version. You can do this by following the steps below:

##### Step 1: Pause the Composition with the Old Version
```bash
kubectl annotate fireworksapp fireworksapp-composition-1 \
  -n fireworksapp-system \
  "krateo.io/paused=true"
```

This ensures that the old version of the composition is not reconciled while you are upgrading to the new version.

##### Step 2: Create a New FireworksApp Instance with the New Version

Create a new FireworksApp instance with the new version of the chart. Note that you need to add a label called `krateo.io/release-name` to the new FireworksApp instance. This label is used by the `composition-dynamic-controller` to identify the release name of the composition. The value of this label should be the same as the name of the old FireworksApp instance. In the following example, the value of the label is `fireworksapp-composition-1`, which is the name of the release installed at [step 1](#3-creating-the-fireworksapp-instance). Normally, this label is automatically added to the FireworksApp instance when it is created. However, in this case, you need to add it manually because you are creating a new FireworksApp instance with a different name.

You can do that by applying the following manifest:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v1-1-14
kind: FireworksApp
metadata:
  name: fireworksapp-composition-2
  namespace: fireworksapp-system
  labels:
    krateo.io/release-name: fireworksapp-composition-1
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

At this point, the helm release `fireworksapp-composition-1`  will be upgraded to version 1.1.14. 

##### Step 3: Remove the old composition at version 1.1.13
```bash
kubectl delete fireworksapp fireworksapp-composition-1 \
  -n fireworksapp-system
```

You also need to remove the finalizer in the old composition because the annotation `krateo.io/paused=true` will prevent the old composition from being deleted. You can do this by applying the following command:

```bash
kubectl patch fireworksapp fireworksapp-composition-1 \
  -n fireworksapp-system \
  --type=merge \
  -p '{"metadata":{"finalizers":null}}'
```

At this point, the old composition will be deleted and the new composition will be upgraded to version 1.1.14.

#### 4. Pausing Composition Reconciliation

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
  You could also check events:
  ```bash
  kubectl get events -n fireworksapp-system --sort-by='.metadata.creationTimestamp' | grep "fireworksapp-composition-1"
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

#### 4. Safely Deleting Compositions

You can safely delete all compositions and their associated resources using the following command:

```bash
kubectl delete compositiondefinition fireworksapp-cd \
  -n fireworksapp-system
```
**Expected Outcome:**
- The `deletionTimestamp` is set in the generated CRD and in all the compositions related to this CRD.
- The `compositiondefinition` and all associated resources are deleted. (It may take a few minutes because any release associated with the composition will be deleted first.)

Note: If some compositions are stuck in the deletion process, do not enforce their deletion by removing the finalizer in the CRD. Instead, you should check the logs of the `composition-dynamic-controller` to understand why the deletion is stuck. If you need to force the deletion, you can do so by removing the finalizer in the composition CR. Then the core-provider will safely delete the CRD. 

Deleting the finalizer in the CRD can cause problems with Kubernetes etcd, so it is not recommended. See section 5. Error after creating CompositionDefinition for troubleshooting.

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

#### 4. Certificate Issues: Mutating Webhook Configuration - Valid ONLY for versions of core-provider before <0.24.2>
**Note:** This issue is only valid for versions of core-provider before <0.24.2. In versions after this, the management of certificates is automatically handled by the core-provider and you should not face this issue.

**Symptoms:**
- You receive an error like `Internal error occurred: failed calling webhook "core.provider.krateo.io": failed to call webhook: Post "https://core-provider-webhook-
service.krateo-v2-system.svc:9443/mutate? timeout=10s": t|s: failed to verify certificate: x509: certificate signed by unknown authority (possibly because of "crypto/rsa: verification error" while trying to verify candidate authority certificate "core-provider-webhook-
service.krateo-v2-system.svc")`
- You cannot create the related composition in the cluster

**Diagnostic Steps:**
```bash
# Check the caBundle in the core-provider-webhook-service
kubectl get mutatingWebhookConfiguration core-provider -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 

# Check if the base64 encoded caBundle is aligned with the core-provider tls secret
kubectl get secret core-provider-certs -n krateo-system -o json | jq -r '.data["tls.crt"]'
```

If you receive the error described above, the caBundle is not aligned with the core-provider tls secret. So you need to patch the caBundle in the core-provider mutatingWebhookConfiguration.

**Remediation Steps:**
- Patch the caBundle in the core-provider mutatingWebhookConfiguration using the command provided above.
```bash
caBundle=$(kubectl get secret core-provider-certs -n krateo-system -o json | jq -r '.data["tls.crt"]')
kubectl patch mutatingWebhookConfiguration core-provider \
  --type='json' \
  -p='[{"op": "replace", "path": "/webhooks/0/clientConfig/caBundle", "value": "'"$caBundle"'"}]'
```

#### 4.1 Certificate Issues: Conversion Webhook Configuration
**Symptoms:**
- You receive an error like `conversion webhook for composition.krateo.io/v0-0-13, Kind=KrateoTemplateToolAwsGlueJobs failed: Post \"https://core-provider-webhook-service.krateo-v2-system.svc:9443/convert?timeout=30s\": tls: failed to verify certificate: x509: certificate signed by unknown authority`
- You cannot create the related composition in the cluster
  
**Diagnostic Steps:**
```bash
# Check the caBundle in the core-provider-webhook-service
kubectl get crds <your-crd-name> -o jsonpath='{.spec.conversion.webhook.clientConfig.caBundle}'
# Check if the base64 encoded caBundle is aligned with the core-provider tls secret
kubectl get secret core-provider-certs -n krateo-system -o json | jq -r '.data["tls.crt"]'
```

**Remediation Steps:**
- Patch the caBundle in the core-provider mutatingWebhookConfiguration using the command provided above.
```bash
caBundle=$(kubectl get secret core-provider-certs -n krateo-system -o json | jq -r '.data["tls.crt"]')
kubectl patch crd <your-crd-name> \
  --type='json' \
  -p='[{"op": "replace", "path": "/spec/conversion/webhook/clientConfig/caBundle", "value": "'"$caBundle"'"}]'
```
You will need to replace `<your-crd-name>` with the name of the CRD you are trying to create.

#### 5. Error after creating CompositionDefinition
**Symptoms:**
- You receive an error like `request to convert CR from an invalid group/version: composition.krateo.io/vacuum`
- You cannot create the related composition in the cluster

**Diagnostic Steps:**
```bash
# This error is usually present in the Kubernetes API server logs, in core-provider logs, and is also generally present when listing `compositions`

kubectl get compositions -A
```

**Remediation Steps:**
- To solve this issue, you need to force core-provider to re-create the `vacuum` version in the CRD. You can do this changing the version of the chart in the `CompositionDefinition`.

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