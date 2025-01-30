apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: core-provider-webhook
webhooks:
- name: core.provider.krateo.io
  admissionReviewVersions:
    - v1
    - v1alpha2
    - v1alpha1
  rules:
    - operations: ["CREATE"]
      apiGroups: ["composition.krateo.io"]
      apiVersions: ["*"]
      resources: ["*"]
      scope: "*"
  sideEffects: None
  clientConfig:
    service:
      namespace: demo-system
      name: core-provider-webhook-service
      path: /mutate
      port: 9443
    caBundle: CA_BUNDLE