package definitions

import (
	"context"
	"strings"

	"github.com/avast/retry-go"
	"github.com/gobuffalo/flect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/runtime/schema"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

func lookupCRD(ctx context.Context, kube client.Client, gvk schema.GroupVersionKind) (bool, error) {
	gvr := toGroupVersionResource(gvk)

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

func installCRD(ctx context.Context, kube client.Client, obj *apiextensionsv1.CustomResourceDefinition) error {
	return retry.Do(
		func() error {
			tmp := apiextensionsv1.CustomResourceDefinition{}
			err := kube.Get(ctx, client.ObjectKeyFromObject(obj), &tmp)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return kube.Create(ctx, obj)
				}

				return err
			}

			gracePeriod := int64(0)
			_ = kube.Delete(ctx, &tmp, &client.DeleteOptions{GracePeriodSeconds: &gracePeriod})

			return kube.Create(ctx, obj)
		},
	)
}

func unmarshalCRD(dat []byte) (*apiextensionsv1.CustomResourceDefinition, error) {
	if !clientsetscheme.Scheme.IsGroupRegistered("apiextensions.k8s.io") {
		_ = apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)
	}

	// Create a YAML serializer.
	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := &apiextensionsv1.CustomResourceDefinition{}
	_, _, err := s.Decode(dat, nil, res)
	return res, err
}

func toGroupVersionResource(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(flect.Pluralize(gvk.Kind)),
	}
}
