package schemadefinitions

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	schemadefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/schemadefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/crdutil"
	"github.com/krateoplatformops/core-provider/internal/ptr"
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
	errNotCR = "managed resource is not a FormDefinition custom resource"

	reconcileGracePeriod = 1 * time.Minute
	reconcileTimeout     = 4 * time.Minute

	defaultGroup   = "apps.krateo.io"
	defaultVersion = "v1alpha1"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	_ = apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)

	name := reconciler.ControllerName(schemadefinitionsv1alpha1.SchemaDefinitionGroupKind)

	log := o.Logger.WithValues("controller", name)

	recorder := mgr.GetEventRecorderFor(name)

	r := reconciler.NewReconciler(mgr,
		resource.ManagedKind(schemadefinitionsv1alpha1.SchemaDefinitionGroupVersionKind),
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
		For(&schemadefinitionsv1alpha1.SchemaDefinition{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube     client.Client
	log      logging.Logger
	recorder record.EventRecorder
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (reconciler.ExternalClient, error) {
	_, ok := mg.(*schemadefinitionsv1alpha1.SchemaDefinition)
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
	cr, ok := mg.(*schemadefinitionsv1alpha1.SchemaDefinition)
	if !ok {
		return reconciler.ExternalObservation{}, errors.New(errNotCR)
	}

	verbose := meta.IsVerbose(cr)

	gvk := toGVK(cr)
	gr := crdutil.InferGroupResource(gvk.GroupKind())

	ok, err := crdutil.Lookup(ctx, e.kube, gr.WithVersion(gvk.Version))
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	if !ok {
		if verbose {
			e.log.Debug("CRD does not exists", "name", cr.Name, "groupResource", gr.String())
		}

		return reconciler.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}

	cr.Status.APIVersion, cr.Status.Kind = gvk.ToAPIVersionAndKind()
	cr.SetConditions(rtv1.Available())

	want, err := e.computeDigest(cr)
	if err != nil {
		return reconciler.ExternalObservation{}, err
	}

	got := ptr.Deref(cr.Status.Digest, "")

	if verbose {
		e.log.Debug("CRD already exists", "name", cr.Name, "groupResource", gr.String())
		e.log.Debug("CRD digest", "remote", want, "local", got)
	}

	return reconciler.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: (want == got),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*schemadefinitionsv1alpha1.SchemaDefinition)
	if !ok {
		return errors.New(errNotCR)
	}

	if !meta.IsActionAllowed(cr, meta.ActionCreate) {
		e.log.Debug("External resource should not be updated by provider, skip creating.")
		return nil
	}

	cr.SetConditions(rtv1.Creating())

	res := e.generateCRD(ctx, cr)
	if res.Err != nil {
		return res.Err
	}

	obj, err := crdutil.Unmarshal(res.Manifest)
	if err != nil {
		return err
	}

	err = crdutil.Install(ctx, e.kube, obj)
	if err != nil {
		return err
	}

	cr.Status.APIVersion, cr.Status.Kind = toGVK(cr).ToAPIVersionAndKind()
	cr.Status.Digest = ptr.To(res.Digest)

	return e.kube.Status().Update(ctx, cr)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*schemadefinitionsv1alpha1.SchemaDefinition)
	if !ok {
		return errors.New(errNotCR)
	}

	if !meta.IsActionAllowed(cr, meta.ActionUpdate) {
		e.log.Debug("External resource should not be updated by provider, skip updating.")
		return nil
	}

	verbose := meta.IsVerbose(cr)

	cr = cr.DeepCopy()

	res := e.generateCRD(ctx, cr)
	if res.Err != nil {
		return res.Err
	}

	if verbose {
		e.log.Debug("CRD Generated.", "digest", res.Digest)
	}

	newObj, err := crdutil.Unmarshal(res.Manifest)
	if err != nil {
		return err
	}

	gvk := toGVK(cr)
	gr := crdutil.InferGroupResource(gvk.GroupKind())
	err = crdutil.Update(ctx, e.kube, gr, newObj)
	if err != nil {
		return err
	}

	if verbose {
		e.log.Debug("CRD Updated.", "digest", res.Digest)
	}

	cr.Status.APIVersion, cr.Status.Kind = gvk.ToAPIVersionAndKind()
	cr.Status.Digest = ptr.To(res.Digest)

	return e.kube.Status().Update(ctx, cr)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*schemadefinitionsv1alpha1.SchemaDefinition)
	if !ok {
		return errors.New(errNotCR)
	}

	if !meta.IsActionAllowed(cr, meta.ActionDelete) {
		e.log.Debug("External resource should not be deleted by provider, skip deleting.")
		return nil
	}

	cr.SetConditions(rtv1.Deleting())

	gvk := toGVK(cr)
	gr := crdutil.InferGroupResource(gvk.GroupKind())

	return crdutil.Uninstall(ctx, e.kube, gr)
}

func (e *external) generateCRD(ctx context.Context, cr *schemadefinitionsv1alpha1.SchemaDefinition) crdgen.Result {
	return crdgen.Generate(ctx, crdgen.Options{
		WorkDir:    cr.GetName(),
		Categories: []string{"krateo", "forms"},
		GVK: schema.GroupVersionKind{
			Group:   defaultGroup,
			Version: ptr.Deref(cr.Spec.Schema.Version, defaultVersion),
			Kind:    cr.Spec.Schema.Kind,
		},
		SpecJsonSchemaGetter: UrlJsonSchemaGetter(cr.Spec.Schema.Url),
	})
}

func (e *external) computeDigest(cr *schemadefinitionsv1alpha1.SchemaDefinition) (string, error) {
	dat, err := UrlJsonSchemaGetter(cr.Spec.Schema.Url).Get()
	if err != nil {
		return "", err
	}

	h := sha256.New()
	if _, err = h.Write(dat); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
