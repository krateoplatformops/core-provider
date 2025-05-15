spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          namespace: demo-system
          name: core-provider-webhook-service
          path: /convert
          port: 9443
        caBundle: CA_BUNDLE    
      conversionReviewVersions:
      - v1
      - v1alpha2
      - v1alpha1