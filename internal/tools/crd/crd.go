package crd

import (
	"context"
	"fmt"
	"strings"
	"time"

	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/avast/retry-go"
	contexttools "github.com/krateoplatformops/core-provider/internal/tools/context"
	"github.com/krateoplatformops/core-provider/internal/tools/crd/generation"
	"github.com/krateoplatformops/core-provider/internal/tools/kube"
	"github.com/krateoplatformops/core-provider/internal/tools/kube/watcher"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InferGroupResource(gk schema.GroupKind) schema.GroupResource {
	kind := types.Type{Name: types.Name{Name: gk.Kind}}
	namer := namer.NewPrivatePluralNamer(nil)
	return schema.GroupResource{
		Group:    gk.Group,
		Resource: strings.ToLower(namer.Name(&kind)),
	}

}

func Uninstall(ctx context.Context, kube client.Client, gr schema.GroupResource) error {
	if err := registerEventually(); err != nil {
		return err
	}

	return retry.Do(
		func() error {
			obj := apiextensionsv1.CustomResourceDefinition{}
			err := kube.Get(ctx, client.ObjectKey{Name: gr.String()}, &obj, &client.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}

				return err
			}

			err = kube.Delete(ctx, &obj, &client.DeleteOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}

				return err
			}

			return nil
		},
	)
}

func Get(ctx context.Context, kube client.Client, gr schema.GroupResource) (*apiextensionsv1.CustomResourceDefinition, error) {
	if err := registerEventually(); err != nil {
		return nil, err
	}

	res := apiextensionsv1.CustomResourceDefinition{}
	err := kube.Get(ctx, client.ObjectKey{Name: gr.String()}, &res, &client.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &res, nil
}

func Lookup(ctx context.Context, kube client.Client, gvr schema.GroupVersionResource) (bool, error) {
	if err := registerEventually(); err != nil {
		return false, err
	}

	res := apiextensionsv1.CustomResourceDefinition{}
	err := kube.Get(ctx, client.ObjectKey{Name: gvr.GroupResource().String()}, &res, &client.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	for _, el := range res.Spec.Versions {
		if el.Name == gvr.Version {
			return true, nil
		}
	}

	return false, nil
}

type ApplyOpts struct {
	CABundle                []byte
	WebhookServiceNamespace string
	WebhookServiceName      string
}

// UpdateVersion updates the given CRD to set the given version spec. If the version
// does not exist, the CRD is created
// If the version exists, its specs are updated
// Before returning, the CRD is applied to the cluster and waited to be established with a watcher
// Returns an error if any occurs
func ApplyOrUpdateCRD(ctx context.Context,
	cli client.Client,
	dyn dynamic.Interface,
	newcrd *apiextensionsv1.CustomResourceDefinition,
	opts ApplyOpts,
) (schema.GroupVersionResource, error) {

	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())

	// Getting GVR from CRD
	gvr := schema.GroupVersionResource{
		Group:    newcrd.Spec.Group,
		Version:  newcrd.Spec.Versions[0].Name,
		Resource: newcrd.Spec.Names.Plural,
	}

	crd, err := Get(ctx, cli, gvr.GroupResource())
	if err != nil {
		return gvr, fmt.Errorf("error getting CRD: %w", err)
	}

	if crd == nil {
		log.Debug("Creating CRD", "gvr", gvr.String())
		err = kube.Apply(ctx, cli, newcrd, kube.ApplyOptions{})
		if err != nil {
			return gvr, fmt.Errorf("error applying CRD: %w", err)
		}
		err = watcher.NewWatcher(
			dyn,
			apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"),
			1*time.Minute,
			IsReady).WatchResource(ctx, "", newcrd.Name)
		if err != nil {
			return gvr, fmt.Errorf("error waiting for CRD to be established: %w", err)
		}

		return gvr, nil
	}
	log.Debug("Updating CRD", "gvr", gvr.String())

	statusEqual, err := generation.StatusEqual(crd, newcrd)
	if err != nil {
		return gvr, fmt.Errorf("error comparing CRD status: %w", err)
	}
	if !statusEqual {
		log.Debug("CRD status differs, updating status only", "crd", crd.Name, "version", gvr.Version)
		// To avoid issues with changed in the dynamically generated part of the CRD (the spec), we only update the static part (the status) to any version that already exists
		err = generation.UpdateStatus(crd, newcrd.Spec.Versions[0])
		if err != nil {
			return gvr, fmt.Errorf("error updating CRD version: %w", err)
		}
		err = kube.Apply(ctx, cli, crd, kube.ApplyOptions{})
		if err != nil {
			return gvr, fmt.Errorf("error applying CRD status update: %w", err)
		}

		err = watcher.NewWatcher(
			dyn,
			apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"),
			1*time.Minute,
			IsReady).WatchResource(ctx, "", crd.Name)
		if err != nil {
			return gvr, fmt.Errorf("error waiting for CRD to be established: %w", err)
		}
		return gvr, nil
	}
	if generation.GVKExists(crd, schema.GroupVersionKind{
		Group:   newcrd.Spec.Group,
		Kind:    newcrd.Spec.Names.Kind,
		Version: gvr.Version,
	}) {
		log.Debug("CRD version exists and is equal, skipping update", "crd", crd.Name, "version", gvr.Version)
		return gvr, nil
	}

	crd, err = generation.AppendVersion(*crd, *newcrd)
	if err != nil {
		return gvr, fmt.Errorf("error appending version to CRD: %w", err)
	}

	if opts.CABundle == nil {
		return gvr, fmt.Errorf("CA bundle is nil")
	}
	if opts.WebhookServiceName == "" {
		return gvr, fmt.Errorf("webhook service name is empty")
	}
	if opts.WebhookServiceNamespace == "" {
		return gvr, fmt.Errorf("webhook service namespace is empty")
	}
	injectConversionConfToCRD(crd, opts)

	generation.SetServedStorage(crd, gvr.Version, true, false)
	err = kube.Apply(ctx, cli, crd, kube.ApplyOptions{})
	if err != nil {
		return gvr, fmt.Errorf("error setting properties on CRD: %w", err)
	}

	err = watcher.NewWatcher(
		dyn,
		apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"),
		1*time.Minute,
		IsReady).WatchResource(ctx, "", crd.Name)
	if err != nil {
		return gvr, fmt.Errorf("error waiting for CRD to be established: %w", err)
	}

	return gvr, nil
}

func injectConversionConfToCRD(crd *apiextensionsv1.CustomResourceDefinition, opts ApplyOpts) {
	whport := int32(9443)
	whpath := "/convert"
	conf := &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ConversionReviewVersions: []string{"v1", "v1alpha1", "v1alpha2"},
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Namespace: opts.WebhookServiceNamespace,
					Name:      opts.WebhookServiceName,
					Port:      &whport,
					Path:      &whpath,
				},
				CABundle: opts.CABundle,
			},
		},
	}
	crd.Spec.Conversion = conf
}

func IsReady(crd *apiextensionsv1.CustomResourceDefinition) bool {
	if crd != nil {
		for _, cond := range crd.Status.Conditions {
			if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

func registerEventually() error {
	if clientsetscheme.Scheme.IsGroupRegistered(apiextensionsv1.SchemeGroupVersion.Group) {
		return nil
	}

	return apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)
}
