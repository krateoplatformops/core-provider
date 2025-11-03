package compositiondefinitions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/certificates"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/webhooks/conversion"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/webhooks/mutation"
	"github.com/krateoplatformops/core-provider/internal/tools/chart"
	"github.com/krateoplatformops/core-provider/internal/tools/chart/chartfs"
	crdtools "github.com/krateoplatformops/core-provider/internal/tools/crd"
	"github.com/krateoplatformops/core-provider/internal/tools/deploy"
	"github.com/krateoplatformops/core-provider/internal/tools/kube"
	"github.com/krateoplatformops/core-provider/internal/tools/objects"
	pluralizerlib "github.com/krateoplatformops/core-provider/internal/tools/pluralizer"
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"github.com/krateoplatformops/provider-runtime/pkg/controller"
	"github.com/krateoplatformops/provider-runtime/pkg/event"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/meta"
	"github.com/krateoplatformops/provider-runtime/pkg/ratelimiter"
	"github.com/krateoplatformops/provider-runtime/pkg/reconciler"
	"github.com/krateoplatformops/provider-runtime/pkg/resource"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errNotCR                       = "managed resource is not a Definition custom resource"
	reconcileGracePeriod           = 1 * time.Minute
	reconcileTimeout               = 4 * time.Minute
	compositionStillExistFinalizer = "composition.krateo.io/still-exist-compositions-finalizer"
)

var (
	compositionConversionWebhook    = conversion.NewWebhookHandler(runtime.NewScheme())
	CDCtemplateDeploymentPath       = filepath.Join(os.TempDir(), "assets/cdc-deployment/deployment.yaml")
	CDCtemplateConfigmapPath        = filepath.Join(os.TempDir(), "assets/cdc-configmap/configmap.yaml")
	CDCrbacConfigFolder             = filepath.Join(os.TempDir(), "assets/cdc-rbac/")
	MutatingWebhookPath             = filepath.Join(os.TempDir(), "assets/mutating-webhook-configuration/mutating-webhook.yaml")
	JSONSchemaTemplateConfigmapPath = filepath.Join(os.TempDir(), "assets/json-schema-configmap/configmap.yaml")
	ServiceTemplatePath             = filepath.Join(os.TempDir(), "assets/cdc-service/service.yaml")
	CertsPath                       = filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs")
)

type Options struct {
	ControllerOptions controller.Options
	CertManager       *certificates.CertManager
	Pluralizer        pluralizerlib.PluralizerInterface
}

