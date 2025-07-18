package compositiondefinitions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/webhooks/conversion"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/webhooks/mutation"
	"github.com/krateoplatformops/core-provider/internal/controllers/logger"
	"github.com/krateoplatformops/core-provider/internal/tools/certs"
	"github.com/krateoplatformops/core-provider/internal/tools/chart"
	"github.com/krateoplatformops/core-provider/internal/tools/chart/chartfs"
	crdtools "github.com/krateoplatformops/core-provider/internal/tools/crd"
	"github.com/krateoplatformops/core-provider/internal/tools/deploy"
	"github.com/krateoplatformops/core-provider/internal/tools/deployment"
	"github.com/krateoplatformops/core-provider/internal/tools/kube"
	"github.com/krateoplatformops/core-provider/internal/tools/objects"
	"github.com/krateoplatformops/crdgen"
	"github.com/krateoplatformops/plumbing/kubeutil/plurals"
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
	WebhookServiceName      string
	WebhookServiceNamespace string
	CertOpts                certs.GenerateClientCertAndKeyOpts
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

func GetCABundle() ([]byte, error) {
	fb, err := os.ReadFile(filepath.Join(CertsPath, "tls.crt"))
	if err != nil {
		return nil, err
	}

	return fb, nil
}

func Setup(mgr ctrl.Manager, o controller.Options) error {
	_ = apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)

	name := reconciler.ControllerName(compositiondefinitionsv1alpha1.CompositionDefinitionGroupKind)

	l := o.Logger.WithValues("controller", name)

	recorder := mgr.GetEventRecorderFor(name)
	cli := mgr.GetClient()

	mgr.GetWebhookServer().Register("/mutate", mutation.NewWebhookHandler(cli))
	mgr.GetWebhookServer().Register("/convert", compositionConversionWebhook)

	r := reconciler.NewReconciler(mgr,
		resource.ManagedKind(compositiondefinitionsv1alpha1.CompositionDefinitionGroupVersionKind),
		reconciler.WithExternalConnecter(&connector{
			client:   kubernetes.NewForConfigOrDie(mgr.GetConfig()),
			dynamic:  dynamic.NewForConfigOrDie(mgr.GetConfig()),
			kube:     cli,
			log:      l,
			recorder: recorder,
		}),
		reconciler.WithTimeout(reconcileTimeout),
		reconciler.WithCreationGracePeriod(reconcileGracePeriod),
		reconciler.WithPollInterval(o.PollInterval),
		reconciler.WithLogger(l),
		reconciler.WithRecorder(event.NewAPIRecorder(recorder)))

	// Setup any crds and webhooks with cabundle
	cabundle, err := GetCABundle()
	if err != nil {
		return fmt.Errorf("error getting CA bundle: %w", err)
	}

	// client from the manager is not ready yet, so we need to create a new one (cache is not ready)
	cliNow, err := client.New(mgr.GetConfig(), client.Options{})
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}
	var cdList compositiondefinitionsv1alpha1.CompositionDefinitionList
	err = cliNow.List(context.Background(), &cdList, &client.ListOptions{Namespace: metav1.NamespaceAll})
	if err != nil {
		return fmt.Errorf("error listing CompositionDefinitions: %s", err)
	}
	for i := range cdList.Items {
		cd := &cdList.Items[i]
		if cd.Status.ApiVersion != "" && cd.Status.Kind != "" {
			gvk := schema.FromAPIVersionAndKind(cd.Status.ApiVersion, cd.Status.Kind)
			pluralInfo, err := plurals.Get(gvk, plurals.GetOptions{})
			if err != nil {
				return fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, gvk.String())
			}
			gvr := schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: pluralInfo.Plural,
			}

			err = propagateCABundle(context.Background(), cliNow, cabundle, gvr, l.Debug)
			if err != nil {
				return fmt.Errorf("error updating CA bundle: %w", err)
			}
			l.Info("Updated CA bundle for CRD and MutatingWebhookConfiguration", "Name", gvr.String())
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&compositiondefinitionsv1alpha1.CompositionDefinition{}).
		Complete(ratelimiter.New(name, r, o.GlobalRateLimiter))

}

