package definitions

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/definitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator"
	"github.com/krateoplatformops/core-provider/internal/tools"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
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

	labelKeyGroup    = "krateo.io/crd-group"
	labelKeyVersion  = "krateo.io/crd-version"
	labelKeyResource = "krateo.io/crd-resource"
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

	pkg, err := chartfs.FromURL(cr.Spec.ChartUrl)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	gvr, err := tools.GroupVersionResource(pkg)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	exists, err := tools.LookupCRD(ctx, e.kube, gvr)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	if !exists {
		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("CRD for '%s' does not exists yet", gvr.String())))
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, e.kube.Status().Update(ctx, cr)
	}

	if meta.ExternalCreateIncomplete(cr) {
		e.log.Info("CRD generation pending.", "gvr", gvr.String())
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, nil
	}

	if meta.IsVerbose(cr) {
		e.log.Debug("Searching for Dynamic Controller", "gvr", gvr.String())
	}

	obj, err := tools.CreateDeployment(gvr, cr.Namespace)
	if err != nil {
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, err
	}

	deployed, err := tools.LookupDeployment(ctx, e.kube, &obj)
	if err != nil {
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, err
	}

	if !deployed {
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil // e.kube.Update(ctx, cr)
	}

	if meta.IsVerbose(cr) {
		e.log.Debug("Dynamic Controller already deployed", "gvr", gvr.String())
	}

	cr.SetConditions(rtv1.Available())

	return reconciler.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil // e.kube.Update(ctx, cr)
}

func (e *external) Create(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*definitionsv1alpha1.Definition)
	if !ok {
		return errors.New(errNotCR)
	}

	if !meta.IsActionAllowed(cr, meta.ActionCreate) {
		e.log.Debug("External resource should not be updated by provider, skip creating.")
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

	crd, err := tools.UnmarshalCRD(dat)
	if err != nil {
		return err
	}

	if err := tools.InstallCRD(ctx, e.kube, crd); err != nil {
		return err
	}

	return nil // e.kube.Status().Update(ctx, cr)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*definitionsv1alpha1.Definition)
	if !ok {
		return errors.New(errNotCR)
	}

	pkg, err := chartfs.FromURL(cr.Spec.ChartUrl)
	if err != nil {
		return err
	}

	gvr, err := tools.GroupVersionResource(pkg)
	if err != nil {
		return err
	}

	if cr.Labels == nil {
		cr.Labels = make(map[string]string)
	}

	cr.Labels[labelKeyGroup] = gvr.Group
	cr.Labels[labelKeyVersion] = gvr.Version
	cr.Labels[labelKeyResource] = gvr.Resource

	if err := e.kube.Update(ctx, cr, &client.UpdateOptions{}); err != nil {
		return err
	}

	if meta.IsVerbose(cr) {
		e.log.Debug("Deploying Dynamic Controller",
			"gvr", gvr.String(),
			"namespace", cr.Namespace,
		)
	}

	err = tools.Deploy(ctx, e.kube, tools.DeployOptions{
		Namespace: cr.Namespace,
		ChartFS:   pkg,
	})
	if err != nil {
		return err
	}

	if meta.IsVerbose(cr) {
		e.log.Debug("Dynamic Controller successfully deployed",
			"gvr", gvr.String(),
			"namespace", cr.Namespace,
		)
	}

	return nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	return nil // NOOP
}
