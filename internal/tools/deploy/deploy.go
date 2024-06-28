package deploy

import (
	"context"
	"errors"
	"fmt"

	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	tools "github.com/krateoplatformops/core-provider/internal/tools"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	crd "github.com/krateoplatformops/core-provider/internal/tools/crd"
	deployment "github.com/krateoplatformops/core-provider/internal/tools/deployment"
	"github.com/krateoplatformops/core-provider/internal/tools/rbacgen"
	"github.com/krateoplatformops/core-provider/internal/tools/rbactools"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UndeployOptions struct {
	DiscoveryClient discovery.DiscoveryInterface
	KubeClient      client.Client
	NamespacedName  types.NamespacedName
	GVR             schema.GroupVersionResource
	Spec            *definitionsv1alpha1.ChartInfo
	Log             func(msg string, keysAndValues ...any)
}

func Undeploy(ctx context.Context, kube client.Client, opts UndeployOptions) error {
	err := deployment.UninstallDeployment(ctx, deployment.UninstallOptions{
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

	pkg, err := chartfs.ForSpec(ctx, kube, opts.Spec)
	if err != nil {
		return err
	}
	gvk, err := tools.GroupVersionKind(pkg)
	if err != nil {
		return err
	}
	gvr := tools.ToGroupVersionResource(gvk)
	rbgen := rbacgen.NewRbacGenerator(opts.DiscoveryClient, pkg, opts.NamespacedName.Name, opts.NamespacedName.Namespace)
	rbMap, err := rbgen.PopulateRBAC(gvr.Resource)
	if err != nil && !errors.Is(err, rbacgen.ErrKindApiVersion) {
		return err
	}

	for ns := range rbMap {
		if ns == "" {
			ns = "default"
		}
		nsName := types.NamespacedName{
			Namespace: ns,
			Name:      opts.NamespacedName.Name,
		}
		err = rbactools.UninstallClusterRoleBinding(ctx, rbactools.UninstallOptions{
			KubeClient:     opts.KubeClient,
			NamespacedName: nsName,
			Log:            opts.Log,
		})
		if err != nil {
			return err
		}

		err = rbactools.UninstallClusterRole(ctx, rbactools.UninstallOptions{
			KubeClient:     opts.KubeClient,
			NamespacedName: nsName,
			Log:            opts.Log,
		})
		if err != nil {
			return err
		}

		err = rbactools.UninstallRoleBinding(ctx, rbactools.UninstallOptions{
			KubeClient:     opts.KubeClient,
			NamespacedName: nsName,
			Log:            opts.Log,
		})
		if err != nil {
			return err
		}

		err = rbactools.UninstallRole(ctx, rbactools.UninstallOptions{
			KubeClient:     opts.KubeClient,
			NamespacedName: nsName,
			Log:            opts.Log,
		})
		if err != nil {
			return err
		}

		err = rbactools.UninstallServiceAccount(ctx, rbactools.UninstallOptions{
			KubeClient:     opts.KubeClient,
			NamespacedName: nsName,
			Log:            opts.Log,
		})
		if err != nil {
			return err
		}
	}
	err = crd.UninstallCRD(ctx, opts.KubeClient, opts.GVR.GroupResource())
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

	gvk, err := tools.GroupVersionKind(pkg)
	if err != nil {
		return err, nil
	}

	gvr := tools.ToGroupVersionResource(gvk)

	rbgen := rbacgen.NewRbacGenerator(opts.DiscoveryClient, pkg, opts.NamespacedName.Name, opts.NamespacedName.Namespace)

	rbMap, err := rbgen.PopulateRBAC(gvr.Resource)
	if errors.Is(err, rbacgen.ErrKindApiVersion) {
		rbacErr = err
		err = nil
	} else if err != nil {
		return err, nil
	}

	if opts.Log != nil {
		opts.Log("RBAC successfully populated", "gvr", gvr.String(), "rbMap", len(rbMap))
	}
	for _, rb := range rbMap {
		if rb.ServiceAccount != nil {
			if err = rbactools.InstallServiceAccount(ctx, opts.KubeClient, rb.ServiceAccount); err != nil {
				return err, rbacErr
			}
			if opts.Log != nil {
				opts.Log("ServiceAccount successfully installed",
					"gvr", gvr.String(), "name", rb.ServiceAccount.Name, "namespace", rb.ServiceAccount.Namespace)
			}
		}
		if rb.Role != nil {
			if err = rbactools.InstallRole(ctx, opts.KubeClient, rb.Role); err != nil {
				return err, rbacErr
			}
			if opts.Log != nil {
				opts.Log("Role successfully installed",
					"gvr", gvr.String(), "name", rb.Role.Name, "namespace", rb.Role.Namespace)
			}
		}
		if rb.RoleBinding != nil {
			if err = rbactools.InstallRoleBinding(ctx, opts.KubeClient, rb.RoleBinding); err != nil {
				return err, rbacErr
			}
			if opts.Log != nil {
				opts.Log("RoleBinding successfully installed",
					"gvr", gvr.String(), "name", rb.RoleBinding.Name, "namespace", rb.RoleBinding.Namespace)
			}
		}
		if rb.ClusterRole != nil {
			if err = rbactools.InstallClusterRole(ctx, opts.KubeClient, rb.ClusterRole); err != nil {
				return err, rbacErr
			}
			if opts.Log != nil {
				opts.Log("ClusterRole successfully installed",
					"gvr", gvr.String(), "name", rb.ClusterRole.Name, "namespace", rb.ClusterRole.Namespace)
			}
		}
		if rb.ClusterRoleBinding != nil {
			if err = rbactools.InstallClusterRoleBinding(ctx, opts.KubeClient, rb.ClusterRoleBinding); err != nil {
				return err, rbacErr
			}
			if opts.Log != nil {
				opts.Log("ClusterRoleBinding successfully installed",
					"gvr", gvr.String(), "name", rb.ClusterRoleBinding.Name, "namespace", rb.ClusterRoleBinding.Namespace)
			}
		}
	}

	dep, err := deployment.CreateDeployment(gvr, opts.NamespacedName, opts.CDCImageTag)
	if err != nil {
		return err, rbacErr
	}

	err = deployment.InstallDeployment(ctx, opts.KubeClient, &dep)
	if err == nil {
		if opts.Log != nil {
			opts.Log("Deployment successfully installed",
				"gvr", gvr.String(), "name", dep.Name, "namespace", dep.Namespace)
		}
	}
	return err, rbacErr
}
