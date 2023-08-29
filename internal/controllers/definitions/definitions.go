package definitions

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	reconcileGracePeriod = 1 * time.Minute
	reconcileTimeout     = 4 * time.Minute
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
		reconciler.WithTimeout(reconcileTimeout),
		reconciler.WithCreationGracePeriod(reconcileGracePeriod),
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

	pkg, err := chartfs.ForSpec(cr.Spec.Chart)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	gvr, err := tools.GroupVersionResource(pkg)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	crdOk, err := tools.LookupCRD(ctx, e.kube, gvr)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	if !crdOk {
		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("CRD for '%s' does not exists yet", gvr.String())))
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}

	cr.Status.Resource = fmt.Sprintf("%s.%s", gvr.Resource, gvr.GroupVersion().String())

	// if meta.ExternalCreateIncomplete(cr) {
	// 	e.log.Info("CRD generation pending.", "gvr", gvr.String())
	// 	return reconciler.ExternalObservation{
	// 		ResourceExists:   true,
	// 		ResourceUpToDate: true,
	// 	}, nil
	// }

	if meta.IsVerbose(cr) {
		e.log.Debug("Searching for Dynamic Controller", "gvr", gvr.String())
	}

	obj, err := tools.CreateDeployment(gvr, types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      fmt.Sprintf("%s-%s-controller", gvr.Resource, gvr.Version),
	})
	if err != nil {
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, err
	}

	deployOk, deployReady, err := tools.LookupDeployment(ctx, e.kube, &obj)
	if err != nil {
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, err
	}

	if !deployOk {
		if meta.IsVerbose(cr) {
			e.log.Debug("Dynamic Controller not deployed yet",
				"name", obj.Name, "namespace", obj.Namespace, "gvr", gvr.String())
		}

		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("Dynamic Controller '%s' not deployed yet", obj.Name)))

		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}

	if meta.IsVerbose(cr) {
		e.log.Debug("Dynamic Controller already deployed",
			"name", obj.Name, "namespace", obj.Namespace,
			"gvr", gvr.String())
	}

	cr.Status.PackageURL = pkg.PackageURL()

	if !deployReady {
		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("Dynamic Controller '%s' not ready yet", obj.Name)))

		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, nil
	}

	cr.SetConditions(rtv1.Available())

	return reconciler.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
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

	gen, err := generator.ForSpec(ctx, cr.Spec.Chart)
	if err != nil {
		return err
	}

	gvk, err := gen.GVK()
	if err != nil {
		return err
	}

	gvr := tools.ToGroupVersionResource(gvk)
	crdOk, err := tools.LookupCRD(ctx, e.kube, gvr)
	if err != nil {
		return err
	}

	if !crdOk {
		if meta.IsVerbose(cr) {
			e.log.Debug("Generating CRD", "gvr", gvr.String())
		}

		cr.SetConditions(rtv1.Condition{
			Type:               rtv1.TypeReady,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "GeneratingCRD",
			Message:            fmt.Sprintf("Generating CRD for: %s", gvr),
		})

		dat, err := gen.Generate(ctx)
		if err != nil {
			return err
		}

		crd, err := tools.UnmarshalCRD(dat)
		if err != nil {
			return err
		}

		return tools.InstallCRD(ctx, e.kube, crd)
	}

	if meta.IsVerbose(cr) {
		e.log.Debug("CRD alredy generated", "gvr", gvr.String())
	}

	if cr.Labels == nil {
		cr.Labels = make(map[string]string)
	}

	dirty := false
	if _, ok := cr.Labels[labelKeyGroup]; !ok {
		dirty = true
		cr.Labels[labelKeyGroup] = gvr.Group
	}

	if _, ok := cr.Labels[labelKeyVersion]; !ok {
		dirty = true
		cr.Labels[labelKeyVersion] = gvr.Version
	}

	if _, ok := cr.Labels[labelKeyResource]; !ok {
		dirty = true
		cr.Labels[labelKeyResource] = gvr.Resource
	}

	if dirty {
		err := e.kube.Update(ctx, cr, &client.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	if meta.IsVerbose(cr) {
		e.log.Debug("Deploying Dynamic Controller",
			"gvr", gvr.String(),
			"namespace", cr.Namespace,
		)
	}

	err = tools.Deploy(ctx, e.kube, tools.DeployOptions{
		Namespace: cr.Namespace,
		Spec:      cr.Spec.Chart.DeepCopy(),
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

func (e *external) Update(ctx context.Context, mg resource.Managed) error {
	// cr, ok := mg.(*definitionsv1alpha1.Definition)
	// if !ok {
	// 	return errors.New(errNotCR)
	// }

	return nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	return nil // TODO(@lucasepe): should be the related dynamic controlled removed?
}
