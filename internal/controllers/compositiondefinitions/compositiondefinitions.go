package compositiondefinitions

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
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
	crdtools "github.com/krateoplatformops/core-provider/internal/tools/crd"
	"github.com/krateoplatformops/core-provider/internal/tools/deploy"
	"github.com/krateoplatformops/core-provider/internal/tools/deployment"
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
		Complete(ratelimiter.New(name, r, o.GlobalRateLimiter))
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

	pkg, err := chartfs.ForSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	gvk, err := tools.GroupVersionKind(pkg)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	gvr := tools.ToGroupVersionResource(gvk)
	log.Printf("[DBG] Observing (gvk: %s, gvr: %s)\n", gvk.String(), gvr.String())

	crdOk, err := crdtools.Lookup(ctx, e.kube, gvr)
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

	crd, err := crdtools.Get(ctx, e.kube, gvr)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	log.Printf("[DBG] Searching for Dynamic Controller (gvr: %q)\n", gvr.String())

	obj, err := deployment.CreateDeployment(gvr, types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, os.Getenv(cdcImageTagEnvVar))
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	deployOk, deployReady, err := deployment.LookupDeployment(ctx, e.kube, &obj)
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

	// Sets the status of the CompositionDefinition
	if crd != nil {
		updateVersionInfo(cr, crd)
		cr.Status.Managed.Group = crd.Spec.Group
		cr.Status.Managed.Kind = crd.Spec.Names.Kind
	}
	cr.Status.ApiVersion, cr.Status.Kind = gvk.ToAPIVersionAndKind()
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

	pkg, dir, err := generator.ChartInfoFromSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return err
	}

	gvk, err := generator.ChartGroupVersionKind(pkg, dir)
	if err != nil {
		return err
	}

	gvr := tools.ToGroupVersionResource(gvk)

	crdOk, err := crdtools.Lookup(ctx, e.kube, gvr)
	if err != nil {
		return err
	}

	var crd *apiextensionsv1.CustomResourceDefinition
	if !crdOk {
		crd, err = crdtools.Get(ctx, e.kube, gvr)
		if err != nil {
			return err
		}

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

		newcrd, err := crdtools.Unmarshal(res.Manifest)
		if err != nil {
			return err
		}

		if crd == nil {
			return crdtools.Install(ctx, e.kube, newcrd)
		}

		crd, err = crdtools.AppendVersion(*crd, *newcrd)
		if err != nil {
			return err
		}
		return crdtools.Update(ctx, e.kube, crd.Name, crd)
	} else {
		crd, err = crdtools.Get(ctx, e.kube, gvr)
		if err != nil {
			return err
		}
		if crd == nil {
			return errors.New("CRD not found")
		}

		if meta.IsVerbose(cr) {
			e.log.Debug("CRD alredy generated, checking served resources", "gvr", gvr.String())
		}

		crdtools.SetServedStorage(crd, gvr.Version)

		err = crdtools.Update(ctx, e.kube, crd.Name, crd)
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

	opts := deploy.DeployOptions{
		DiscoveryClient: memory.NewMemCacheClient(e.discovery),
		KubeClient:      e.kube,
		NamespacedName: types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      resourceNamer(gvr.Resource, gvr.Version),
		},
		CDCImageTag: os.Getenv(cdcImageTagEnvVar),
		Spec:        cr.Spec.Chart.DeepCopy(),
	}
	if meta.IsVerbose(cr) {
		opts.Log = e.log.Debug
	}

	err, rbacErr := deploy.Deploy(ctx, e.kube, opts)
	if rbacErr != nil {
		strErr := rbacErr.Error()
		cr.Status.Error = &strErr
		e.log.Info("Error deploying Dynamic Controller", "error", rbacErr.Error())
		cr.SetConditions(rtv1.Unavailable().WithMessage(rbacErr.Error()))
	}
	if err != nil {
		return err
	}

	// Undeploy olders versions of the CRD
	for _, v := range crd.Spec.Versions {
		if !v.Served && !v.Storage {
			err = deploy.Undeploy(ctx, e.kube, deploy.UndeployOptions{
				DiscoveryClient: memory.NewMemCacheClient(e.discovery),
				Spec:            cr.Spec.Chart.DeepCopy(),
				KubeClient:      e.kube,
				GVR:             tools.ToGroupVersionResource(gvk),
				NamespacedName: types.NamespacedName{
					Name:      resourceNamer(gvr.Resource, v.Name),
					Namespace: cr.Namespace,
				},
				SkipCRD: true,
			})

			// err = deployment.UninstallDeployment(ctx, deployment.UninstallOptions{
			// 	KubeClient: e.kube,
			// 	NamespacedName: types.NamespacedName{
			// 		Namespace: cr.Namespace,
			// 		Name:      resourceNamer(gvr.Resource, v.Name),
			// 	},
			// 	Log: e.log.Debug,
			// })
			if err != nil {
				return err
			}
		}

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

	pkg, dir, err := generator.ChartInfoFromSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return err
	}

	gvk, err := generator.ChartGroupVersionKind(pkg, dir)
	if err != nil {
		return err
	}

	gvr := tools.ToGroupVersionResource(gvk)

	opts := deploy.UndeployOptions{
		DiscoveryClient: memory.NewMemCacheClient(e.discovery),
		Spec:            cr.Spec.Chart.DeepCopy(),
		KubeClient:      e.kube,
		GVR:             gvr,
		NamespacedName: types.NamespacedName{
			Name:      resourceNamer(gvr.Resource, gvr.Version),
			Namespace: cr.Namespace,
		},
		SkipCRD: false,
	}
	if meta.IsVerbose(cr) {
		opts.Log = e.log.Debug
	}

	return deploy.Undeploy(ctx, e.kube, opts)
}

func resourceNamer(resourceName string, chartVersion string) string {
	return fmt.Sprintf("%s-%s-controller", resourceName, chartVersion)
}

func updateVersionInfo(cr *compositiondefinitionsv1alpha1.CompositionDefinition, crd *apiextensionsv1.CustomResourceDefinition) {
	for _, v := range crd.Spec.Versions {
		i := -1
		for j, cv := range cr.Status.Managed.VersionInfo {
			if cv.Version == v.Name {
				i = j
				break
			}
		}

		if i == -1 {
			cr.Status.Managed.VersionInfo = append(cr.Status.Managed.VersionInfo, compositiondefinitionsv1alpha1.VersionDetail{
				Version: v.Name,
				Served:  v.Served,
				Stored:  v.Storage,
			})
			continue
		}
		cr.Status.Managed.VersionInfo[i].Served = v.Served
		cr.Status.Managed.VersionInfo[i].Stored = v.Storage
	}
}
