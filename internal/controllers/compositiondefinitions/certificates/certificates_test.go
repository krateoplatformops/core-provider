package certificates

import (
	"os"
	"path/filepath"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

func TestNewCertManager_NilRestConfig(t *testing.T) {
	_, err := NewCertManager(Opts{})
	if err == nil {
		t.Fatal("expected error when RestConfig is nil, got nil")
	}
	if err.Error() != "rest config cannot be nil" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCertManager_EmptyTemplatePath(t *testing.T) {
	rc := &rest.Config{}
	_, err := NewCertManager(Opts{
		RestConfig: rc,
	})
	if err == nil {
		t.Fatal("expected error when MutatingWebhookTemplatePath is empty, got nil")
	}
	if err.Error() != "mutating webhook template path cannot be empty" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCertManager_EmptyServiceName(t *testing.T) {
	rc := &rest.Config{}
	_, err := NewCertManager(Opts{
		RestConfig:                  rc,
		MutatingWebhookTemplatePath: "tmpl",
	})
	if err == nil {
		t.Fatal("expected error when WebhookServiceName is empty, got nil")
	}
	if err.Error() != "webhook service name cannot be empty" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCertManager_EmptyServiceNamespace(t *testing.T) {
	rc := &rest.Config{}
	_, err := NewCertManager(Opts{
		RestConfig:                  rc,
		MutatingWebhookTemplatePath: "tmpl",
		WebhookServiceName:          "svc",
	})
	if err == nil {
		t.Fatal("expected error when WebhookServiceNamespace is empty, got nil")
	}
	if err.Error() != "webhook service namespace cannot be empty" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCABundle_Success(t *testing.T) {
	dir := t.TempDir()
	content := []byte("my-ca-bundle")
	if err := os.WriteFile(filepath.Join(dir, "tls.crt"), content, 0o644); err != nil {
		t.Fatalf("failed to write tls.crt: %v", err)
	}

	got, err := getCABundle(dir)
	if err != nil {
		t.Fatalf("getCABundle returned error: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("expected %q, got %q", string(content), string(got))
	}
}

func TestInjectConversionConfToCRD(t *testing.T) {
	m := &CertManager{
		webhookServiceMeta: types.NamespacedName{
			Name:      "my-svc",
			Namespace: "my-ns",
		},
		caBundle: []byte("cab"),
	}

	crd := &apiextensionsv1.CustomResourceDefinition{}
	m.InjectConversionConfToCRD(crd)

	if crd.Spec.Conversion == nil {
		t.Fatal("expected Conversion to be injected, got nil")
	}
	if crd.Spec.Conversion.Strategy != apiextensionsv1.WebhookConverter {
		t.Fatalf("expected Strategy WebhookConverter, got %v", crd.Spec.Conversion.Strategy)
	}
	if crd.Spec.Conversion.Webhook == nil || crd.Spec.Conversion.Webhook.ClientConfig == nil {
		t.Fatalf("expected Webhook and ClientConfig to be set")
	}
	cc := crd.Spec.Conversion.Webhook.ClientConfig
	if cc.Service == nil {
		t.Fatalf("expected Service to be set in ClientConfig")
	}
	if cc.Service.Name != "my-svc" {
		t.Fatalf("expected Service.Name my-svc, got %s", cc.Service.Name)
	}
	if cc.Service.Namespace != "my-ns" {
		t.Fatalf("expected Service.Namespace my-ns, got %s", cc.Service.Namespace)
	}
	if cc.Service.Path == nil || *cc.Service.Path != "/convert" {
		t.Fatalf("expected Service.Path /convert, got %v", cc.Service.Path)
	}
	if cc.Service.Port == nil || *cc.Service.Port != int32(9443) {
		t.Fatalf("expected Service.Port 9443, got %v", cc.Service.Port)
	}
	if string(cc.CABundle) != "cab" {
		t.Fatalf("expected CABundle 'cab', got %q", string(cc.CABundle))
	}
}
