# Troubleshooting

This guide covers the most common issues encountered when working with the Krateo Core Provider. Use Ctrl+F / Cmd+F to jump directly to your error.

---

## Table of Contents

- [CompositionDefinition not becoming Ready](#1-compositiondefinition-not-becoming-ready)
- [Compositions failing to deploy](#2-compositions-failing-to-deploy)
- [Upgrade/Rollback failures](#3-upgraderollback-failures)
- [Certificate Issues: Webhook Failure (core-provider ≥ 0.24.2)](#4a-certificate-issues-webhook-failure--core-provider--0242)
- [Certificate Issues: Mutating Webhook (core-provider < 0.24.2)](#4b-certificate-issues-mutating-webhook--core-provider--0242)
- [Certificate Issues: Conversion Webhook](#41-certificate-issues-conversion-webhook)
- [Error: invalid group/version "vacuum"](#5-error-invalid-groupversion-vacuum)
- [General Diagnostic Tools](#general-diagnostic-tools)
- [Common Solutions](#common-solutions)

---

## 1. CompositionDefinition not becoming Ready

**Symptoms:**
- CompositionDefinition remains in "Not Ready" state
- No corresponding CDC pod is created
- CRD is not generated

**Diagnostic steps:**

```bash
# Check status conditions
kubectl describe compositiondefinition <name> -n <namespace>

# Check Core Provider logs for chart download or CRD generation errors
kubectl logs -n krateo-system -l app=core-provider --tail=100
```

**What to look for:**
- `Status.Conditions` in the describe output — look for error messages
- Chart download failures (network, wrong URL, missing credentials)
- Missing `values.schema.json` in the chart

---

## 2. Compositions failing to deploy

**Symptoms:**
- Composition resource stuck in "Not Ready" state

**Diagnostic steps:**

```bash
# Check status and events
kubectl describe githubscaffoldinglifecycles <name> -n <namespace>

# Check CDC logs
kubectl logs -n <namespace> -l app.kubernetes.io/name=githubscaffoldinglifecycles-controller

# Check ArgoCD application status (if applicable)
kubectl get application -n krateo-system
```

**What to examine:**
- Events section for resource creation failures
- CDC logs for Git operation or Helm install errors
- ArgoCD application health and sync status

---

## 3. Upgrade/Rollback failures

**Symptoms:**
- Version transition is stuck
- Mixed-version resources exist
- CDC is in a crash loop

**Diagnostic steps:**

```bash
# Check rollout status of the new CDC
kubectl rollout status deployment/githubscaffoldinglifecycles-v0-0-2-controller \
  -n <namespace>

# Check version labels on resources
kubectl get all -n <app-namespace> -l krateo.io/composition-version

# Inspect the Composition desired vs actual state
kubectl get githubscaffoldinglifecycles -n <namespace> -o yaml
```

**Investigation areas:**
- Consistency of `krateo.io/composition-version` labels across resources
- CDC logs for reconciliation errors
- Helm release history: `helm history <release-name> -n <namespace>`

---

## 4a. Certificate Issues: Webhook Failure — core-provider ≥ 0.24.2

**Symptoms:**
- Error: `x509: certificate signed by unknown authority`
- Cannot create/get/list/delete the Composition

**Cause:** The automatic TLS certificate rotation managed by the Core Provider encountered an issue.

**Remediation:** Restart the Core Provider pod to trigger certificate rotation:

```bash
kubectl get pods -n krateo-system -l app=core-provider
kubectl delete pod -n krateo-system -l app=core-provider
```

---

## 4b. Certificate Issues: Mutating Webhook — core-provider < 0.24.2

> This applies **only** to versions before 0.24.2. From 0.24.2 onwards, certificate management is fully automatic.

**Symptoms:**
```
Internal error occurred: failed calling webhook "core.provider.krateo.io": ...
tls: failed to verify certificate: x509: certificate signed by unknown authority
```

**Cause:** The `caBundle` in the MutatingWebhookConfiguration is out of sync with the TLS secret.

**Diagnostic steps:**

```bash
# Read the caBundle from the webhook configuration
kubectl get mutatingWebhookConfiguration core-provider \
  -o jsonpath='{.webhooks[0].clientConfig.caBundle}'

# Read the cert from the TLS secret
kubectl get secret core-provider-certs -n krateo-system \
  -o json | jq -r '.data["tls.crt"]'
```

If the two values differ, patch the webhook:

```bash
caBundle=$(kubectl get secret core-provider-certs -n krateo-system \
  -o json | jq -r '.data["tls.crt"]')

kubectl patch mutatingWebhookConfiguration core-provider \
  --type='json' \
  -p='[{"op": "replace", "path": "/webhooks/0/clientConfig/caBundle", "value": "'"$caBundle"'"}]'
```

---

## 4.1 Certificate Issues: Conversion Webhook

**Symptoms:**
```
conversion webhook for composition.krateo.io/v0-0-13, Kind=... failed:
tls: failed to verify certificate: x509: certificate signed by unknown authority
```

**Diagnostic steps:**

```bash
# Read the caBundle from the CRD
kubectl get crds <your-crd-name> \
  -o jsonpath='{.spec.conversion.webhook.clientConfig.caBundle}'

# Read cert from TLS secret
kubectl get secret core-provider-certs -n krateo-system \
  -o json | jq -r '.data["tls.crt"]'
```

If they differ, patch the CRD:

```bash
caBundle=$(kubectl get secret core-provider-certs -n krateo-system \
  -o json | jq -r '.data["tls.crt"]')

kubectl patch crd <your-crd-name> \
  --type='json' \
  -p='[{"op": "replace", "path": "/spec/conversion/webhook/clientConfig/caBundle", "value": "'"$caBundle"'"}]'
```

---

## 5. Error: invalid group/version "vacuum"

**Symptoms:**
```
request to convert CR from an invalid group/version: composition.krateo.io/vacuum
```
Cannot create Compositions; error also appears when running `kubectl get compositions -A`.

**Cause:** The Core Provider's internal `vacuum` version entry in the CRD has been lost or corrupted.

**Remediation:** Force the Core Provider to recreate the `vacuum` version by changing the chart version in the CompositionDefinition (change it to any different value and back if needed). This triggers a full CRD reconciliation.

---

## General Diagnostic Tools

```bash
# Cluster-wide error events
kubectl get events -A --sort-by='.metadata.creationTimestamp' | grep -i "error\|fail"

# Cross-namespace pod status
kubectl get pods -A -o wide | grep -E "krateo-system|cheatsheet-system"

# Verify CRD and API resources are registered
kubectl api-resources | grep composition

# Basic network connectivity to chart repo
kubectl run -it --rm --restart=Never network-test \
  --image=alpine -n krateo-system -- \
  ping charts.krateo.io
```

---

## Common Solutions

| Symptom | Solution |
|:--------|:---------|
| Authentication failure downloading chart | Recreate the credentials secret; verify the token has correct scopes |
| Version conflict / mixed state | Check version labels on Compositions; delete and recreate the CompositionDefinition if necessary |
| Resource constraints / pending pods | Check `kubectl get pods --field-selector=status.phase=Pending`; increase node capacity |
| Composition stuck deleting | Remove the Composition's (not the CRD's) finalizer — see [Delete Safely](how-to/delete-safely.md) |
