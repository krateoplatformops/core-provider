//go:build integration
// +build integration

package certs_test

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/krateoplatformops/core-provider/internal/tools/certs"
	kube "github.com/krateoplatformops/plumbing/certs"
	"github.com/krateoplatformops/plumbing/e2e"
	xenv "github.com/krateoplatformops/plumbing/env"
	certv1 "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/support/kind"
)

var (
	testenv     env.Environment
	clusterName string
	namespace   string
)

const (
	testCertsPath = "./test-certs"
)

func TestMain(m *testing.M) {
	xenv.SetTestMode(true)

	namespace = "test-certs-system"
	clusterName = "krateo-certs-test"
	testenv = env.New()
	kindCluster := kind.NewCluster(clusterName)

	testenv.Setup(
		envfuncs.CreateCluster(kindCluster, clusterName),
		e2e.CreateNamespace(namespace),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			// Create certificates directory for tests
			if err := os.MkdirAll(testCertsPath, 0755); err != nil {
				return ctx, fmt.Errorf("failed to create test certs directory: %w", err)
			}
			return ctx, nil
		},
	).Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyCluster(clusterName),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			// Clean up test certificates
			if err := os.RemoveAll(testCertsPath); err != nil {
				return ctx, fmt.Errorf("failed to clean up test certs directory: %w", err)
			}
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}

