package definitions

import (
	"context"
	"strings"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/gobuffalo/flect"
	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/definitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator"
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"github.com/krateoplatformops/provider-runtime/pkg/controller"
	"github.com/krateoplatformops/provider-runtime/pkg/event"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/meta"
	"github.com/krateoplatformops/provider-runtime/pkg/ratelimiter"
	"github.com/krateoplatformops/provider-runtime/pkg/reconciler"
	"github.com/krateoplatformops/provider-runtime/pkg/resource"
	"github.com/pkg/errors"
)

const (
	errNotCR = "managed resource is not a Definition custom resource"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	_ = apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)

	name := reconciler.ControllerName(definitionsv1alpha1.DefinitionGroupKind)

	log := o.Logger.WithValues("controller", name)

	recorder := mgr.GetEventRecorderFor(name)

	r := reconciler.NewReconciler(mgr,
		resource.ManagedKind(definitionsv1alpha1.DefinitionGroupVersionKind),
		reconciler.WithExternalConnecter(&connector{
			kube:     mgr.GetClient(),
			log:      log,
			recorder: recorder,
		}),
		reconciler.WithPollInterval(o.PollInterval),
		reconciler.WithLogger(log),
		reconciler.WithRecorder(event.NewAPIRecorder(recorder)))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&definitionsv1alpha1.Definition{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube     client.Client
	log      logging.Logger
	recorder record.EventRecorder
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (reconciler.ExternalClient, error) {
	_, ok := mg.(*definitionsv1alpha1.Definition)
	if !ok {
		return nil, errors.New(errNotCR)
	}

	return &external{
		kube: c.kube,
		log:  c.log,

		rec: c.recorder,
	}, nil
}

type external struct {
	kube client.Client
	log  logging.Logger
	rec  record.EventRecorder
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (reconciler.ExternalObservation, error) {
	cr, ok := mg.(*definitionsv1alpha1.Definition)
	if !ok {
		return reconciler.ExternalObservation{}, errors.New(errNotCR)
	}

	gvk, err := generator.GroupVersionKindFromTarGzipURL(ctx, cr.Spec.ChartUrl)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	exists, err := lookupCRD(ctx, e.kube, gvk)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	if !exists {
		cr.SetConditions(rtv1.Unavailable())
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, e.kube.Status().Update(ctx, cr)
	}

	if exists {
		labels := cr.Labels
		if len(labels) > 0 {
			if cr.Labels == nil {
				cr.Labels = make(map[string]string)
			}
			cr.Labels["krateo.io/crd-group-version"] = gvk.GroupVersion().String()
			cr.Labels["krateo.io/crd-kind"] = gvk.Kind
			cr.Labels["krateo.io/crd-resource"] = strings.ToLower(flect.Pluralize(gvk.Kind))
		}

		if meta.ExternalCreateIncomplete(cr) {
			e.log.Info("Generating CRD", "gvk", gvk.String())
			return reconciler.ExternalObservation{
				ResourceExists:   true,
				ResourceUpToDate: true,
			}, nil
		}
	}

	cr.SetConditions(rtv1.Available())

	return reconciler.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil //e.kube.Status().Update(ctx, cr)
}

func (e *external) Create(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*definitionsv1alpha1.Definition)
	if !ok {
		return errors.New(errNotCR)
	}

	if !meta.IsActionAllowed(cr, meta.ActionUpdate) {
		e.log.Debug("External resource should not be updated by provider, skip updating.")
		return nil
	}

	cr.SetConditions(rtv1.Creating())

	gen, err := generator.ForTarGzipURL(ctx, cr.Spec.ChartUrl)
	if err != nil {
		return err
	}

	dat, err := gen.Generate(ctx)
	if err != nil {
		return err
	}

	crd, err := unmarshalCRD(dat)
	if err != nil {
		return err
	}

	if err := installCRD(ctx, e.kube, crd); err != nil {
		return err
	}

	return e.kube.Status().Update(ctx, cr)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) error {
	return nil // NOOP
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	return nil // NOOP
}
