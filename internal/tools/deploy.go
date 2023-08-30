package tools

import (
	"context"

	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/definitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeployOptions struct {
	Namespace string
	Name      string
	Spec      *definitionsv1alpha1.ChartInfo
}

func Undeploy(ctx context.Context, kube client.Client, gvr schema.GroupVersionResource, nn types.NamespacedName) error {
	if err := UninstallDeployment(ctx, kube, nn); err != nil {
		return err
	}

	if err := UninstallClusterRoleBinding(ctx, kube, nn); err != nil {
		return err
	}

	if err := UninstallClusterRole(ctx, kube, nn); err != nil {
		return err
	}

	if err := UninstallRoleBinding(ctx, kube, nn); err != nil {
		return err
	}

	if err := UninstallRole(ctx, kube, nn); err != nil {
		return err
	}

	if err := UninstallServiceAccount(ctx, kube, nn); err != nil {
		return err
	}

	return UninstallCRD(ctx, kube, gvr.GroupResource())
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
		Name:      opts.Name,
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

	cr := CreateClusterRole(nn)
	if err := InstallClusterRole(ctx, kube, &cr); err != nil {
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
