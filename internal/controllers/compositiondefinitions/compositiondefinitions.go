package compositiondefinitions

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/generator"
	"github.com/krateoplatformops/core-provider/internal/tools"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"github.com/krateoplatformops/crdgen"
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

	// labelKeyGroup    = "krateo.io/crd-group"
	// labelKeyVersion  = "krateo.io/crd-version"
	// labelKeyResource = "krateo.io/crd-resource"

	reconcileGracePeriod = 1 * time.Minute
	reconcileTimeout     = 4 * time.Minute

	cdcImageTagEnvVar = "CDC_IMAGE_TAG"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	_ = apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)

	name := reconciler.ControllerName(compositiondefinitionsv1alpha1.CompositionDefinitionGroupKind)

	l := o.Logger.WithValues("controller", name)

	recorder := mgr.GetEventRecorderFor(name)

	r := reconciler.NewReconciler(mgr,
		resource.ManagedKind(compositiondefinitionsv1alpha1.CompositionDefinitionGroupVersionKind),
		reconciler.WithExternalConnecter(&connector{
			discovery: discovery.NewDiscoveryClientForConfigOrDie(mgr.GetConfig()),
			kube:      mgr.GetClient(),
			log:       l,
			recorder:  recorder,
		}),
		reconciler.WithTimeout(reconcileTimeout),
		reconciler.WithCreationGracePeriod(reconcileGracePeriod),
		reconciler.WithPollInterval(o.PollInterval),
		reconciler.WithLogger(l),
		reconciler.WithRecorder(event.NewAPIRecorder(recorder)))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&compositiondefinitionsv1alpha1.CompositionDefinition{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	discovery discovery.DiscoveryInterface
	kube      client.Client
	log       logging.Logger
	recorder  record.EventRecorder
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (reconciler.ExternalClient, error) {
	_, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return nil, errors.New(errNotCR)
	}

	if meta.IsVerbose(mg) {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(io.Discard)
	}

	return &external{
		kube:      c.kube,
		log:       c.log,
		discovery: c.discovery,
		rec:       c.recorder,
	}, nil
}

type external struct {
	discovery discovery.DiscoveryInterface
	kube      client.Client
	log       logging.Logger
	rec       record.EventRecorder
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (reconciler.ExternalObservation, error) {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return reconciler.ExternalObservation{}, errors.New(errNotCR)
	}

	pkg, err := chartfs.ForSpec(cr.Spec.Chart)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	gvk, err := tools.GroupVersionKind(pkg)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	gvr := tools.ToGroupVersionResource(gvk)
	log.Printf("[DBG] Observing (gvk: %s, gvr: %s)\n", gvk.String(), gvr.String())

	crdOk, err := tools.LookupCRD(ctx, e.kube, gvr)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	if !crdOk {
		log.Printf("[DBG] CRD does not exists yet (gvr: %q)\n", gvr.String())

		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("CRD for '%s' does not exists yet", gvr.String())))
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}

	log.Printf("[DBG] Searching for Dynamic Controller (gvr: %q)\n", gvr.String())

	obj, err := tools.CreateDeployment(gvr, types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, os.Getenv(cdcImageTagEnvVar))
	if err != nil {
		return reconciler.ExternalObservation{}, err
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

	cr.Status.APIVersion, cr.Status.Kind = gvk.ToAPIVersionAndKind()
	cr.Status.PackageURL = pkg.PackageURL()

	if !deployReady {
		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("Dynamic Controller '%s' not ready yet", obj.Name)))

		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, nil
	}

	if cr.Status.Error != nil {
		cr.SetConditions(rtv1.Unavailable().WithMessage(*cr.Status.Error))
	} else {
		cr.SetConditions(rtv1.Available())
	}

	return reconciler.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return errors.New(errNotCR)
	}

	if !meta.IsActionAllowed(cr, meta.ActionCreate) {
		e.log.Debug("External resource should not be updated by provider, skip creating.")
		return nil
	}

	pkg, dir, err := generator.ChartInfoFromSpec(ctx, cr.Spec.Chart)
	if err != nil {
		return err
	}

	gvk, err := generator.ChartGroupVersionKind(pkg, dir)
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

		res := crdgen.Generate(ctx, crdgen.Options{
			Managed:              true,
			WorkDir:              dir,
			GVK:                  gvk,
			Categories:           []string{"compositions", "comps"},
			SpecJsonSchemaGetter: generator.ChartJsonSchemaGetter(pkg, dir),
		})
		if res.Err != nil {
			return res.Err
		}

		crd, err := tools.UnmarshalCRD(res.Manifest)
		if err != nil {
			return err
		}

		return tools.InstallCRD(ctx, e.kube, crd)
	}

	if meta.IsVerbose(cr) {
		e.log.Debug("CRD alredy generated", "gvr", gvr.String())
	}

	// err = e.kube.Update(ctx, cr, &client.UpdateOptions{})
	// if err != nil {
	// 	return err
	// }

	if meta.IsVerbose(cr) {
		e.log.Debug("Deploying Dynamic Controller",
			"gvr", gvr.String(),
			"namespace", cr.Namespace,
		)
	}

	opts := tools.DeployOptions{
		DiscoveryClient: e.discovery,
		KubeClient:      e.kube,
		NamespacedName: types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      cr.Name,
		},
		CDCImageTag: os.Getenv(cdcImageTagEnvVar),
		Spec:        cr.Spec.Chart.DeepCopy(),
	}
	if meta.IsVerbose(cr) {
		opts.Log = e.log.Debug
	}

	err, rbacErr := tools.Deploy(ctx, opts)
	if rbacErr != nil {
		strErr := rbacErr.Error()
		cr.Status.Error = &strErr
		e.log.Info("Error deploying Dynamic Controller", "error", rbacErr.Error())
		cr.SetConditions(rtv1.Unavailable().WithMessage(rbacErr.Error()))
	}
	if err != nil {
		return err
	}

	err = e.kube.Status().Update(ctx, cr)
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
	return nil // NOOP
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return errors.New(errNotCR)
	}

	if !meta.IsActionAllowed(cr, meta.ActionDelete) {
		e.log.Debug("External resource should not be deleted by provider, skip deleting.")
		return nil
	}

	pkg, dir, err := generator.ChartInfoFromSpec(ctx, cr.Spec.Chart)
	if err != nil {
		return err
	}

	gvk, err := generator.ChartGroupVersionKind(pkg, dir)
	if err != nil {
		return err
	}

	opts := tools.UndeployOptions{
		KubeClient: e.kube,
		GVR:        tools.ToGroupVersionResource(gvk),
		NamespacedName: types.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
	}
	if meta.IsVerbose(cr) {
		opts.Log = e.log.Debug
	}

	return tools.Undeploy(ctx, opts)
}
