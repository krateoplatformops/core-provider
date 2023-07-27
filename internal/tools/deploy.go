package tools

import (
	"context"

	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeployOptions struct {
	GVR            schema.GroupVersionResource
	NamespacedName types.NamespacedName
	Tag            string
	ChartURL       string
}

func Deploy(ctx context.Context, kube client.Client, opts DeployOptions) error {
	sa := CreateServiceAccount(opts.NamespacedName)
	if err := InstallServiceAccount(ctx, kube, &sa); err != nil {
		return err
	}

	pkg, err := chartfs.FromURL(opts.ChartURL)
	if err != nil {
		return err
	}

	role, err := CreateRole(pkg, opts.GVR.Resource, opts.NamespacedName)
	if err != nil {
		return err
	}
	if err := InstallRole(ctx, kube, &role); err != nil {
		return err
	}

	rb := CreateRoleBinding(opts.NamespacedName)
	if err := InstallRoleBinding(ctx, kube, &rb); err != nil {
		return err
	}

	dep, err := CreateDeployment(opts.GVR, opts.NamespacedName)
	if err != nil {
		return err
	}
	return InstallDeployment(ctx, kube, &dep)
}
