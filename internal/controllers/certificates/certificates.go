package certificates

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	crdclient "github.com/krateoplatformops/core-provider/internal/tools/crd"
	crdutils "github.com/krateoplatformops/core-provider/internal/tools/crd/generation"

	"github.com/krateoplatformops/core-provider/internal/tools/pluralizer"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/krateoplatformops/core-provider/internal/tools/certs"
	"github.com/krateoplatformops/core-provider/internal/tools/kube"
	"github.com/krateoplatformops/core-provider/internal/tools/objects"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CertManagerInterface abstracts certificate lifecycle management for testability.
type CertManagerInterface interface {
	ManageCertificates(ctx context.Context, gvr schema.GroupVersionResource) error
	GetCABundle() []byte
	GetServiceName() string
	GetServiceNamespace() string
	UpdateExistingResources(ctx context.Context) error
}

type CertManager struct {
	kube                        client.Client
	client                      kubernetes.Interface
	certOpts                    certs.GenerateClientCertAndKeyOpts
	log                         func(msg string, keysAndValues ...any)
	pluralizer                  pluralizer.PluralizerInterface
	mutatingWebhookTemplatePath string
	certPath                    string
	caBundleMu                  sync.RWMutex
	caBundle                    []byte
	webhookServiceMeta          types.NamespacedName
	certGenMu                   sync.Mutex // Serializes certificate generation to prevent file system races
}

type Opts struct {
	WebhookServiceName          string
	WebhookServiceNamespace     string
	MutatingWebhookTemplatePath string
	CertOpts                    certs.GenerateClientCertAndKeyOpts
	RestConfig                  *rest.Config
}

func NewCertManager(o Opts, optsFuncs ...FuncOption) (*CertManager, error) {
	if o.RestConfig == nil {
		return nil, fmt.Errorf("rest config cannot be nil")
	}
	if o.MutatingWebhookTemplatePath == "" {
		return nil, fmt.Errorf("mutating webhook template path cannot be empty")
	}
	if o.WebhookServiceName == "" {
		return nil, fmt.Errorf("webhook service name cannot be empty")
	}
	if o.WebhookServiceNamespace == "" {
		return nil, fmt.Errorf("webhook service namespace cannot be empty")
	}
	kube, err := client.New(o.RestConfig, client.Options{})
	if err != nil {
		panic(fmt.Sprintf("error creating kube client: %s", err))
	}
	client, err := kubernetes.NewForConfig(o.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %w", err)
	}

	opts := defaultOptions()
	for _, fn := range optsFuncs {
		fn(&opts)
	}

	mgr := &CertManager{
		kube:                        kube,
		client:                      client,
		certOpts:                    o.CertOpts,
		log:                         opts.log,
		pluralizer:                  opts.pluralizer,
		mutatingWebhookTemplatePath: o.MutatingWebhookTemplatePath,
		certPath:                    opts.path,
		webhookServiceMeta: types.NamespacedName{
			Name:      o.WebhookServiceName,
			Namespace: o.WebhookServiceNamespace,
		},
	}
	cabundle, err := os.ReadFile(filepath.Join(opts.path, "tls.crt"))
	if err != nil {
		if os.IsNotExist(err) {
			mgr.RefreshCertificates()
			return mgr, nil
		}
		return nil, fmt.Errorf("error reading CA bundle: %w", err)
	}
	mgr.caBundleMu.Lock()
	mgr.caBundle = cabundle
	mgr.caBundleMu.Unlock()
	return mgr, nil
}

func (m *CertManager) ManageCertificates(ctx context.Context, gvr schema.GroupVersionResource) error {
	m.certGenMu.Lock()
	ok, cert, key, err := certs.CheckOrRegenerateClientCertAndKey(m.client, m.log, m.certOpts)
	if err != nil {
		m.certGenMu.Unlock()
		return err
	}

	if !ok {
		m.log("Certificate has been regenerated, updating certificates for webhook server")
		err = certs.UpdateCerts(cert, key, m.certPath)
		if err != nil {
			m.certGenMu.Unlock()
			return err
		}
		cabundleData, err := getCABundle(m.certPath)
		if err != nil {
			m.certGenMu.Unlock()
			return fmt.Errorf("error getting CA bundle: %w", err)
		}
		m.caBundleMu.Lock()
		m.caBundle = cabundleData
		m.caBundleMu.Unlock()
		m.log("Certificate has been updated")
	}
	m.certGenMu.Unlock()

	cabundle := m.GetCABundle()
	if err := validateCABundleFormat(cabundle); err != nil {
		return fmt.Errorf("invalid CA bundle format: %w", err)
	}

	if err := m.propagateCABundleWithRetry(ctx, cabundle, gvr); err != nil {
		return fmt.Errorf("error updating CA bundle after retries: %w", err)
	}
	return nil
}

