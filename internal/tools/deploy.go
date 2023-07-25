package tools

import (
	"context"
	"fmt"

	"github.com/krateoplatformops/core-provider/internal/controllers/compositions/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeployOptions struct {
	Group     string
	Version   string
	Resource  string
	Namespace string
	Tag       string
}

func Deploy(ctx context.Context, kube client.Client, opts DeployOptions) error {
	values := templates.Values(templates.Renderoptions{
		Group:     opts.Group,
		Version:   opts.Version,
		Resource:  opts.Resource,
		Namespace: opts.Namespace,
		Tag:       opts.Tag,
	})

	if err := deployObject(ctx, kube, templates.ServiceAccount, values); err != nil {
		return err
	}

	if err := deployObject(ctx, kube, templates.Role, values); err != nil {
		return err
	}

	if err := deployObject(ctx, kube, templates.RoleBinding, values); err != nil {
		return err
	}

	if err := deployObject(ctx, kube, templates.Deployment, values); err != nil {
		return err
	}

	return nil
}

func deployObject(ctx context.Context, kube client.Client, tt templates.TemplateType, values map[string]string) error {
	dat, err := templates.Render(tt, values)
	if err != nil {
		return err
	}

	switch tt {
	case templates.ServiceAccount:
		obj, err := UnmarshalServiceAccount(dat)
		if err != nil {
			return err
		}
		return InstallServiceAccount(ctx, kube, obj)
	case templates.Role:
		obj, err := UnmarshalRole(dat)
		if err != nil {
			return err
		}
		return InstallRole(ctx, kube, obj)
	case templates.RoleBinding:
		obj, err := UnmarshalRoleBinding(dat)
		if err != nil {
			return err
		}
		return InstallRoleBinding(ctx, kube, obj)
	case templates.Deployment:
		obj, err := UnmarshalDeployment(dat)
		if err != nil {
			return err
		}
		return InstallDeployment(ctx, kube, obj)
	default:
		return fmt.Errorf("unknow type: %s", string(tt))
	}
}
