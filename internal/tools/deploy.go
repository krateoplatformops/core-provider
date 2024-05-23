package tools

import (
	"context"
	"errors"
	"fmt"

	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UndeployOptions struct {
	KubeClient     client.Client
	NamespacedName types.NamespacedName
	GVR            schema.GroupVersionResource
	Log            func(msg string, keysAndValues ...any)
}

func Undeploy(ctx context.Context, opts UndeployOptions) error {
	err := UninstallDeployment(ctx, UninstallOptions{
		KubeClient: opts.KubeClient,
		NamespacedName: types.NamespacedName{
			Namespace: opts.NamespacedName.Namespace,
			Name:      fmt.Sprintf("%s-%s-controller", opts.GVR.Resource, opts.GVR.Version),
		},
		Log: opts.Log,
	})
	if err != nil {
		return err
	}

	err = UninstallClusterRoleBinding(ctx, UninstallOptions{
		KubeClient:     opts.KubeClient,
		NamespacedName: opts.NamespacedName,
		Log:            opts.Log,
	})
	if err != nil {
		return err
	}

	err = UninstallClusterRole(ctx, UninstallOptions{
		KubeClient:     opts.KubeClient,
		NamespacedName: opts.NamespacedName,
		Log:            opts.Log,
	})
	if err != nil {
		return err
	}

	err = UninstallRoleBinding(ctx, UninstallOptions{
		KubeClient:     opts.KubeClient,
		NamespacedName: opts.NamespacedName,
		Log:            opts.Log,
	})
	if err != nil {
		return err
	}

	err = UninstallRole(ctx, UninstallOptions{
		KubeClient:     opts.KubeClient,
		NamespacedName: opts.NamespacedName,
		Log:            opts.Log,
	})
	if err != nil {
		return err
	}

	err = UninstallServiceAccount(ctx, UninstallOptions{
		KubeClient:     opts.KubeClient,
		NamespacedName: opts.NamespacedName,
		Log:            opts.Log,
	})
	if err != nil {
		return err
	}

	err = UninstallCRD(ctx, opts.KubeClient, opts.GVR.GroupResource())
	if err == nil {
		if opts.Log != nil {
			opts.Log("CRD successfully uninstalled", "name", opts.GVR.GroupResource().String())
		}
	}
	return err
}

type DeployOptions struct {
	DiscoveryClient discovery.DiscoveryInterface
	KubeClient      client.Client
	NamespacedName  types.NamespacedName
	Spec            *definitionsv1alpha1.ChartInfo
	CDCImageTag     string
	Log             func(msg string, keysAndValues ...any)
}

func Deploy(ctx context.Context, kube client.Client, opts DeployOptions) (err error, rbacErr error) {
	pkg, err := chartfs.ForSpec(ctx, kube, opts.Spec)
	if err != nil {
		return err, nil
	}

	gvk, err := GroupVersionKind(pkg)
	if err != nil {
		return err, nil
	}

	gvr := ToGroupVersionResource(gvk)

	sa := CreateServiceAccount(opts.NamespacedName)
	if err = InstallServiceAccount(ctx, opts.KubeClient, &sa); err != nil {
		return err, nil
	}
	if opts.Log != nil {
		opts.Log("ServiceAccount successfully installed",
			"gvr", gvr.String(), "name", sa.Name, "namespace", sa.Namespace)
	}

	role, err := InitRole(pkg, gvr.Resource, opts.NamespacedName)
	if err != nil {
		return err, nil
	}
	cr := InitClusterRole(opts.NamespacedName)

	err = PopulateRoleClusterRole(pkg, opts.DiscoveryClient, &role, &cr)
	if errors.Is(err, ErrKindApiVersion) {
		rbacErr = err
	} else if err != nil {
		return err, nil
	}

	if err := InstallRole(ctx, opts.KubeClient, &role); err != nil {
		return err, rbacErr
	}
	if opts.Log != nil {
		opts.Log("Role successfully installed",
			"gvr", gvr.String(), "name", role.Name, "namespace", role.Namespace)
	}

	rb := CreateRoleBinding(opts.NamespacedName)
	if err := InstallRoleBinding(ctx, opts.KubeClient, &rb); err != nil {
		return err, rbacErr
	}
	if opts.Log != nil {
		opts.Log("RoleBinding successfully installed",
			"gvr", gvr.String(), "name", rb.Name, "namespace", rb.Namespace)
	}

	if err := InstallClusterRole(ctx, opts.KubeClient, &cr); err != nil {
		return err, rbacErr
	}
	if opts.Log != nil {
		opts.Log("ClusterRole successfully installed",
			"gvr", gvr.String(), "name", cr.Name, "namespace", cr.Namespace)
	}

	crb := CreateClusterRoleBinding(opts.NamespacedName)
	if err := InstallClusterRoleBinding(ctx, opts.KubeClient, &crb); err != nil {
		return err, rbacErr
	}
	if opts.Log != nil {
		opts.Log("ClusterRoleBinding successfully installed",
			"gvr", gvr.String(), "name", crb.Name, "namespace", crb.Namespace)
	}

	dep, err := CreateDeployment(gvr, opts.NamespacedName, opts.CDCImageTag)
	if err != nil {
		return err, rbacErr
	}

	err = InstallDeployment(ctx, opts.KubeClient, &dep)
	if err == nil {
		if opts.Log != nil {
			opts.Log("Deployment successfully installed",
				"gvr", gvr.String(), "name", dep.Name, "namespace", dep.Namespace)
		}
	}
	return err, rbacErr
}
