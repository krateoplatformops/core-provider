## Comprehensive Deployment Guide with Expected Outcomes

- [Comprehensive Deployment Guide with Expected Outcomes](#comprehensive-deployment-guide-with-expected-outcomes)
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
  - [Initial Setup](#initial-setup)
  - [Core Platform Installation](#core-platform-installation)
  - [CompositionDefinition Deployment](#compositiondefinition-deployment)
  - [Creating Compositions](#creating-compositions)
  - [Advanced Operations](#advanced-operations)
    - [1. Deploying Multiple Versions](#1-deploying-multiple-versions)
    - [2. Upgrading Compositions (massive migration)](#2-upgrading-compositions-massive-migration)
    - [3. Upgrading Compositions with breaking changes in the `values.schema.json`](#3-upgrading-compositions-with-breaking-changes-in-the-valuesschemajson)
    - [4. Upgrading a single Composition or a subset of Compositions to a new version of a CompositionDefinition](#4-upgrading-a-single-composition-or-a-subset-of-compositions-to-a-new-version-of-a-compositiondefinition)
    - [5. Pausing Composition Reconciliation](#5-pausing-composition-reconciliation)
    - [6. Pausing Composition Gracefully](#6-pausing-composition-gracefully)
    - [7. Safely Deleting Compositions](#7-safely-deleting-compositions)
- [Troubleshooting Guide](#troubleshooting-guide)
  - [Common Issues and Diagnostic Procedures](#common-issues-and-diagnostic-procedures)
    - [1. CompositionDefinition Not Becoming Ready](#1-compositiondefinition-not-becoming-ready)
    - [2. Compositions Failing to Deploy](#2-compositions-failing-to-deploy)
    - [3. Upgrade/Rollback Failures](#3-upgraderollback-failures)
    - [4a. Certificate Issues: Webhook Failure - Valid ONLY for versions of core-provider after \<0.24.2\>](#4a-certificate-issues-webhook-failure---valid-only-for-versions-of-core-provider-after-0242)
    - [4b. Certificate Issues: Mutating Webhook Configuration - Valid ONLY for versions of core-provider before \<0.24.2\>](#4b-certificate-issues-mutating-webhook-configuration---valid-only-for-versions-of-core-provider-before-0242)
    - [4.1 Certificate Issues: Conversion Webhook Configuration](#41-certificate-issues-conversion-webhook-configuration)
    - [5. Error after creating CompositionDefinition](#5-error-after-creating-compositiondefinition)
  - [General Diagnostic Tools](#general-diagnostic-tools)
  - [Common Solutions](#common-solutions)


## Introduction

The Krateo V2 Template Fireworks App provides a complete solution for deploying and managing the GithubScaffoldingLifecycle Blueprint on Kubernetes using Krateo's Composition system. This guide covers the entire lifecycle from initial deployment to advanced management scenarios.

## Prerequisites

Before beginning, ensure you have:
- A Kubernetes cluster (v1.30+ recommended)
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
helm upgrade installer-crd installer-crd \
  --repo https://charts.krateo.io \
  --namespace krateo-system \
  --create-namespace \
  --install \
  --version 2.7.0 \
  --wait

helm upgrade installer installer \
  --repo https://charts.krateo.io \
  --namespace krateo-system \
  --create-namespace \
  --install \
  --version 2.7.0 \
  --wait
```

**Expected Behavior:**
- Helm will create the `krateo-system` namespace if it doesn't exist
- The installer chart (version 2.7.0) will be deployed
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
  helm repo add marketplace https://marketplace.krateo.io
  helm repo update marketplace
  helm install github-provider-kog-repo marketplace/github-provider-kog-repo --namespace krateo-system --create-namespace --wait --version 1.0.0
  helm install git-provider krateo/git-provider --namespace krateo-system --create-namespace --wait --version 0.10.1
  helm repo add argo https://argoproj.github.io/argo-helm
  helm repo update argo
  helm install argocd argo/argo-cd --namespace krateo-system --create-namespace --wait --version 8.0.17
  ```
These installations set up the necessary providers for GitHub and ArgoCD, enabling integration with your deployment process of the applation.

### CompositionDefinition Deployment

#### 1. Creating the Application Namespace
```bash
kubectl create namespace cheatsheet-system
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
  name: lifecycleapp-cd-v1
  namespace: cheatsheet-system
spec:
  chart:
    repo: github-scaffolding-lifecycle
    url: https://marketplace.krateo.io
    version: 0.0.1
EOF
```
**What Happens Next:**
- Krateo processes the definition and generates a Custom Resource Definition (CRD)
- A dedicated controller pod is deployed to manage compositions
- The system prepares to accept GithubScaffoldingLifecycle custom resources

#### 3. Verifying CompositionDefinition Status
```bash
kubectl wait compositiondefinition lifecycleapp-cd-v1 --for condition=Ready=True --namespace cheatsheet-system --timeout=600s
```
**System Behavior:**
- Command waits until the CompositionDefinition is fully processed
- During this time, Krateo is setting up the necessary controllers
- Success means you can now create GithubScaffoldingLifecycle instances

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


#### 3. Creating the GithubScaffoldingLifecycle Instance

```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v0-0-1
kind: GithubScaffoldingLifecycle
metadata:
  name: lifecycle-composition-1
  namespace: cheatsheet-system
spec:
  argocd:
    namespace: krateo-system
    application:
      project: default
      source:
        path: chart/
      destination:
        server: https://kubernetes.default.svc
        namespace: githubscaffolding-app
      syncEnabled: false
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
  app:
    service:
      type: NodePort
      port: 31180
  git:
    unsupportedCapabilities: true
    insecure: true
    fromRepo:
      scmUrl: https://github.com
      org: krateoplatformops-blueprints
      name: github-scaffolding-lifecycle
      branch: main
      path: skeleton/
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
    toRepo:
      scmUrl: https://github.com
      org: krateoplatformops-test # Replace with your GitHub organization
      name: lifecycleapp-test-1 # You can customize the repository name
      branch: main
      path: /
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
      private: false
      initialize: true
      deletionPolicy: Delete
      verbose: false
      configurationRef:
        name: repo-config
        namespace: demo-system
EOF
```

**What Occurs:**
- Krateo creates a new GithubScaffoldingLifecycle resource
- The controller begins provisioning the application
- ArgoCD is configured to manage the deployment
- A new GitHub repository is created (if specified)

#### 4. Monitoring Composition Progress
```bash
kubectl wait githubscaffoldinglifecycles lifecycle-composition-1 \
  --for condition=Ready=True \
  --timeout=300s \
  --namespace cheatsheet-system
```
**Expected Workflow:**
- Command waits until all resources are provisioned
- During this time, containers are pulled and started
- Services are created and become accessible
- Success means your application is fully deployed

#### 5. Check Helm Release Status
```bash
helm list -n cheatsheet-system
```

**What to Expect:**
- Helm lists all releases in the `cheatsheet-system` namespace
- You should see the `lifecycle-composition-1-<[:8]UUID>` release with version 0.0.1
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
  name: lifecycleapp-cd-v2
  namespace: cheatsheet-system
spec:
  chart:
    repo: github-scaffolding-lifecycle
    url: https://marketplace.krateo.io
    version: 0.0.2
EOF
```
**System Response:**
- A second controller is deployed for the new version
- Both versions can operate simultaneously
- Each version maintains its own CRD and controller
  
This will create a new `CompositionDefinition` named `lifecycleapp-cd-v2` in the `cheatsheet-system` namespace, which will manage resources of version 0.0.2 of the lifecycleapp chart.
You can then deploy the new version of the chart by applying the `CompositionDefinition` manifest. The `core-provider` will add a new version to the existing CRD `githubscaffoldinglifecycles.composition.krateo.io` and deploy a new instance of the `composition-dynamic-controller` to manage resources of version 0.0.2.
The `core-provider` will leave the previous version of the chart (0.0.1) running along with its associated `composition-dynamic-controller` instance. This allows you to run multiple versions of the same application simultaneously, each managed by its own `composition-dynamic-controller`.

##### Verifying CompositionDefinition Status
```bash
kubectl wait compositiondefinition lifecycleapp-cd-v2 --for condition=Ready=True --namespace cheatsheet-system --timeout=600s
```
**System Behavior:**
- Command waits until the CompositionDefinition is fully processed
- During this time, Krateo is setting up the necessary controllers
- Success means you can now create GithubScaffoldingLifecycle instances

##### Create a New GithubScaffoldingLifecycle Instance
```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v0-0-2
kind: GithubScaffoldingLifecycle
metadata:
  name: lifecycle-composition-2
  namespace: cheatsheet-system
spec:
  argocd:
    namespace: krateo-system
    application:
      project: default
      source:
        path: chart/
      destination:
        server: https://kubernetes.default.svc
        namespace: githubscaffolding-app
      syncEnabled: false
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
  app:
    service:
      type: NodePort
      port: 31180
  git:
    unsupportedCapabilities: true
    insecure: true
    fromRepo:
      scmUrl: https://github.com
      org: krateoplatformops-blueprints
      name: github-scaffolding-lifecycle
      branch: main
      path: skeleton/
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
    toRepo:
      scmUrl: https://github.com
      org: krateoplatformops-test # Replace with your GitHub organization
      name: lifecycleapp-test-2 # You can customize the repository name
      branch: main
      path: /
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
      private: false
      initialize: true
      deletionPolicy: Delete
      verbose: false
      configurationRef:
        name: repo-config
        namespace: demo-system
EOF
```

##### Check Helm Release Status
```bash
helm list -n cheatsheet-system
```

**What to Expect:**
- You should see both `lifecycle-composition-1` and `lifecycle-composition-2`
- Each release corresponds to its respective version
- This confirms that both versions are deployed and managed independently


#### 2. Upgrading Compositions (massive migration)

##### Scenario:
You need to upgrade the existing version of the application to a newer version (0.0.2). To be sure the new version can be deployed, the `values.schema.json` should not add any new required fields or remove any existing required fields. If you need to add new required fields, you should create a new `CompositionDefinition` with the new version of the chart. Refer to [the next section](#3-upgrading-compositions-with-changes-in-the-valuesschemajson) for more details. 

```bash
kubectl patch compositiondefinition lifecycleapp-cd-v1 \
  -n cheatsheet-system \
  --type=merge \
  -p '{"spec":{"chart":{"version":"0.0.2"}}}'
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
kubectl get po -n cheatsheet-system
```

This automatic cleanup helps maintain cluster cleaniness by removing outdated controller instances when they are no longer needed.

##### Check Helm Release Status
```bash
helm list -n cheatsheet-system
```

**What to Expect:**
- You should see both `lifecycle-composition-1` and `lifecycle-composition-2` listed
- Each release corresponds to its respective version but now `lifecycle-composition-1` will be updated to version 0.0.2


#### 3. Upgrading Compositions with breaking changes in the `values.schema.json`
##### Scenario:
This can be useful for upgrading a chart where you have changed the `values.schema.json` and you want to ensure that the new version can be deployed without issues.

About breaking changes in a new version of a chart:
- Adding new required fields in the `values.schema.json`
- Removing existing required fields in the `values.schema.json`
- Changing the type of existing fields in the `values.schema.json`
- Modifying the structure of the `values.schema.json` in a way that is not backward compatible
- Introducing new dependencies that are not compatible with the previous version
- Altering validation rules that could reject previously valid configurations

Suppose you have installed composition `lifecycleapp-cd-v1` with version 0.0.1 and you want to upgrade it to version 0.0.2, but the new version has changes in the `values.schema.json` that require you to create a new `CompositionDefinition` with the new version of the chart.

The first step is to create a new `CompositionDefinition` with the new version of the chart. You can do this by applying the following manifest as done in [the previous section](#1-deploying-multiple-versions):
```bash
cat <<EOF | kubectl apply -f -
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: lifecycleapp-cd-v2
  namespace: lifecycleapp-system
spec:
  chart:
    repo: github-scaffolding-lifecycle
    url: https://marketplace.krateo.io
    version: 0.0.2
EOF
```

At this point, you have two `CompositionDefinition` resources in the cluster: `lifecycleapp-cd-v1` with version 0.0.1 and `lifecycleapp-cd-v2` with version 0.0.2.

At this point, suppose you need to upgrade an existing release of a composition to the new version. You can do this by following the steps below:

##### Step 1: Pause the Composition with the Old Version
```bash
kubectl annotate githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  "krateo.io/paused=true"
```

This ensures that the old version of the composition is not reconciled while you are upgrading to the new version.

##### Step 2: Create a New Composition Instance with the New Version

Create a new GithubScaffoldingLifecycle instance with the new version of the chart. Note that you need to add a label called `krateo.io/release-name` to the new GithubScaffoldingLifecycle instance. This label is used by the `composition-dynamic-controller` to identify the release name of the composition. The value of this label should be the same as the name of the old GithubScaffoldingLifecycle instance. In the following example, the value of the label is `lifecycle-composition-1`, which is the name of the release installed at [step 1](#3-creating-the-githubscaffoldinglifecycle-instance). Normally, this label is automatically added to the GithubScaffoldingLifecycle instance when it is created. However, in this case, you need to add it manually because you are creating a new GithubScaffoldingLifecycle instance with a different name.

The manifest below shows how to create a new GithubScaffoldingLifecycle instance with the new version of the chart, with the values that follows the new `values.schema.json` with breaking changes.
You can do that by applying the following manifest: 

As a first step, you need to get the release name of the old composition. You can do that by running the following command:
```bash
kubectl get githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  -o jsonpath='{.metadata.labels.krateo\.io/release-name}'
```

Then, you can use the release name in the manifest below.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v0-0-2
kind: GithubScaffoldingLifecycle
metadata:
  name: lifecycle-composition-2
  namespace: cheatsheet-system
  labels:
    krateo.io/release-name: <RELEASE_NAME_FROM_OLD_COMPOSITION>
spec:
  argocd:
    namespace: krateo-system
    application:
      project: default
      source:
        path: chart/
      destination:
        server: https://kubernetes.default.svc
        namespace: githubscaffolding-app
      syncEnabled: false
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
  app:
    service:
      type: NodePort
      port: 31180
  git:
    unsupportedCapabilities: true
    insecure: true
    fromRepo:
      scmUrl: https://github.com
      org: krateoplatformops-blueprints
      name: github-scaffolding-lifecycle
      branch: main
      path: skeleton/
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
    toRepo:
      scmUrl: https://github.com
      org: krateoplatformops-test # Replace with your GitHub organization
      name: lifecycleapp-test-2 # You can customize the repository name
      branch: main
      path: /
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
      private: false
      initialize: true
      deletionPolicy: Delete
      verbose: false
      configurationRef:
        name: repo-config
        namespace: demo-system
EOF
```

At this point, the helm release `lifecycle-composition-1`  will be upgraded to version 0.0.2. 

##### Step 3: Remove the old composition at version 0.0.1
```bash
kubectl delete githubscaffoldinglifecycle lifecycle-composition-1 \
  -n cheatsheet-system
```

You also need to append the annotation `krateo.io/management-policy=orphan` to the new composition to prevent the deletion of the helm release when the old composition is deleted. You can do this by applying the following command:

```bash
kubectl annotate githubscaffoldinglifecycle lifecycle-composition-1 \
  -n cheatsheet-system \
  krateo.io/management-policy=orphan
```

You also need to remove the finalizer in the old composition because the annotation `krateo.io/paused=true` will prevent the old composition from being deleted. You can do this by applying the following command:

```bash
kubectl patch githubscaffoldinglifecycle lifecycle-composition-1 \
  -n cheatsheet-system \
  --type=merge \
  -p '{"metadata":{"finalizers":null}}'
```

At this point, the old composition will be deleted and the new composition will be upgraded to version 1.1.14.

#### 4. Upgrading a single Composition or a subset of Compositions to a new version of a CompositionDefinition

##### Scenario:
This can be useful when you want to upgrade only specific Compositions after a new version of a CompositionDefinition is released, while keeping other Compositions on the previous version. This allows for a gradual migration and testing of the new version without impacting all existing Compositions at once.

Suppose you have installed the CompositionDefinition `lifecycleapp-cd-v1` with version 0.0.1 (from [the previous step](#2-deploying-the-compositiondefinition)), you have already created some Compositions based on it, and now you want to upgrade only some of these Compositions to version 0.0.2 of the CompositionDefinition.
Note that the version 0.0.2 of the CompositionDefinition introduces an additional field `description` at path `git.toRepo.description`, with a default value.

##### Step 1: Setup Initial CompositionDefinition and Compositions


Now, let's create two Compositions based on this CompositionDefinition:

```sh
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v0-0-1
kind: GithubScaffoldingLifecycle
metadata:
  name: composition-1
  namespace: cheatsheet-system
spec:
  argocd:
    namespace: krateo-system
    application:
      project: default
      source:
        path: chart/
      destination:
        server: https://kubernetes.default.svc
        namespace: githubscaffolding-app
      syncEnabled: false
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
  app:
    service:
      type: NodePort
      port: 31180
  git:
    unsupportedCapabilities: true
    insecure: true
    fromRepo:
      scmUrl: https://github.com
      org: krateoplatformops-blueprints
      name: github-scaffolding-lifecycle
      branch: main
      path: skeleton/
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
    toRepo:
      scmUrl: https://github.com
      org: krateoplatformops-test
      name: postdemoday-1
      branch: main
      path: /
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
      private: false
      initialize: true
      deletionPolicy: Delete
      verbose: false
      configurationRef:
        name: repo-config
        namespace: demo-system
EOF
```

```sh
cat <<EOF | kubectl apply -f -
apiVersion: composition.krateo.io/v0-0-1
kind: GithubScaffoldingLifecycle
metadata:
  name: composition-2
  namespace: cheatsheet-system
spec:
  argocd:
    namespace: krateo-system
    application:
      project: default
      source:
        path: chart/
      destination:
        server: https://kubernetes.default.svc
        namespace: githubscaffolding-app
      syncEnabled: false
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
  app:
    service:
      type: NodePort
      port: 31180
  git:
    unsupportedCapabilities: true
    insecure: true
    fromRepo:
      scmUrl: https://github.com
      org: krateoplatformops-blueprints
      name: github-scaffolding-lifecycle
      branch: main
      path: skeleton/
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
    toRepo:
      scmUrl: https://github.com
      org: krateoplatformops-test
      name: postdemoday-1
      branch: main
      path: /
      credentials:
        authMethod: generic
        secretRef:
          namespace: krateo-system
          name: github-repo-creds
          key: token
      private: false
      initialize: true
      deletionPolicy: Delete
      verbose: false
      configurationRef:
        name: repo-config
        namespace: demo-system
EOF
```

At this point, both `composition-1` and `composition-2` are based on version 0.0.1 of the CompositionDefinition and are managed by the same CDC (`githubscaffoldinglifecycles-v0-0-1-controller`).

##### Step 2: Apply the New Version of the CompositionDefinition

Now, let's apply the new version 0.0.2 of the CompositionDefinition:

```sh
cat <<EOF | kubectl apply -f -
apiVersion: core.krateo.io/v1alpha1
kind: CompositionDefinition
metadata:
  name: lifecycleapp-cd-v2
  namespace: cheatsheet-system
spec:
  chart:
    repo: github-scaffolding-lifecycle
    url: https://marketplace.krateo.io
    version: 0.0.2
EOF
```

##### Step 3: Upgrade Specific Composition to the New Version

In order to upgrade only `composition-2` to version 0.0.2 of the CompositionDefinition, we need to update 2 labels on the `composition-2` resource.

From:
```yaml
krateo.io/composition-definition-name: github-scaffolding-lifecycle-v1
krateo.io/composition-version: v0-0-1
```
To:
```yaml
krateo.io/composition-definition-name: github-scaffolding-lifecycle-v2
krateo.io/composition-version: v0-0-2
```

The other labels should remain unchanged.

##### Step 4: Verify the Upgrade

At this point, the CDC `githubscaffoldinglifecycles-v0-0-2-controller` will take over the management of `composition-2` while `composition-1` will still be managed by the previous CDC `githubscaffoldinglifecycles-v0-0-1-controller`.

To check whether the upgrade was successful, you can check the newly added default field `description` in the `toRepo` section of `composition-2`:

```sh
kubectl get githubscaffoldinglifecycles.v0-0-2.composition.krateo.io composition-2 -o yaml
```

Note that checking the version v0-0-1 of `composition-2` will not show the new `description` field.

Additionally, checking the logs of both CDCs will show that `composition-2` is being reconciled by the new CDC
(`githubscaffoldinglifecycles-v0-0-2-controller`) while `composition-1` continues to be reconciled by the old CDC (`githubscaffoldinglifecycles-v0-0-1-controller`).

#### 5. Pausing Composition Reconciliation

##### Use Case:
Temporarily stop automatic reconciliation during maintenance or troubleshooting.

##### Step 1: Pause Composition
```bash
kubectl annotate githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  "krateo.io/paused=true"
```

**Expected Result:**
- Composition is annotated with `krateo.io/paused=true`
- Controller stops reconciling this instance
- Verify with:
  ```bash
  kubectl get githubscaffoldinglifecycles lifecycle-composition-1 -n cheatsheet-system -o jsonpath='{.metadata.annotations}'
  ```
  You could also check events:
  ```bash
  kubectl get events -n cheatsheet-system --sort-by='.metadata.creationTimestamp' | grep "lifecycle-composition-1"
  ```


##### Step 2: Make Manual Changes
During paused state, you can:
- Manually modify resources
- Troubleshoot issues
- Perform maintenance

##### Step 3: Resume Reconciliation
```bash
kubectl annotate githubscaffoldinglifecycles lifecycle-composition-1 \
  -n cheatsheet-system \
  "krateo.io/paused-"
```

**Expected Result:**
- Pause annotation is removed
- Controller resumes normal operation
- Changes are reconciled according to desired state



#### 6. Pausing Composition Gracefully
`composition-dynamic-controller` 0.19.3 and later (released with `core-provider-chart` 0.33.4) supports a graceful pause mechanism that ensures any resource managed by the composition will be paused *before* the composition reconciliation is paused. This means the resources managed by the composition will not be reconciled until the composition is resumed.

To support this feature, you need to follow the steps described in the [`composition-dynamic-controller` documentation](https://github.com/krateoplatformops/composition-dynamic-controller?tab=readme-ov-file#about-the-gracefullypaused-value). Please note that this feature is not supported for compositions created with versions of `composition-dynamic-controller` earlier than 0.19.3, and you will need to modify your chart according to the documentation linked above.

#### 7. Safely Deleting Compositions

You can safely delete all compositions and their associated resources using the following command:

```bash
kubectl delete compositiondefinition lifecycleapp-cd-v1 \
  -n cheatsheet-system
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
kubectl describe compositiondefinition <name> -n <namespace>

# Examine core provider logs for processing errors
kubectl logs -n krateo-system -l app=core-provider --tail=100
```

**What to Look For:**
- In the describe output: Check "Status.Conditions" for error messages
- In core provider logs: Look for chart download or CRD generation failures
- Verify network connectivity to the chart repository

#### 2. Compositions Failing to Deploy

**Symptoms:**
- GithubScaffoldingLifecycle resource stuck in "Not Ready" state


**Diagnostic Steps:**
```bash
# Get detailed status of the composition
kubectl describe githubscaffoldinglifecycles <name> -n cheatsheet-system

# Check controller logs for reconciliation errors
kubectl logs -n cheatsheet-system -l app.kubernetes.io/name=githubscaffoldinglifecycles-controller

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
kubectl rollout status deployment/githubscaffoldinglifecycles-v0-0-2-controller -n cheatsheet-system

# Verify resource versions
kubectl get all -n githubscaffolding-app -l krateo.io/composition-version

# Compare desired vs actual state
kubectl get githubscaffoldinglifecycles -n cheatsheet-system -o yaml
```

**Investigation Areas:**
- Resource version labels consistency
- Controller pod logs for reconciliation errors
- Helm release history for the composition

#### 4a. Certificate Issues: Webhook Failure - Valid ONLY for versions of core-provider after <0.24.2>
**Symptoms:**
- You receive an error like `x509: certificate signed by unknown authority`
- You cannot create/get/list/delete the related composition in the cluster
  

Something went wrong with the automatic rotation of the webhook certificates managed by the core-provider. To force the rotation of the certificates, you need to restart the core-provider pod.
**Diagnostic Steps:**
```bash
# List core-provider pods
kubectl get pods -n krateo-system -l app=core-provider
```
**Remediation Steps:**
- Restart the core-provider pod to trigger certificate rotation
```bash
kubectl delete pod -n krateo-system -l app=core-provider
```

#### 4b. Certificate Issues: Mutating Webhook Configuration - Valid ONLY for versions of core-provider before <0.24.2>
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