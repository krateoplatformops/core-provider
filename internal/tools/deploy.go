package tools

import (
	"context"
	"fmt"

	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/definitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeployOptions struct {
	Namespace string
	Spec      *definitionsv1alpha1.ChartInfo
}

func Deploy(ctx context.Context, kube client.Client, opts DeployOptions) error {
	pkg, err := chartfs.ForSpec(opts.Spec)
	if err != nil {
		return err
	}

	gvr, err := GroupVersionResource(pkg)
	if err != nil {
		return err
	}

	nn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s-controller", gvr.Resource, gvr.Version),
		Namespace: opts.Namespace,
	}

	sa := CreateServiceAccount(nn)
	if err := InstallServiceAccount(ctx, kube, &sa); err != nil {
		return err
	}

	role, err := CreateRole(pkg, gvr.Resource, nn)
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

	dep, err := CreateDeployment(gvr, nn)
	if err != nil {
		return err
	}
	return InstallDeployment(ctx, kube, &dep)
}
