package tools

import (
	"context"
	"fmt"

	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeployOptions struct {
	Namespace string
	ChartFS   *chartfs.ChartFS
}

func Deploy(ctx context.Context, kube client.Client, opts DeployOptions) error {
	gvr, err := GroupVersionResource(opts.ChartFS)
	if err != nil {
		return err
	}

	nn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-controller", gvr.Resource),
		Namespace: opts.Namespace,
	}

	sa := CreateServiceAccount(nn)
	if err := InstallServiceAccount(ctx, kube, &sa); err != nil {
		return err
	}

	role, err := CreateRole(opts.ChartFS, gvr.Resource, nn)
	if err != nil {
		return err
	}
	if err := InstallRole(ctx, kube, &role); err != nil {
		return err
	}

	rb := CreateRoleBinding(nn)
	if err := InstallRoleBinding(ctx, kube, &rb); err != nil {
		return err
	}

	crb := CreateClusterRoleBinding(nn)
	if err := InstallClusterRoleBinding(ctx, kube, &crb); err != nil {
		return err
	}

	dep, err := CreateDeployment(gvr, nn.Namespace)
	if err != nil {
		return err
	}
	return InstallDeployment(ctx, kube, &dep)
}
