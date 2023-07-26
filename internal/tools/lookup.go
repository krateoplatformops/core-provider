package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/krateoplatformops/core-provider/internal/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type LookupOptions struct {
	ObjectType templates.TemplateType
	Group      string
	Version    string
	Resource   string
	Namespace  string
}

func Lookup(ctx context.Context, kube client.Client, opts LookupOptions) (bool, error) {
	values := templates.Values(templates.Renderoptions{
		Group:     opts.Group,
		Version:   opts.Version,
		Resource:  opts.Resource,
		Namespace: opts.Namespace,
	})

	var obj client.Object
	var err error

	switch opts.ObjectType {
	case templates.ServiceAccount:
		dat, err := templates.Render(templates.ServiceAccount, values)
		if err != nil {
			return false, err
		}

		obj, err = UnmarshalServiceAccount(dat)
		if err != nil {
			return false, err
		}
	case templates.Role:
		dat, err := templates.Render(templates.Role, values)
		if err != nil {
			return false, err
		}

		obj, err = UnmarshalRole(dat)
		if err != nil {
			return false, err
		}
	case templates.RoleBinding:
		dat, err := templates.Render(templates.RoleBinding, values)
		if err != nil {
			return false, err
		}

		obj, err = UnmarshalRoleBinding(dat)
		if err != nil {
			return false, err
		}
	case templates.Deployment:
		dat, err := templates.Render(templates.Deployment, values)
		if err != nil {
			return false, err
		}

		obj, err = UnmarshalDeployment(dat)
		if err != nil {
			return false, err
		}
	default:
		return false, fmt.Errorf("unknow type: %s", string(opts.ObjectType))
	}

	err = kube.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func LookupCRD(ctx context.Context, kube client.Client, gvr schema.GroupVersionResource) (bool, error) {
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

func ToGroupVersionResource(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(flect.Pluralize(gvk.Kind)),
	}
}
