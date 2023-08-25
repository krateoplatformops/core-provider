package tools

import (
	"context"
	"os"

	"github.com/avast/retry-go"
	"github.com/krateoplatformops/core-provider/internal/templates"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
)

func InstallDeployment(ctx context.Context, kube client.Client, obj *appsv1.Deployment) error {
	return retry.Do(
		func() error {
			tmp := appsv1.Deployment{}
			err := kube.Get(ctx, client.ObjectKeyFromObject(obj), &tmp)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return kube.Create(ctx, obj)
				}

				return err
			}

			return nil
		},
	)
}

func CreateDeployment(gvr schema.GroupVersionResource, nn types.NamespacedName) (appsv1.Deployment, error) {
	values := templates.Values(templates.Renderoptions{
		Group:     gvr.Group,
		Version:   gvr.Version,
		Resource:  gvr.Resource,
		Namespace: nn.Namespace,
		Name:      nn.Name,
		Tag:       os.Getenv("CDC_IMAGE_TAG"),
	})

	dat, err := templates.RenderDeployment(values)
	if err != nil {
		return appsv1.Deployment{}, err
	}

	if !clientsetscheme.Scheme.IsGroupRegistered("apps") {
		_ = appsv1.AddToScheme(clientsetscheme.Scheme)
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := appsv1.Deployment{}
	_, _, err = s.Decode(dat, nil, &res)
	return res, err
}

func LookupDeployment(ctx context.Context, kube client.Client, obj *appsv1.Deployment) (bool, error) {
	err := kube.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
