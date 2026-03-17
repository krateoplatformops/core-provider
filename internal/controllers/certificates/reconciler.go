package certificates

import (
	"context"
	"time"

	"github.com/krateoplatformops/core-provider/internal/tools/pluralizer"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CertificateReconciler manages certificate lifecycle independently from the main controller.
// It runs on a periodic schedule to ensure certificates are refreshed and CA bundles are propagated
// to CRDs and Mutating Webhook Configurations.
type CertificateReconciler struct {
	certManager  CertManagerInterface
	pluralizer   pluralizer.PluralizerInterface
	log          logging.Logger
	syncInterval time.Duration
}

// NewCertificateReconciler creates a new CertificateReconciler instance.
func NewCertificateReconciler(
	certManager CertManagerInterface,
	pluralizer pluralizer.PluralizerInterface,
	log logging.Logger,
	syncInterval time.Duration,
) *CertificateReconciler {
	if syncInterval <= 0 {
		syncInterval = 5 * time.Minute // Default sync interval
	}
	return &CertificateReconciler{
		certManager:  certManager,
		pluralizer:   pluralizer,
		log:          log,
		syncInterval: syncInterval,
	}
}

// Start implements the Runnable interface and begins periodic certificate reconciliation.
func (r *CertificateReconciler) Start(ctx context.Context) error {
	r.log.Info("Starting certificate reconciler", "syncInterval", r.syncInterval.String())

	ticker := time.NewTicker(r.syncInterval)
	defer ticker.Stop()

	// Perform initial sync on startup
	r.syncCertificates(ctx)

	for {
		select {
		case <-ctx.Done():
			r.log.Info("Stopping certificate reconciler")
			return nil
		case <-ticker.C:
			r.syncCertificates(ctx)
		}
	}
}

// syncCertificates performs the certificate synchronization for all CompositionDefinitions.
func (r *CertificateReconciler) syncCertificates(ctx context.Context) {
	// Create a timeout for the sync operation to prevent hanging
	syncCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// First, manage certificates globally (check/regenerate if needed)
	err := r.manageCertificatesGlobally(syncCtx)
	if err != nil {
		r.log.Error(err, "error managing certificates globally")
		return
	}

	// Then, update all existing CompositionDefinitions with current CA bundle
	err = r.certManager.UpdateExistingResources(syncCtx)
	if err != nil {
		r.log.Error(err, "error updating existing resources with CA bundle")
		return
	}

	r.log.Debug("certificate synchronization completed")
}

// manageCertificatesGlobally checks and refreshes certificates for the cluster.
// It calls ManageCertificates with the CompositionDefinition GVR as an anchor point.
// This ensures the certificate check/regeneration logic runs periodically, not just during
// CompositionDefinition reconciliation.
func (r *CertificateReconciler) manageCertificatesGlobally(ctx context.Context) error {
	// Use CompositionDefinition as the anchor GVR for certificate management.
	// The GVR is only used to propagate the CA bundle; the cert check/regenerate
	// logic is global and independent of this specific GVR.
	compositionDefinitionGVR := schema.GroupVersionResource{
		Group:    "core.krateo.io",
		Version:  "v1alpha1",
		Resource: "compositiondefinitions",
	}

	err := r.certManager.ManageCertificates(ctx, compositionDefinitionGVR)
	if err != nil {
		return err
	}

	r.log.Debug("certificates verified/regenerated and propagated to anchor resource")
	return nil
}

// SetupWithManager sets up the CertificateReconciler with a controller-runtime manager.
// This is a no-op since the reconciler doesn't watch any resources, but is provided
// for consistency with the controller-runtime interfaces.
func (r *CertificateReconciler) SetupWithManager(mgr interface{}) error {
	return nil
}