type connector struct {
	dynamic  dynamic.Interface
	client   kubernetes.Interface
	kube     client.Client
	log      logging.Logger
	recorder record.EventRecorder
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (reconciler.ExternalClient, error) {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return nil, errors.New(errNotCR)
	}

	log := logger.Logger{
		Verbose: meta.IsVerbose(cr),
		Logger:  c.log,
	}

	return &external{
		kube:    c.kube,
		log:     &log,
		dynamic: c.dynamic,
		client:  c.client,
		rec:     c.recorder,
	}, nil
}

type external struct {
	dynamic dynamic.Interface
	kube    client.Client
	client  kubernetes.Interface
	log     logging.Logger
	rec     record.EventRecorder
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (reconciler.ExternalObservation, error) {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return reconciler.ExternalObservation{}, errors.New(errNotCR)
	}

	if meta.WasDeleted(cr) {
		e.log.Debug("CompositionDefinition was deleted, skipping observation")
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, e.Delete(ctx, cr)
	}

	pkg, err := chartfs.ForSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	gvk, err := chartfs.GroupVersionKind(pkg)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	crdOk := true

	pluralInfo, err := plurals.Get(gvk, plurals.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			crdOk = false
		} else {
			return reconciler.ExternalObservation{}, fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, gvk.String())
		}
	}
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: pluralInfo.Plural,
	}

	if crdOk {
		crdOk, err = crdtools.Lookup(ctx, e.kube, gvr)
		if err != nil {
			return reconciler.ExternalObservation{}, err
		}
	}
	e.log.Info("Observing", "gvk", gvk.String())

	if !crdOk {
		e.log.Info("CRD not found, waiting for CRD to be created", "gvk", gvk.String())
		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("crd for '%s' does not exists yet", gvk.String())))
		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}

	ok, cert, key, err := certs.CheckOrRegenerateClientCertAndKey(e.client, e.log.Debug, CertOpts)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}
	if !ok {
		e.log.Info("Certificate has been regenerated, updating certificates for webhook server")
		err = certs.UpdateCerts(cert, key, CertsPath)
		if err != nil {
			return reconciler.ExternalObservation{}, err
		}
		e.log.Info("Updating certficates for CRDs and Mutating Webhook Configurations")
	}
	cabundle, err := GetCABundle()
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error getting CA bundle: %w", err)
	}
	err = propagateCABundle(ctx,
		e.kube,
		cabundle,
		gvr,
		e.log.Debug)
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error updating CA bundle: %w", err)
	}

	ul, err := getCompositions(ctx, e.dynamic, e.log.Debug, gvr)
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error getting compositions: %w", err)
	}
	if len(ul.Items) > 0 {
		if !meta.FinalizerExists(cr, compositionStillExistFinalizer) {
			e.log.Debug("Adding finalizer to CompositionDefinition", "name", cr.Name)
			meta.AddFinalizer(cr, compositionStillExistFinalizer)
			err = e.kube.Update(ctx, cr)
			if err != nil {
				return reconciler.ExternalObservation{}, err
			}
		}
	}

	e.log.Info("Searching for Dynamic Controller", "gvr", gvr)

	obj := appsv1.Deployment{}
	err = objects.CreateK8sObject(&obj, gvr, types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}, CDCtemplateDeploymentPath)
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
		e.log.Debug("Dynamic Controller not deployed yet",
			"name", obj.Name, "namespace", obj.Namespace, "gvr", gvr.String())

		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("Dynamic Controller '%s' not deployed yet", obj.Name)))

		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}

	e.log.Debug("Dynamic Controller already deployed",
		"name", obj.Name, "namespace", obj.Namespace,
		"gvr", gvr.String())

	if !deployReady {
		cr.SetConditions(rtv1.Unavailable().
			WithMessage(fmt.Sprintf("Dynamic Controller '%s' not ready yet", obj.Name)))

		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, nil
	}

	// if version is different, Update
	oldGVK := schema.FromAPIVersionAndKind(cr.Status.ApiVersion, cr.Status.Kind)
	if oldGVK.Version != gvk.Version && cr.Status.Kind == gvk.Kind && oldGVK.Group == gvk.Group {
		e.log.Info("Updating CompositionDefinition GVK", "old", oldGVK, "new", gvk)
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}
	crd, err := crdtools.Get(ctx, e.kube, gvr)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	// Sets the status of the CompositionDefinition
	if crd != nil {
		updateVersionInfo(cr, crd, gvr)
		cr.Status.Managed.Group = crd.Spec.Group
		cr.Status.Managed.Kind = crd.Spec.Names.Kind
	}
	cr.Status.ApiVersion, cr.Status.Kind = gvk.ToAPIVersionAndKind()
	cr.Status.Resource = gvr.Resource
	cr.Status.PackageURL = pkg.PackageURL()

	cr.SetConditions(rtv1.Available())

	pkgInfo, dir, err := chart.ChartInfoFromSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return reconciler.ExternalObservation{}, fmt.Errorf("error getting chart info: %w", err)
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
		Log:                    e.log.Debug,
		JsonSchemaTemplatePath: JSONSchemaTemplateConfigmapPath,
		JsonSchemaBytes:        jsonschemaBytes,
		ServiceTemplatePath:    ServiceTemplatePath,
		DryRunServer:           true,
	}

	dig, err := deploy.Deploy(ctx, e.kube, opts)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	if cr.Status.Digest != dig {
		e.log.Info("Rendered resources digest changed", "status", cr.Status.Digest, "rendered", dig)
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	dig, err = deploy.Lookup(ctx, e.kube, opts)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}
	if cr.Status.Digest != dig {
		e.log.Info("Deployed resources digest changed", "status", cr.Status.Digest, "deployed", dig)
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
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
		e.log.Info("External resource should not be updated by provider, skip creating.")
		return nil
	}

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

	crdOk := true
	pluralInfo, err := plurals.Get(gvk, plurals.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			crdOk = false
		} else {
			return fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, gvk.String())
		}
	}
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: pluralInfo.Plural,
	}

	if crdOk {
		crdOk, err = crdtools.Lookup(ctx, e.kube, gvr)
		if err != nil {
			return err
		}
	}
	jsonSchemaGetter := chart.ChartJsonSchemaGetter(pkg, dir)

	var crd *apiextensionsv1.CustomResourceDefinition
	if !crdOk {
		pluralgvk := schema.FromAPIVersionAndKind(cr.Status.ApiVersion, cr.Status.Kind)

		if pluralgvk.Version == "" || pluralgvk.Group == "" || pluralgvk.Kind == "" {
			lst, err := getCompositionDefinitions(ctx, e.kube, schema.GroupKind{
				Group: gvr.Group,
				Kind:  gvk.Kind,
			})
			if err != nil {
				return fmt.Errorf("error getting CompositionDefinitions: %w", err)
			}
			if len(lst) > 0 {
				// range until you find the first compositiondefiniton with A non-empty GVK in the status
				// and use that as the GVK for the CRD
				for i := range lst {
					cd := &lst[i]
					if cd.Status.ApiVersion != "" && cd.Status.Kind != "" {
						pluralgvk = schema.FromAPIVersionAndKind(cd.Status.ApiVersion, cd.Status.Kind)
						break
					}
				}

				if pluralgvk.Version == "" || pluralgvk.Group == "" || pluralgvk.Kind == "" {
					return fmt.Errorf("error getting GVK from CompositionDefinition: %s", cr.Name)
				}
			} else {
				pluralgvk = gvk
				e.log.Debug("CompositionDefinition not found, using default GVK", "gvk", pluralgvk.String())
			}
		}
		pluralInfo, err := plurals.Get(pluralgvk, plurals.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e.log.Debug("Plural not found, using default GVK", "gvk", pluralgvk.String())
			} else {
				return fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, gvk.String())
			}
		}
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: pluralInfo.Plural,
		}

		gvr.Version = gvk.Version

		crd, err = crdtools.Get(ctx, e.kube, gvr)
		if err != nil {
			return err
		}

		e.log.Debug("Generating CRD", "gvr", gvr.String())

		cr.SetConditions(rtv1.Condition{
			Type:               rtv1.TypeReady,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "GeneratingCRD",
			Message:            fmt.Sprintf("Generating CRD for: %s", gvr),
		})
		res := crdgen.Generate(ctx, crdgen.Options{
			Managed:                true,
			WorkDir:                dir,
			GVK:                    gvk,
			Categories:             []string{"compositions", "comps"},
			SpecJsonSchemaGetter:   jsonSchemaGetter,
			StatusJsonSchemaGetter: StaticJsonSchemaGetter(),
		})
		if res.Err != nil {
			return res.Err
		}

		newcrd, err := crdtools.Unmarshal(res.Manifest)
		if err != nil {
			return err
		}

		if crd == nil {
			e.log.Debug("CRD not found, installing new CRD", "gvr", gvk.String())
			return kube.Apply(ctx, e.kube, newcrd, kube.ApplyOptions{})
		}

		crd, err = crdtools.AppendVersion(*crd, *newcrd)
		if err != nil {
			return err
		}

		whport := int32(9443)
		whpath := "/convert"

		cabundle, err := GetCABundle()
		if err != nil {
			return fmt.Errorf("error getting CA bundle: %w", err)
		}
		crd = crdtools.ConversionConf(*crd, &apiextensionsv1.CustomResourceConversion{
			Strategy: apiextensionsv1.WebhookConverter,
			Webhook: &apiextensionsv1.WebhookConversion{
				ConversionReviewVersions: []string{"v1", "v1alpha1", "v1alpha2"},
				ClientConfig: &apiextensionsv1.WebhookClientConfig{
					Service: &apiextensionsv1.ServiceReference{
						Namespace: WebhookServiceNamespace,
						Name:      WebhookServiceName,
						Port:      &whport,
						Path:      &whpath,
					},
					CABundle: cabundle,
				},
			},
		})
		return kube.Apply(ctx, e.kube, crd, kube.ApplyOptions{})
	} else {
		crd, err = crdtools.Get(ctx, e.kube, gvr)
		if err != nil {
			return err
		}
		if crd == nil {
			return errors.New("CRD not found")
		}

		if meta.IsVerbose(cr) {
			e.log.Debug("CRD already generated, checking served resources", "gvr", gvr.String())
		}

		err = kube.Apply(ctx, e.kube, crd, kube.ApplyOptions{})
		if err != nil {
			return err
		}
	}

	jsonSchemaBytes, err := jsonSchemaGetter.Get()
	if err != nil {
		return fmt.Errorf("error getting JSON schema: %w", err)
	}
	e.log.Debug("JSON schema ConfigMap created", "gvr", gvr.String(), "namespace", cr.Namespace)

	e.log.Debug("Deploying Dynamic Controller",
		"gvr", gvr.String(),
		"namespace", cr.Namespace,
	)

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
		Log:                    e.log.Debug,
	}

	dig, err := deploy.Deploy(ctx, e.kube, opts)
	if err != nil {
		return err
	}

	cr.Status.Digest = dig

	err = e.kube.Status().Update(ctx, cr)
	if err != nil {
		return err
	}

	e.log.Debug("Dynamic Controller successfully deployed",
		"gvr", gvr.String(),
		"namespace", cr.Namespace,
	)

	return nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*compositiondefinitionsv1alpha1.CompositionDefinition)
	if !ok {
		return errors.New(errNotCR)
	}

	if !meta.IsActionAllowed(cr, meta.ActionUpdate) {
		e.log.Info("External resource should not be updated by provider, skip updating.")
		return nil
	}

	e.log.Debug("Updating CompositionDefinition", "name", cr.Name)

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

	oldGVK := schema.FromAPIVersionAndKind(cr.Status.ApiVersion, cr.Status.Kind)

	pluralInfo, err := plurals.Get(oldGVK, plurals.GetOptions{})
	if err != nil {
		return fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, gvk.String())
	}
	oldGVR := schema.GroupVersionResource{
		Group:    oldGVK.Group,
		Version:  oldGVK.Version,
		Resource: pluralInfo.Plural,
	}
	gvr := oldGVR
	gvr.Version = gvk.Version

	crd, err := crdtools.Get(ctx, e.kube, gvr)
	if err != nil {
		return err
	}

	e.log.Debug("Updating Compositions", "gvr", gvr.String())

	pkgInfo, dir, err := chart.ChartInfoFromSpec(ctx, e.kube, cr.Spec.Chart)
	if err != nil {
		return fmt.Errorf("error getting chart info: %w", err)
	}
	jsonschemaBytes, err := chart.ChartJsonSchemaGetter(pkgInfo, dir).Get()
	if err != nil {
		return fmt.Errorf("error getting JSON schema: %w", err)
	}

	if oldGVK.Version == gvk.Version {
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
			Log:                    e.log.Debug,
			JsonSchemaTemplatePath: JSONSchemaTemplateConfigmapPath,
			ServiceTemplatePath:    ServiceTemplatePath,
			JsonSchemaBytes:        jsonschemaBytes,
		}

		dig, err := deploy.Deploy(ctx, e.kube, opts)
		if err != nil {
			return err
		}

		cr.Status.Digest = dig

		err = e.kube.Status().Update(ctx, cr)
		if err != nil {
			return err
		}

		e.log.Debug("Dynamic Controller successfully updated",
			"gvr", gvr.String(),
			"namespace", cr.Namespace,
		)

		return nil
	}

	e.log.Debug("Updating from GVK", "old", oldGVK, "new", gvk)

	// Undeploy olders versions of the CRD
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
				Log:     e.log.Debug,
			})
			if err != nil {
				return err
			}
			e.log.Debug("Undeployed old version of CRD", "gvr", oldGVR.String())
		}
	}

	if oldGVK.Version != gvk.Version && cr.Status.Kind == gvk.Kind && oldGVK.Group == gvk.Group {
		err = updateCompositionsVersion(ctx, e.dynamic, e.log.Debug, oldGVR, gvk.Version)
		if err != nil {
			return fmt.Errorf("error updating compositions version: %w", err)
		}
		e.log.Debug("Updated compositions version", "gvr", oldGVR.String())
	}

	// Sets the new version as served in the CRD
	crdtools.SetServedStorage(crd, gvk.Version, true, false)

	err = kube.Apply(ctx, e.kube, crd, kube.ApplyOptions{})
	if err != nil {
		return err
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

	if !meta.IsActionAllowed(cr, meta.ActionDelete) {
		e.log.Info("External resource should not be deleted by provider, skip deleting.")
		return nil
	}

	e.log.Debug("Deleting CompositionDefinition", "name", cr.Name)

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
	crdOk := true
	pluralInfo, err := plurals.Get(gvk, plurals.GetOptions{})
	if apierrors.IsNotFound(err) {
		crdOk = false
		e.log.Debug("Plural not found, CRD not found, skipping deletion", "gvk", gvk.String())
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, gvk.String())
	}
	if crdOk {
		gvr = schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: pluralInfo.Plural,
		}

		lst, err := getCompositionDefinitionsWithVersion(ctx, e.kube, schema.GroupKind{
			Group: gvk.Group,
			Kind:  gvk.Kind,
		}, cr.Spec.Chart.Version)
		if err != nil {
			e.log.Debug("Error getting CompositionDefinitions", "error", err)
			return fmt.Errorf("error getting CompositionDefinitions: %w", err)
		}
		if len(lst) == 1 {
			e.log.Debug("Deleting Compositions of this version", "gvk", gvk.String())

			// Delete compositions of this version manually
			ul, err := getCompositions(ctx, e.dynamic, e.log.Debug, gvr)
			if err != nil {
				e.log.Info("Error getting compositions", "gvr", gvr, "error", err)
				return fmt.Errorf("error getting compositions: %w", err)
			}

			for i := range ul.Items {
				e.log.Debug("Deleting composition", "name", ul.Items[i].GetName(), "namespace", ul.Items[i].GetNamespace())
				err := kube.Uninstall(ctx, e.kube, &ul.Items[i], kube.UninstallOptions{})
				if err != nil {
					return err
				}
			}

			ul, err = getCompositions(ctx, e.dynamic, e.log.Debug, gvr)
			if err != nil {
				e.log.Info("Error getting compositions", "gvr", gvr, "error", err)
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
			e.log.Debug("Error getting CompositionDefinitions", "error", err)
			return fmt.Errorf("error getting CompositionDefinitions: %w", err)
		}
		if len(lst) > 1 {
			skipCRD = true
			e.log.Info("Skipping CRD deletion, other CompositionDefinitions exist", "gvk", gvk.String())
		} else {
			skipCRD = false
			e.log.Info("Deleting CRD", "gvk", gvk.String())
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
			Log:                    e.log.Debug,
		}

		err = deploy.Undeploy(ctx, e.kube, opts)
		if err != nil {
			if errors.Is(err, deploy.ErrCompositionStillExist) {
				return fmt.Errorf("error undeploying CompositionDefinition: waiting for composition deletion")
			}
			return fmt.Errorf("error undeploying CompositionDefinition: %w", err)

		}
	} else {
		e.log.Info("CRD not found, deletion has already been completed", "gvk", gvk.String())
	}

	meta.RemoveFinalizer(cr, compositionStillExistFinalizer)
	return e.kube.Update(ctx, cr)
}