func TestGenerateClientCertAndKey(t *testing.T) {
	f := features.New("Certificate Generation").
		Assess("Generate a new client certificate and key", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			_, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create resources client: %v", err)
			}

			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create kubernetes client: %v", err)
			}

			logger := func(msg string, keysAndValues ...any) {
				t.Logf("%s: %v", msg, keysAndValues)
			}

			// Test parameters
			testUsername := "test-node"
			testDuration := 24 * time.Hour
			testApprover := "test-signer"

			// Generate a certificate
			opts := certs.GenerateClientCertAndKeyOpts{
				Duration:              testDuration,
				LeaseExpirationMargin: time.Hour,
				Username:              testUsername,
				Approver:              testApprover,
			}

			cert, key, err := certs.GenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Failed to generate certificate and key: %v", err)
			}

			// Verify we got non-empty cert and key
			if cert == "" {
				t.Fatal("Generated certificate is empty")
			}
			if key == "" {
				t.Fatal("Generated key is empty")
			}

			// Verify cert and key are valid base64
			_, err = base64.StdEncoding.DecodeString(cert)
			if err != nil {
				t.Fatalf("Generated certificate is not valid base64: %v", err)
			}
			keyBytes, err := base64.StdEncoding.DecodeString(key)
			if err != nil {
				t.Fatalf("Generated key is not valid base64: %v", err)
			}

			// Verify key is in PEM format
			block, _ := pem.Decode(keyBytes)
			if block == nil {
				t.Fatal("Failed to decode PEM key")
			}
			if block.Type != "RSA PRIVATE KEY" {
				t.Fatalf("Expected PEM block type 'RSA PRIVATE KEY', got '%s'", block.Type)
			}

			// Verify the CSR exists in Kubernetes
			csr, err := kube.GetCertificateSigningRequest(clientset, testUsername)
			if err != nil {
				t.Fatalf("Failed to get CSR: %v", err)
			}

			// Verify the CSR is approved
			isApproved := false
			for _, condition := range csr.Status.Conditions {
				if condition.Type == certv1.CertificateApproved {
					isApproved = true
					break
				}
			}
			if !isApproved {
				t.Fatal("CSR is not approved")
			}

			return ctx
		}).
		Assess("Check or regenerate certificate", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create kubernetes client: %v", err)
			}

			logger := func(msg string, keysAndValues ...any) {
				t.Logf("%s: %v", msg, keysAndValues)
			}

			// Test parameters
			testUsername := "test-node-regenerate"
			testDuration := 24 * time.Hour
			testApprover := "test-signer"

			opts := certs.GenerateClientCertAndKeyOpts{
				Duration:              testDuration,
				LeaseExpirationMargin: time.Hour,
				Username:              testUsername,
				Approver:              testApprover,
			}

			// First call should generate a new cert
			exists, cert, key, err := certs.CheckOrRegenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Failed to check or regenerate certificate: %v", err)
			}

			if exists {
				t.Fatal("Expected certificate to not exist yet")
			}
			if cert == "" || key == "" {
				t.Fatal("Expected non-empty certificate and key")
			}

			// Second call should return that cert exists
			exists, _, _, err = certs.CheckOrRegenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Failed on second check for certificate: %v", err)
			}

			if !exists {
				t.Fatal("Expected certificate to exist on second check")
			}

			return ctx
		}).
		Assess("Test certificate expiration", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create kubernetes client: %v", err)
			}

			logger := func(msg string, keysAndValues ...any) {
				t.Logf("%s: %v", msg, keysAndValues)
			}

			// Test parameters - using very short duration to test expiration
			testUsername := "test-node-expire"
			testDuration := 600 * time.Second
			testApprover := "test-signer"
			testMargin := 1 * time.Second

			opts := certs.GenerateClientCertAndKeyOpts{
				Duration:              testDuration,
				LeaseExpirationMargin: testMargin,
				Username:              testUsername,
				Approver:              testApprover,
			}

			// Generate a certificate
			cert, key, err := certs.GenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Failed to generate certificate and key: %v", err)
			}

			if cert == "" || key == "" {
				t.Fatal("Expected non-empty certificate and key")
			}

			// Get the CSR
			csr, err := kube.GetCertificateSigningRequest(clientset, testUsername)
			if err != nil {
				t.Fatalf("Failed to get CSR: %v", err)
			}

			// Wait for the certificate to "expire" based on our margin
			time.Sleep(3 * time.Second)

			// Check if it's expired
			expired := certs.Expired(csr, testMargin)
			if !expired {
				t.Fatal("Expected certificate to be expired")
			}

			// Regenerate should create a new cert
			exists, newCert, newKey, err := certs.CheckOrRegenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Failed to regenerate certificate: %v", err)
			}

			if exists {
				t.Fatal("Expected expired certificate to be regenerated")
			}
			if newCert == "" || newKey == "" {
				t.Fatal("Expected non-empty regenerated certificate and key")
			}
			if newCert == cert || newKey == key {
				t.Fatal("Expected different certificate and key after regeneration")
			}

			return ctx
		}).
		Assess("Update certificate files", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create kubernetes client: %v", err)
			}

			logger := func(msg string, keysAndValues ...any) {
				t.Logf("%s: %v", msg, keysAndValues)
			}

			// Test parameters
			testUsername := "test-node-files"
			testDuration := 24 * time.Hour
			testApprover := "test-signer"

			opts := certs.GenerateClientCertAndKeyOpts{
				Duration:              testDuration,
				LeaseExpirationMargin: time.Hour,
				Username:              testUsername,
				Approver:              testApprover,
			}

			// Generate a certificate
			cert, key, err := certs.GenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Failed to generate certificate and key: %v", err)
			}

			// Write files
			certPath := filepath.Join(testCertsPath, testUsername)
			err = certs.UpdateCerts(cert, key, certPath)
			if err != nil {
				t.Fatalf("Failed to update certificate files: %v", err)
			}

			// Verify files exist
			certFile := filepath.Join(certPath, "tls.crt")
			keyFile := filepath.Join(certPath, "tls.key")

			if _, err := os.Stat(certFile); os.IsNotExist(err) {
				t.Fatalf("Certificate file not created: %s", certFile)
			}
			if _, err := os.Stat(keyFile); os.IsNotExist(err) {
				t.Fatalf("Key file not created: %s", keyFile)
			}

			// Clean up test files for this specific test
			if err := os.RemoveAll(certPath); err != nil {
				t.Logf("Warning: Failed to clean up test cert directory: %v", err)
			}

			return ctx
		}).
		Assess("Test CSR deletion and recreation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create kubernetes client: %v", err)
			}

			logger := func(msg string, keysAndValues ...any) {
				t.Logf("%s: %v", msg, keysAndValues)
			}

			// Test parameters
			testUsername := "test-node-recreate"
			testDuration := 24 * time.Hour
			testApprover := "test-signer"

			opts := certs.GenerateClientCertAndKeyOpts{
				Duration:              testDuration,
				LeaseExpirationMargin: time.Hour,
				Username:              testUsername,
				Approver:              testApprover,
			}

			// First, create a certificate
			cert1, key1, err := certs.GenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Failed to generate certificate and key: %v", err)
			}

			// Force recreation by calling generate again (which should delete and recreate)
			cert2, key2, err := certs.GenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Failed to regenerate certificate and key: %v", err)
			}

			// CSRs should be different (they have different timestamps and keys)
			if cert1 == cert2 {
				t.Fatal("Expected different certificates after recreation")
			}
			if key1 == key2 {
				t.Fatal("Expected different keys after recreation")
			}

			// Verify we can get the recreated CSR
			_, err = kube.GetCertificateSigningRequest(clientset, testUsername)
			if err != nil {
				t.Fatalf("Failed to get recreated CSR: %v", err)
			}

			return ctx
		}).
		Assess("Test error handling for missing CSR", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create kubernetes client: %v", err)
			}

			logger := func(msg string, keysAndValues ...any) {
				t.Logf("%s: %v", msg, keysAndValues)
			}

			// Test parameters
			nonExistentCSR := "non-existent-csr"

			// Verify the CSR doesn't exist
			_, err = kube.GetCertificateSigningRequest(clientset, nonExistentCSR)
			if !errors.IsNotFound(err) {
				t.Fatalf("Expected NotFound error for non-existent CSR, got: %v", err)
			}

			// Check/regenerate should handle this gracefully
			opts := certs.GenerateClientCertAndKeyOpts{
				Duration:              time.Hour,
				LeaseExpirationMargin: time.Hour,
				Username:              nonExistentCSR,
				Approver:              "test-approver",
			}

			exists, cert, key, err := certs.CheckOrRegenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("CheckOrRegenerateClientCertAndKey failed for non-existent CSR: %v", err)
			}

			if exists {
				t.Fatal("Expected non-existent CSR to be reported as not existing")
			}
			if cert == "" || key == "" {
				t.Fatal("Expected non-empty certificate and key for non-existent CSR")
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, f)
}

