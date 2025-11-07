package certificates

import (
	"os"
	"path/filepath"
	"testing"

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