func (m *CertManager) UpdateExistingResources(ctx context.Context) error {
	var cdList compositiondefinitionsv1alpha1.CompositionDefinitionList
	err := m.kube.List(ctx, &cdList, &client.ListOptions{Namespace: metav1.NamespaceAll})
	if err != nil {
		return fmt.Errorf("error listing CompositionDefinitions: %s", err)
	}
	for i := range cdList.Items {
		cd := &cdList.Items[i]
		if cd.Status.ApiVersion != "" && cd.Status.Kind != "" {
			gvk := schema.FromAPIVersionAndKind(cd.Status.ApiVersion, cd.Status.Kind)
			gvr, err := m.pluralizer.GVKtoGVR(gvk)
			if err != nil {
				return fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, gvk.String())
			}

			m.caBundleMu.RLock()
			cabundle := m.caBundle
			m.caBundleMu.RUnlock()

			// Use retry logic to handle transient failures in CA bundle propagation
			err = m.propagateCABundleWithRetry(ctx, cabundle, gvr)
			if err != nil {
				return fmt.Errorf("error updating CA bundle for GVR %s: %w", gvr.String(), err)
			}
			m.log("Updated CA bundle for CRD and MutatingWebhookConfiguration", "GVR", gvr.String())
		}
	}
	return nil
}

func (m *CertManager) RefreshCertificates() error {
	// Serialize certificate generation to prevent concurrent file system writes
	m.certGenMu.Lock()
	defer m.certGenMu.Unlock()

	cert, key, err := certs.GenerateClientCertAndKey(m.client, m.log, m.certOpts)
	if err != nil {
		return fmt.Errorf("error generating client certificate and key: %w", err)
	}
	err = certs.UpdateCerts(cert, key, m.certPath)
	if err != nil {
		return fmt.Errorf("error updating certificates: %w", err)
	}
	cabundle, err := getCABundle(m.certPath)
	if err != nil {
		return fmt.Errorf("error getting CA bundle: %w", err)
	}
	m.caBundleMu.Lock()
	m.caBundle = cabundle
	m.caBundleMu.Unlock()
	return nil
}

func (m *CertManager) GetCertsPath() string {
	return m.certPath
}

func (m *CertManager) GetCABundle() []byte {
	m.caBundleMu.RLock()
	defer m.caBundleMu.RUnlock()
	return m.caBundle
}

func (m *CertManager) GetServiceNamespace() string {
	return m.webhookServiceMeta.Namespace
}

func (m *CertManager) GetServiceName() string {
	return m.webhookServiceMeta.Name
}

func getCABundle(certsPath string) ([]byte, error) {
	fb, err := os.ReadFile(filepath.Join(certsPath, "tls.crt"))
	if err != nil {
		return nil, err
	}

	return fb, nil
}

// propagateCABundleWithRetry attempts to propagate the CA bundle with exponential backoff retry logic
// This handles transient failures like K8s API timeouts or conflicts
func (m *CertManager) propagateCABundleWithRetry(ctx context.Context, cabundle []byte, gvr schema.GroupVersionResource) error {
	// Validate CA bundle format upfront to fail fast
	if err := validateCABundleFormat(cabundle); err != nil {
		return fmt.Errorf("invalid CA bundle format, aborting propagation: %w", err)
	}

	const maxRetries = 3
	const initialBackoff = 100 * time.Millisecond

	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check if context is cancelled before attempting
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled: %w", err)
		}

		m.log("Propagating CA bundle", "gvr", gvr.String(), "attempt", attempt+1, "maxAttempts", maxRetries)

		err := m.propagateCABundle(ctx, cabundle, gvr)
		if err == nil {
			m.log("Successfully propagated CA bundle", "gvr", gvr.String(), "attempt", attempt+1)
			return nil
		}

		lastErr = err

		// Check if this is a transient error worth retrying
		if !isTransientError(err) {
			m.log("Non-transient error, not retrying", "gvr", gvr.String(), "error", err)
			return err
		}

		if attempt < maxRetries-1 {
			m.log("CA bundle propagation failed, retrying after backoff",
				"gvr", gvr.String(),
				"attempt", attempt+1,
				"nextBackoff", backoff.String(),
				"error", err)

			// Wait with context cancellation support
			select {
			case <-time.After(backoff):
				// Continue to next attempt
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			}

			// Exponential backoff: double the wait time
			backoff = backoff * 2
		}
	}

	return fmt.Errorf("failed to propagate CA bundle after %d attempts: %w", maxRetries, lastErr)
}