func TestFullCertificateLifecycle(t *testing.T) {
	f := features.New("Certificate Lifecycle").
		Assess("Full certificate lifecycle test", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create kubernetes client: %v", err)
			}

			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create resources client: %v", err)
			}

			logger := func(msg string, keysAndValues ...any) {
				t.Logf("%s: %v", msg, keysAndValues)
			}

			// Test parameters
			testUsername := "test-node-lifecycle"
			testDuration := 24 * time.Hour
			testApprover := "test-signer"
			testMargin := 12 * time.Hour
			testCertDir := filepath.Join(testCertsPath, "lifecycle-test")

			opts := certs.GenerateClientCertAndKeyOpts{
				Duration:              testDuration,
				LeaseExpirationMargin: testMargin,
				Username:              testUsername,
				Approver:              testApprover,
			}

			// 1. Check if certificate exists (it shouldn't) and generate
			exists, cert, key, err := certs.CheckOrRegenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Initial certificate check failed: %v", err)
			}
			if exists {
				t.Fatal("Expected certificate to not exist initially")
			}
			if cert == "" || key == "" {
				t.Fatal("Expected non-empty certificate and key")
			}

			// 2. Save the certificate to disk
			err = certs.UpdateCerts(cert, key, testCertDir)
			if err != nil {
				t.Fatalf("Failed to update certificate files: %v", err)
			}

			// Verify files exist
			certFile := filepath.Join(testCertDir, "tls.crt")
			keyFile := filepath.Join(testCertDir, "tls.key")

			if _, err := os.Stat(certFile); os.IsNotExist(err) {
				t.Fatalf("Certificate file not created: %s", certFile)
			}
			if _, err := os.Stat(keyFile); os.IsNotExist(err) {
				t.Fatalf("Key file not created: %s", keyFile)
			}

			// 3. Check if certificate exists (it should now)
			exists, _, _, err = certs.CheckOrRegenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Second certificate check failed: %v", err)
			}
			if !exists {
				t.Fatal("Expected certificate to exist after generation")
			}

			// 4. Wait for CSR to complete
			csr, err := kube.GetCertificateSigningRequest(clientset, testUsername)
			if err != nil {
				t.Fatalf("Failed to get CSR: %v", err)
			}

			err = wait.For(
				conditions.New(r).ResourceMatch(csr, func(object k8s.Object) bool {
					csr := object.(*certv1.CertificateSigningRequest)
					return csr.Status.Certificate != nil
				}),
				wait.WithTimeout(2*time.Minute),
				wait.WithInterval(5*time.Second),
			)
			if err != nil {
				t.Fatalf("Timed out waiting for certificate to be issued: %v", err)
			}

			// 5. Force expiration by deleting the CSR
			err = kube.DeleteCertificateSigningRequest(clientset, testUsername)
			if err != nil {
				t.Fatalf("Failed to delete CSR: %v", err)
			}

			// Wait for deletion
			err = wait.For(
				conditions.New(r).ResourceDeleted(csr),
				wait.WithTimeout(2*time.Minute),
				wait.WithInterval(5*time.Second),
			)
			if err != nil {
				t.Fatalf("Timed out waiting for CSR deletion: %v", err)
			}

			// 6. Check again, should regenerate
			exists, newCert, newKey, err := certs.CheckOrRegenerateClientCertAndKey(clientset, logger, opts)
			if err != nil {
				t.Fatalf("Regeneration check failed: %v", err)
			}
			if exists {
				t.Fatal("Expected certificate to be regenerated after deletion")
			}
			if newCert == "" || newKey == "" {
				t.Fatal("Expected non-empty regenerated certificate and key")
			}
			if newCert == cert || newKey == key {
				t.Fatal("Expected different certificate and key after regeneration")
			}

			// 7. Update certificate files with new values
			err = certs.UpdateCerts(newCert, newKey, testCertDir)
			if err != nil {
				t.Fatalf("Failed to update certificate files: %v", err)
			}

			// 8. Clean up
			err = os.RemoveAll(testCertDir)
			if err != nil {
				t.Logf("Warning: Failed to clean up test cert directory: %v", err)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, f)
}
