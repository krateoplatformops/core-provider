package compositiondefinitions

import (
	"context"
	"fmt"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/controllers/certificates"
	"github.com/krateoplatformops/core-provider/internal/tools/chart"
	"github.com/krateoplatformops/core-provider/internal/tools/chart/chartfs"
	crdutils "github.com/krateoplatformops/core-provider/internal/tools/crd/generation"
	pluralizerlib "github.com/krateoplatformops/core-provider/internal/tools/pluralizer"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type certificateTriggerReconciler struct {
	client      client.Client
	log         logging.Logger
	pluralizer  pluralizerlib.PluralizerInterface
	certManager certificates.CertManagerInterface
}

func (r *certificateTriggerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cr := &compositiondefinitionsv1alpha1.CompositionDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error getting CompositionDefinition %s: %w", req.NamespacedName.String(), err)
	}

	gvr, err := r.resolveTargetGVR(ctx, cr)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.certManager.ManageCertificates(ctx, gvr); err != nil {
		return ctrl.Result{}, fmt.Errorf("error managing certificates for %s: %w", gvr.String(), err)
	}

	r.log.Debug("Triggered certificate propagation for CompositionDefinition",
		"name", cr.Name,
		"namespace", cr.Namespace,
		"gvr", gvr.String())

	return ctrl.Result{}, nil
}

func (r *certificateTriggerReconciler) resolveTargetGVR(ctx context.Context, cr *compositiondefinitionsv1alpha1.CompositionDefinition) (schema.GroupVersionResource, error) {
	pkg, err := chartfs.ForSpec(ctx, r.client, cr.Spec.Chart)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	chartGVK, err := chartfs.GroupVersionKind(pkg)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	specSchemaBytes, err := chart.ChartJsonSchema(pkg.FS(), pkg.RootDir())
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("error getting spec schema: %w", err)
	}

	gvr, err := r.pluralizer.GVKtoGVR(chartGVK)
	if err == nil {
		return gvr, nil
	}
	if !apierrors.IsNotFound(err) {
		return schema.GroupVersionResource{}, fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, chartGVK.String())
	}

	gvr, err = crdutils.GetGVRFromGeneratedCRD(specSchemaBytes, chartGVK)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("error getting GVR from generated CRD for GVR fallback: %w", err)
	}

	return gvr, nil
}

func (r *certificateTriggerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("compositiondefinition-certificate-trigger").
		For(&compositiondefinitionsv1alpha1.CompositionDefinition{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		})).
		Complete(r)
}