// isTransientError determines if an error is transient and worth retrying
// Returns true for errors like timeouts and conflicts that might succeed on retry
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	if apierrors.IsNotFound(err) {
		return true
	}

	errStr := err.Error()

	// K8s API transient errors
	transientPatterns := []string{
		"timeout",           // Connection timeout
		"deadline exceeded", // Context deadline exceeded
		"connection reset",  // Connection reset by peer
		"connection refused",// Connection refused
		"i/o timeout",       // I/O timeout
		"409",               // K8s Conflict - resource was modified
		"503",               // Service Unavailable
		"504",               // Gateway Timeout
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Check for specific K8s error types
	// Note: This is a simple string-based check; in production you might want
	// to use k8s.io/apimachinery/pkg/api/errors for more robust checking
	if strings.Contains(errStr, "conflict") || strings.Contains(errStr, "Conflict") {
		return true
	}

	return false
}

// validateCABundleFormat validates that the CA bundle is in valid PEM format and not empty
func validateCABundleFormat(cabundle []byte) error {
	if len(cabundle) == 0 {
		return fmt.Errorf("CA bundle is empty")
	}

	// Check for PEM markers (basic validation)
	if !bytes.Contains(cabundle, []byte("-----BEGIN")) {
		return fmt.Errorf("CA bundle missing BEGIN marker - invalid PEM format")
	}

	if !bytes.Contains(cabundle, []byte("-----END")) {
		return fmt.Errorf("CA bundle missing END marker - invalid PEM format")
	}

	// Verify PEM structure makes sense (BEGIN before END)
	beginIdx := bytes.Index(cabundle, []byte("-----BEGIN"))
	endIdx := bytes.Index(cabundle, []byte("-----END"))

	if beginIdx >= endIdx {
		return fmt.Errorf("CA bundle has invalid PEM structure (END before BEGIN)")
	}

	return nil
}

func (m *CertManager) propagateCABundle(ctx context.Context, cabundle []byte, gvr schema.GroupVersionResource) error {
	crd, err := crdclient.Get(ctx, m.kube, gvr.GroupResource())
	if err != nil {
		return fmt.Errorf("error getting CRD: %w", err)
	}
	if crd == nil {
		return apierrors.NewNotFound(gvr.GroupResource(), gvr.Resource)
	}

	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Kind:    "CustomResourceDefinition",
		Version: "v1",
	})

	if len(crd.Spec.Versions) > 1 {
		m.log("Updating CA bundle for CRD", "Name", crd.Name)
		err = crdutils.UpdateCABundle(crd, cabundle)
		if err != nil {
			return fmt.Errorf("error updating CA bundle: %w", err)
		}
		// Update the CRD with the new CA bundle
		err = kube.Apply(ctx, m.kube, crd, kube.ApplyOptions{})
		if err != nil {
			return fmt.Errorf("error applying CRD: %w", err)
		}
	}

	// Update the mutating webhook config with the new CA bundle
	mutatingWebhookConfig := admissionregistrationv1.MutatingWebhookConfiguration{}
	err = objects.CreateK8sObject(&mutatingWebhookConfig,
		schema.GroupVersionResource{},
		types.NamespacedName{},
		m.mutatingWebhookTemplatePath,
		"caBundle", base64.StdEncoding.EncodeToString(cabundle))
	if err != nil {
		return fmt.Errorf("error creating mutating webhook config: %w", err)
	}
	m.log("Updating CA bundle for MutatingWebhookConfiguration", "Name", mutatingWebhookConfig.Name)
	err = kube.Apply(ctx, m.kube, &mutatingWebhookConfig, kube.ApplyOptions{})
	if err != nil {
		return fmt.Errorf("error applying mutating webhook config: %w", err)
	}

	return nil
}