func Setup(mgr ctrl.Manager, o Options) error {
	_ = apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)

	name := reconciler.ControllerName(compositiondefinitionsv1alpha1.CompositionDefinitionGroupKind)

	l := o.ControllerOptions.Logger.WithValues("controller", name)

	recorder := mgr.GetEventRecorderFor(name)
	cli := mgr.GetClient()

	mgr.GetWebhookServer().Register("/mutate", mutation.NewWebhookHandler(cli))
	mgr.GetWebhookServer().Register("/convert", compositionConversionWebhook)

	r := reconciler.NewReconciler(mgr,
		resource.ManagedKind(compositiondefinitionsv1alpha1.CompositionDefinitionGroupVersionKind),
		reconciler.WithExternalConnecter(&connector{
			client:      kubernetes.NewForConfigOrDie(mgr.GetConfig()),
			dynamic:     dynamic.NewForConfigOrDie(mgr.GetConfig()),
			kube:        cli,
			log:         l,
			recorder:    recorder,
			pluralizer:  o.Pluralizer,
			certManager: o.CertManager,
		}),
		reconciler.WithTimeout(reconcileTimeout),
		reconciler.WithPollInterval(o.ControllerOptions.PollInterval),
		reconciler.WithLogger(l),
		reconciler.WithRecorder(event.NewAPIRecorder(recorder)))

	ctx, cancel := context.WithTimeout(context.Background(), reconcileGracePeriod)
	defer cancel()
	err := o.CertManager.UpdateExistingResources(ctx)
	if err != nil {
		return fmt.Errorf("error updating existing resources with CA bundle: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ControllerOptions.ForControllerRuntime()).
		For(&compositiondefinitionsv1alpha1.CompositionDefinition{}).
		Complete(ratelimiter.New(name, r, o.ControllerOptions.GlobalRateLimiter))
}

type connector struct {
	dynamic     dynamic.Interface
	client      kubernetes.Interface
	kube        client.Client
	log         logging.Logger
	recorder    record.EventRecorder
	pluralizer  pluralizerlib.PluralizerInterface
	certManager *certificates.CertManager
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (reconciler.ExternalClient, error) {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return nil, errors.New(errNotCR)
	}

	log := c.log.WithValues("name", cr.Name, "namespace", cr.Namespace)

	return &external{
		kube:        c.kube,
		log:         log,
		dynamic:     c.dynamic,
		client:      c.client,
		rec:         c.recorder,
		pluralizer:  c.pluralizer,
		certManager: c.certManager,
	}, nil
}

type external struct {
	dynamic     dynamic.Interface
	kube        client.Client
	client      kubernetes.Interface
	log         logging.Logger
	rec         record.EventRecorder
	pluralizer  pluralizerlib.PluralizerInterface
	certManager *certificates.CertManager
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (reconciler.ExternalObservation, error) {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return reconciler.ExternalObservation{}, errors.New(errNotCR)
	}
	log := e.log.WithValues("operation", "observe")
	var verboseLogger func(msg string, keysAndValues ...any)
	if meta.IsVerbose(cr) {
		verboseLogger = log.Info // use Info here to have the logs in the events event when debug logs are not enabled
	} else {
		verboseLogger = logging.NewNopLogger().Debug
	}

	if meta.WasDeleted(cr) {
		log.Debug("CompositionDefinition was deleted, skipping observation")
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, e.Delete(ctx, cr)
	}

	log.Info("Observing CompositionDefinition")

	pkgInfo, dir, err := chart.ChartInfoFromSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error getting chart info: %w", err)
	}

	pkg, err := chartfs.ForSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	chartGVK, err := chartfs.GroupVersionKind(pkg)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	gvr, err := e.pluralizer.GVKtoGVR(chartGVK)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// fallback to generate the GVR from the chart info
			bcrd, err := generateCRD(pkg, dir, chartGVK, true)
			if err != nil {
				return reconciler.ExternalObservation{}, fmt.Errorf("error generating CRD for GVR fallback: %w", err)
			}
			crd, err := crdtools.Unmarshal(bcrd)
			if err != nil {
				return reconciler.ExternalObservation{}, fmt.Errorf("error unmarshalling generated CRD for GVR fallback: %w", err)
			}
			gvr = schema.GroupVersionResource{
				Group:    crd.Spec.Group,
				Version:  crd.Spec.Versions[0].Name,
				Resource: crd.Spec.Names.Plural,
			}
		} else {
			return reconciler.ExternalObservation{}, fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, chartGVK.String())
		}
	}

	crd, err := crdtools.Get(ctx, e.kube, gvr.GroupResource())
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error getting CRD: %w", err)
	}
	if crd == nil {
		log.Debug("CRD not found", "gvr", gvr.String())
		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("crd for '%s' does not exists yet", gvr.String())))
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: false,
		}, nil
	}
	existVersion, err := crdtools.Lookup(ctx, e.kube, gvr)
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error looking up existing CRD version: %w", err)
	}
	if !existVersion {
		log.Debug("CRD version not found", "gvr", gvr.String())
		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("crd for '%s' does not exists yet", gvr.String())))
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	err = e.certManager.ManageCertificates(ctx, gvr)
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error managing certificates: %w", err)
	}

	ul, err := getCompositions(ctx, e.dynamic, verboseLogger, gvr)
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error getting compositions: %w", err)
	}
	if len(ul.Items) > 0 {
		if !meta.FinalizerExists(cr, compositionStillExistFinalizer) {
			log.Debug("Adding finalizer to CompositionDefinition", "name", cr.Name)
			meta.AddFinalizer(cr, compositionStillExistFinalizer)
			err = e.kube.Update(ctx, cr)
			if err != nil {
				return reconciler.ExternalObservation{}, err
			}
		}
	}

	log.Debug("Searching for Dynamic Controller", "gvr", gvr)

	obj := appsv1.Deployment{}
	err = objects.CreateK8sObject(&obj, gvr, types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, CDCtemplateDeploymentPath)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	jsonschemaBytes, err := chart.ChartJsonSchemaGetter(pkgInfo, dir).Get()
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error getting JSON schema: %w", err)
	}

	opts := deploy.DeployOptions{
		RBACFolderPath:  CDCrbacConfigFolder,
		DiscoveryClient: memory.NewMemCacheClient(e.client.Discovery()),
		KubeClient:      e.kube,
		NamespacedName: types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      resourceNamer(gvr.Resource, gvr.Version),
		},
		GVR:                    gvr,
		Spec:                   cr.Spec.Chart.DeepCopy(),
		DeploymentTemplatePath: CDCtemplateDeploymentPath,
		ConfigmapTemplatePath:  CDCtemplateConfigmapPath,
		Log:                    verboseLogger,
		JsonSchemaTemplatePath: JSONSchemaTemplateConfigmapPath,
		JsonSchemaBytes:        jsonschemaBytes,
		ServiceTemplatePath:    ServiceTemplatePath,
		DynClient:              e.dynamic,
		DryRunServer:           true,
	}

	dig, err := deploy.Deploy(ctx, e.kube, opts)
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error deploying dynamic controller in dry-run mode: %w", err)
	}

	if cr.Status.Digest != dig {
		log.Debug("Rendered resources digest changed", "status", cr.Status.Digest, "rendered", dig)
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	dig, err = deploy.Lookup(ctx, e.kube, opts)
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error looking up deployed resources digest: %w", err)
	}
	if cr.Status.Digest != dig {
		log.Debug("Deployed resources digest changed", "status", cr.Status.Digest, "deployed", dig)
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	// Sets the status of the CompositionDefinition
	updateVersionInfo(cr, crd, gvr)
	cr.Status.Managed.Group = crd.Spec.Group
	cr.Status.Managed.Kind = crd.Spec.Names.Kind
	cr.Status.ApiVersion, cr.Status.Kind = chartGVK.ToAPIVersionAndKind()
	cr.Status.Resource = gvr.Resource
	cr.Status.PackageURL = pkg.PackageURL()

	cr.SetConditions(rtv1.Available())

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

	log := e.log.WithValues("operation", "create")

	log.Info("Creating CompositionDefinition")

	cr.SetConditions(rtv1.Creating())
	err := e.kube.Status().Update(ctx, cr)
	if err != nil {
		return fmt.Errorf("error updating status: %w", err)
	}

	pkg, dir, err := chart.ChartInfoFromSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return err
	}

	gvk, err := chart.ChartGroupVersionKind(pkg, dir)
	if err != nil {
		return err
	}

	var crd *apiextensionsv1.CustomResourceDefinition
	bcrd, err := generateCRD(pkg, dir, gvk, false)
	if err != nil {
		return fmt.Errorf("error generating CRD: %w", err)
	}
	crd, err = crdtools.Unmarshal(bcrd)
	if err != nil {
		return fmt.Errorf("error unmarshalling generated CRD: %w", err)
	}
	if crd == nil {
		return fmt.Errorf("error generating CRD: crd is nil")
	}

	gvr, err := applyOrUpdateCRD(ctx, e.kube, e.dynamic, crd, e.certManager, log.Debug)
	if err != nil {
		return fmt.Errorf("error applying or updating CRD: %w", err)
	}

	jsonSchemaGetter := chart.ChartJsonSchemaGetter(pkg, dir)
	jsonSchemaBytes, err := jsonSchemaGetter.Get()
	if err != nil {
		return fmt.Errorf("error getting JSON schema: %w", err)
	}
	opts := deploy.DeployOptions{
		RBACFolderPath:  CDCrbacConfigFolder,
		DiscoveryClient: memory.NewMemCacheClient(e.client.Discovery()),
		KubeClient:      e.kube,
		NamespacedName: types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      resourceNamer(gvr.Resource, gvr.Version),
		},
		GVR:                    gvr,
		Spec:                   cr.Spec.Chart.DeepCopy(),
		DeploymentTemplatePath: CDCtemplateDeploymentPath,
		ConfigmapTemplatePath:  CDCtemplateConfigmapPath,
		JsonSchemaTemplatePath: JSONSchemaTemplateConfigmapPath,
		ServiceTemplatePath:    ServiceTemplatePath,
		JsonSchemaBytes:        jsonSchemaBytes,
		DynClient:              e.dynamic,
		Log: func(msg string, keysAndValues ...any) {
			if meta.IsVerbose(cr) {
				log.Debug(msg, keysAndValues...)
			} else {
				logging.NewNopLogger().Debug(msg, keysAndValues...)
			}
		},
	}

	dig, err := deploy.Deploy(ctx, e.kube, opts)
	if err != nil {
		return err
	}

	log.Debug("Dynamic Controller successfully deployed",
		"gvr", gvr.String(),
		"namespace", cr.Namespace,
	)

	cr.Status.Digest = dig

	err = e.kube.Status().Update(ctx, cr)
	if err != nil {
		return err
	}

	return nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return errors.New(errNotCR)
	}

	log := e.log.WithValues("operation", "update")

	verboseLogger := func(msg string, keysAndValues ...any) {
		if meta.IsVerbose(cr) {
			log.Debug(msg, keysAndValues...)
		} else {
			logging.NewNopLogger().Debug(msg, keysAndValues...)
		}
	}

	log.Info("Updating CompositionDefinition")

	cr.SetConditions(rtv1.Condition{
		Type:               rtv1.TypeReady,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             "Updating",
	})
	err := e.kube.Status().Update(ctx, cr)
	if err != nil {
		return fmt.Errorf("error updating status: %w", err)
	}

	pkg, dir, err := chart.ChartInfoFromSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return fmt.Errorf("error getting chart info: %w", err)
	}

	gvk, err := chart.ChartGroupVersionKind(pkg, dir)
	if err != nil {
		return err
	}

	var crd *apiextensionsv1.CustomResourceDefinition
	bcrd, err := generateCRD(pkg, dir, gvk, false)
	if err != nil {
		return fmt.Errorf("error generating CRD: %w", err)
	}
	crd, err = crdtools.Unmarshal(bcrd)
	if err != nil {
		return fmt.Errorf("error unmarshalling generated CRD: %w", err)
	}
	if crd == nil {
		return fmt.Errorf("error generating CRD: crd is nil")
	}

	gvr, err := applyOrUpdateCRD(ctx, e.kube, e.dynamic, crd, e.certManager, log.Debug)
	if err != nil {
		return fmt.Errorf("error applying or updating CRD: %w", err)
	}

	jsonschemaBytes, err := chart.ChartJsonSchemaGetter(pkg, dir).Get()
	if err != nil {
		return fmt.Errorf("error getting JSON schema: %w", err)
	}

	opts := deploy.DeployOptions{
		RBACFolderPath:  CDCrbacConfigFolder,
		DiscoveryClient: memory.NewMemCacheClient(e.client.Discovery()),
		KubeClient:      e.kube,
		NamespacedName: types.NamespacedName{
			Namespace: cr.Namespace,
			Name:      resourceNamer(gvr.Resource, gvr.Version),
		},
		GVR:                    gvr,
		Spec:                   cr.Spec.Chart.DeepCopy(),
		DeploymentTemplatePath: CDCtemplateDeploymentPath,
		ConfigmapTemplatePath:  CDCtemplateConfigmapPath,
		JsonSchemaTemplatePath: JSONSchemaTemplateConfigmapPath,
		ServiceTemplatePath:    ServiceTemplatePath,
		JsonSchemaBytes:        jsonschemaBytes,
		Log:                    verboseLogger,
		DynClient:              e.dynamic,
	}

	dig, err := deploy.Deploy(ctx, e.kube, opts)
	if err != nil {
		return fmt.Errorf("error deploying dynamic controller: %w", err)
	}

	cr.Status.Digest = dig
	err = e.kube.Status().Update(ctx, cr)
	if err != nil {
		return err
	}

	log.Debug("Dynamic Controller successfully updated",
		"gvr", gvr.String(),
		"namespace", cr.Namespace,
	)
	oldGVK := schema.FromAPIVersionAndKind(cr.Status.ApiVersion, cr.Status.Kind)
	oldGVR := oldGVK.GroupVersion().WithResource(cr.Status.Resource)
	// Undeploy olders versions of the CRD
	if oldGVK != gvk {
		for _, vi := range cr.Status.Managed.VersionInfo {
			if oldGVK.Kind == cr.Status.Managed.Kind && oldGVK.Version == vi.Version {
				err = deploy.Undeploy(ctx, e.kube, deploy.UndeployOptions{
					DiscoveryClient:        memory.NewMemCacheClient(e.client.Discovery()),
					RBACFolderPath:         CDCrbacConfigFolder,
					DeploymentTemplatePath: CDCtemplateDeploymentPath,
					ConfigmapTemplatePath:  CDCtemplateConfigmapPath,
					JsonSchemaTemplatePath: JSONSchemaTemplateConfigmapPath,
					ServiceTemplatePath:    ServiceTemplatePath,
					DynamicClient:          e.dynamic,
					Spec:                   (*compositiondefinitionsv1alpha1.ChartInfo)(vi.Chart),
					GVR:                    oldGVR,
					KubeClient:             e.kube,
					NamespacedName: types.NamespacedName{
						Name:      resourceNamer(oldGVR.Resource, oldGVR.Version),
						Namespace: cr.Namespace,
					},
					SkipCRD: true,
					Log:     verboseLogger,
				})
				if err != nil {
					return fmt.Errorf("error undeploying older version of dynamic controller: %w", err)
				}
				log.Debug("Undeployed older versions of dynamic controller", "gvr", oldGVR.String())
			}
		}
	}
	log.Debug("Updating Compositions", "gvr", gvr.String())
	if oldGVK.Version != gvk.Version && cr.Status.Kind == gvk.Kind && oldGVK.Group == gvk.Group {
		err = updateCompositionsVersion(ctx, e.dynamic, verboseLogger, oldGVR, gvk.Version)
		if err != nil {
			return fmt.Errorf("error updating compositions version: %w", err)
		}
		log.Debug("Updated compositions version", "gvr", oldGVR.String())
	}
	cr.Status.ApiVersion, cr.Status.Kind = gvk.ToAPIVersionAndKind()
	cr.Status.Resource = gvr.Resource
	err = e.kube.Status().Update(ctx, cr)
	if err != nil {
		return err
	}

	return nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return errors.New(errNotCR)
	}
	log := e.log.WithValues("operation", "delete")

	var verboseLogger func(msg string, keysAndValues ...any)
	if meta.IsVerbose(cr) {
		verboseLogger = log.Info // use Info here to have the logs in the events event when debug logs are not enabled
	} else {
		verboseLogger = logging.NewNopLogger().Debug
	}

	cr.SetConditions(rtv1.Deleting())
	err := e.kube.Status().Update(ctx, cr)
	if err != nil {
		return fmt.Errorf("error updating status: %w", err)
	}

	pkg, dir, err := chart.ChartInfoFromSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return err
	}

	gvk, err := chart.ChartGroupVersionKind(pkg, dir)
	if err != nil {
		return err
	}

	var gvr schema.GroupVersionResource
	crdExist := true
	gvr, err = e.pluralizer.GVKtoGVR(gvk)
	if apierrors.IsNotFound(err) {
		crdExist = false
		log.Debug("Plural not found, CRD not found, skipping deletion", "gvk", gvk.String())
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, gvk.String())
	}
	if crdExist {
		lst, err := getCompositionDefinitionsWithVersion(ctx, e.kube, schema.GroupKind{
			Group: gvk.Group,
			Kind:  gvk.Kind,
		}, cr.Spec.Chart.Version)
		if err != nil {
			return fmt.Errorf("error getting CompositionDefinitions: %w", err)
		}
		if len(lst) == 1 {
			log.Debug("Deleting Compositions of this version", "gvk", gvk.String())

			// Delete compositions of this version manually
			ul, err := getCompositions(ctx, e.dynamic, verboseLogger, gvr)
			if err != nil {
				return fmt.Errorf("error getting compositions: %w", err)
			}

			for i := range ul.Items {
				log.Debug("Deleting composition", "name", ul.Items[i].GetName(), "namespace", ul.Items[i].GetNamespace())
				err := kube.Uninstall(ctx, e.kube, &ul.Items[i], kube.UninstallOptions{})
				if err != nil {
					return err
				}
			}

			ul, err = getCompositions(ctx, e.dynamic, verboseLogger, gvr)
			if err != nil {
				return fmt.Errorf("error getting compositions: %w", err)
			}
			if len(ul.Items) > 0 {
				return fmt.Errorf("error undeploying CompositionDefinition: waiting for composition deletion")
			}
		}

		var skipCRD bool
		lst, err = getCompositionDefinitions(ctx, e.kube, schema.GroupKind{
			Group: gvk.Group,
			Kind:  gvk.Kind,
		})
		if err != nil {
			return fmt.Errorf("error getting CompositionDefinitions: %w", err)
		}
		if len(lst) > 1 {
			skipCRD = true
			log.Debug("Skipping CRD deletion, other CompositionDefinitions exist", "gvk", gvk.String())
		} else {
			skipCRD = false
			log.Debug("Deleting CRD", "gvk", gvk.String())
		}

		opts := deploy.UndeployOptions{
			DiscoveryClient: memory.NewMemCacheClient(e.client.Discovery()),
			Spec:            cr.Spec.Chart.DeepCopy(),
			KubeClient:      e.kube,
			GVR:             gvr,
			NamespacedName: types.NamespacedName{
				Name:      resourceNamer(gvr.Resource, gvr.Version),
				Namespace: cr.Namespace,
			},
			SkipCRD:                skipCRD,
			DynamicClient:          e.dynamic,
			RBACFolderPath:         CDCrbacConfigFolder,
			DeploymentTemplatePath: CDCtemplateDeploymentPath,
			ServiceTemplatePath:    ServiceTemplatePath,
			ConfigmapTemplatePath:  CDCtemplateConfigmapPath,
			JsonSchemaTemplatePath: JSONSchemaTemplateConfigmapPath,
			Log:                    verboseLogger,
		}

		err = deploy.Undeploy(ctx, e.kube, opts)
		if err != nil {
			if errors.Is(err, deploy.ErrCompositionStillExist) {
				return fmt.Errorf("error undeploying CompositionDefinition: waiting for composition deletion")
			}
			return fmt.Errorf("error undeploying CompositionDefinition: %w", err)

		}
	} else {
		log.Debug("CRD not found, deletion has already been completed", "gvk", gvk.String())
	}

	meta.RemoveFinalizer(cr, compositionStillExistFinalizer)
	return e.kube.Update(ctx, cr)
}
