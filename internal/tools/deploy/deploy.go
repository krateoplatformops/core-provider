package deploy

import (
	"context"
	"errors"
	"fmt"
	"path"

	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	tools "github.com/krateoplatformops/core-provider/internal/tools"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"github.com/krateoplatformops/core-provider/internal/tools/configmap"
	crd "github.com/krateoplatformops/core-provider/internal/tools/crd"
	deployment "github.com/krateoplatformops/core-provider/internal/tools/deployment"
	"github.com/krateoplatformops/core-provider/internal/tools/rbactools"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CompositionVersionLabel  = "krateo.io/composition-version"
	CompositionStillExistErr = "compositions still exist"
)

var (
	ErrCompositionStillExist = errors.New(CompositionStillExistErr)
)

type UndeployOptions struct {
	DiscoveryClient discovery.CachedDiscoveryInterface
	KubeClient      client.Client
	DynamicClient   dynamic.Interface
	NamespacedName  types.NamespacedName
	GVR             schema.GroupVersionResource
	Spec            *definitionsv1alpha1.ChartInfo
	RBACFolderPath  string
	Log             func(msg string, keysAndValues ...any)
	SkipCRD         bool
}

const (
	controllerResourceSuffix = "-controller"
	configmapResourceSuffix  = "-configmap"
)

func Undeploy(ctx context.Context, kube client.Client, opts UndeployOptions) error {
	if !opts.SkipCRD {
		err := crd.Uninstall(ctx, opts.KubeClient, opts.GVR.GroupResource())
		if err == nil {
			if opts.Log != nil {
				opts.Log("CRD successfully uninstalled", "name", opts.GVR.GroupResource().String())
			}
		}
		// Create a label requirement for the composition version
		labelreq, err := labels.NewRequirement(CompositionVersionLabel, selection.Equals, []string{opts.GVR.Version})
		if err != nil {
			return err
		}
		selector := labels.NewSelector()
		selector = selector.Add(*labelreq)

		li, err := opts.DynamicClient.Resource(opts.GVR).List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return err
		}

		if len(li.Items) > 0 {
			return fmt.Errorf("%v for %s", ErrCompositionStillExist, opts.GVR.String())
		}
	}

	err := deployment.UninstallDeployment(ctx, deployment.UninstallOptions{
		KubeClient: opts.KubeClient,
		NamespacedName: types.NamespacedName{
			Namespace: opts.NamespacedName.Namespace,
			Name:      opts.NamespacedName.Name + controllerResourceSuffix,
		},
		Log: opts.Log,
	})
	if err != nil {
		return err
	}

	err = configmap.UninstallConfigmap(ctx, configmap.UninstallOptions{
		KubeClient: opts.KubeClient,
		NamespacedName: types.NamespacedName{
			Namespace: opts.NamespacedName.Namespace,
			Name:      opts.NamespacedName.Name + configmapResourceSuffix,
		},
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

	rbacNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + controllerResourceSuffix,
	}

	clusterrole, err := rbactools.CreateClusterRole(gvr, rbacNSName, path.Join(opts.RBACFolderPath, "clusterrole.yaml"))
	if err != nil {
		if opts.Log != nil {
			opts.Log("Error creating clusterrole", "error", err)
		}
	}

	err = rbactools.UninstallClusterRole(ctx, rbactools.UninstallOptions{
		KubeClient: opts.KubeClient,
		NamespacedName: types.NamespacedName{
			Namespace: clusterrole.Namespace,
			Name:      clusterrole.Name,
		},
		Log: opts.Log,
	})
	if err == nil {
		if opts.Log != nil {
			opts.Log("ClusterRole successfully uninstalled",
				"gvr", gvr.String(), "name", clusterrole.Name, "namespace", clusterrole.Namespace)
		}
	}

	clusterrolebinding, err := rbactools.CreateClusterRoleBinding(gvr, rbacNSName, path.Join(opts.RBACFolderPath, "clusterrolebinding.yaml"))
	if err != nil {
		if opts.Log != nil {
			opts.Log("Error creating clusterrolebinding", "error", err)
		}
	}

	err = rbactools.UninstallClusterRoleBinding(ctx, rbactools.UninstallOptions{
		KubeClient: opts.KubeClient,
		NamespacedName: types.NamespacedName{
			Namespace: clusterrolebinding.Namespace,
			Name:      clusterrolebinding.Name,
		},
		Log: opts.Log,
	})
	if err == nil {
		if opts.Log != nil {
			opts.Log("ClusterRoleBinding successfully uninstalled",
				"gvr", gvr.String(), "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace)
		}
	}

	role, err := rbactools.CreateRole(gvr, rbacNSName, path.Join(opts.RBACFolderPath, "compositiondefinition-role.yaml"))
	if err != nil {
		if opts.Log != nil {
			opts.Log("Error creating role", "error", err)
		}
	}

	err = rbactools.UninstallRole(ctx, rbactools.UninstallOptions{
		KubeClient: opts.KubeClient,
		NamespacedName: types.NamespacedName{
			Namespace: role.Namespace,
			Name:      role.Name,
		},
		Log: opts.Log,
	})
	if err == nil {
		if opts.Log != nil {
			opts.Log("Role successfully uninstalled",
				"gvr", gvr.String(), "name", role.Name, "namespace", role.Namespace)
		}
	}

	rolebinding, err := rbactools.CreateRoleBinding(gvr, rbacNSName, path.Join(opts.RBACFolderPath, "compositiondefinition-rolebinding.yaml"))
	if err != nil {
		if opts.Log != nil {
			opts.Log("Error creating rolebinding", "error", err)
		}
	}

	err = rbactools.UninstallRoleBinding(ctx, rbactools.UninstallOptions{
		KubeClient: opts.KubeClient,
		NamespacedName: types.NamespacedName{
			Namespace: rolebinding.Namespace,
			Name:      rolebinding.Name,
		},
		Log: opts.Log,
	})
	if err == nil {
		if opts.Log != nil {
			opts.Log("RoleBinding successfully uninstalled",
				"gvr", gvr.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace)
		}
	}

	if opts.Spec.Credentials != nil {
		secretNSName := types.NamespacedName{
			Namespace: opts.Spec.Credentials.PasswordRef.Namespace,
			Name:      opts.NamespacedName.Name + controllerResourceSuffix,
		}

		role, err := rbactools.CreateRole(gvr, secretNSName, path.Join(opts.RBACFolderPath, "secret-role.yaml", "secretName", opts.Spec.Credentials.PasswordRef.Name))
		if err != nil {
			if opts.Log != nil {
				opts.Log("Error creating role", "error", err)
			}
		}

		err = rbactools.UninstallRole(ctx, rbactools.UninstallOptions{
			KubeClient: opts.KubeClient,
			NamespacedName: types.NamespacedName{
				Namespace: role.Namespace,
				Name:      role.Name,
			},
			Log: opts.Log,
		})
		if err == nil {
			if opts.Log != nil {
				opts.Log("Role successfully uninstalled",
					"gvr", gvr.String(), "name", role.Name, "namespace", role.Namespace)
			}
		}

		rolebinding, err := rbactools.CreateRoleBinding(gvr, secretNSName, path.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"))
		if err != nil {
			if opts.Log != nil {
				opts.Log("Error creating rolebinding", "error", err)
			}
		}

		err = rbactools.UninstallRoleBinding(ctx, rbactools.UninstallOptions{
			KubeClient: opts.KubeClient,
			NamespacedName: types.NamespacedName{
				Namespace: rolebinding.Namespace,
				Name:      rolebinding.Name,
			},
			Log: opts.Log,
		})
		if err == nil {
			if opts.Log != nil {
				opts.Log("RoleBinding successfully uninstalled",
					"gvr", gvr.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace)
			}
		}
	}
	return err
}

type DeployOptions struct {
	DiscoveryClient        discovery.CachedDiscoveryInterface
	KubeClient             client.Client
	NamespacedName         types.NamespacedName
	Spec                   *definitionsv1alpha1.ChartInfo
	RBACFolderPath         string
	DeploymentTemplatePath string
	ConfigmapTemplatePath  string
	Log                    func(msg string, keysAndValues ...any)
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

	if opts.Spec.Credentials != nil {
		secretNSName := types.NamespacedName{
			Namespace: opts.Spec.Credentials.PasswordRef.Namespace,
			Name:      opts.NamespacedName.Name + controllerResourceSuffix,
		}

		role, err := rbactools.CreateRole(gvr, secretNSName, path.Join(opts.RBACFolderPath, "secret-role.yaml", "secretName", opts.Spec.Credentials.PasswordRef.Name))
		if err != nil {
			if opts.Log != nil {
				opts.Log("Error creating role", "error", err)
			}
		}

		err = rbactools.InstallRole(ctx, opts.KubeClient, &role)
		if err == nil {
			if opts.Log != nil {
				opts.Log("Role successfully installed",
					"gvr", gvr.String(), "name", role.Name, "namespace", role.Namespace)
			}
		}

		rolebinding, err := rbactools.CreateRoleBinding(gvr, secretNSName, path.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"))
		if err != nil {
			if opts.Log != nil {
				opts.Log("Error creating rolebinding", "error", err)
			}
		}

		err = rbactools.InstallRoleBinding(ctx, opts.KubeClient, &rolebinding)
		if err == nil {
			if opts.Log != nil {
				opts.Log("RoleBinding successfully installed",
					"gvr", gvr.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace)
			}
		}
	}

	rbacNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + controllerResourceSuffix,
	}

	clusterrole, err := rbactools.CreateClusterRole(gvr, rbacNSName, path.Join(opts.RBACFolderPath, "clusterrole.yaml"))
	if err != nil {
		if opts.Log != nil {
			opts.Log("Error creating clusterrole", "error", err)
		}
	}
	err = rbactools.InstallClusterRole(ctx, opts.KubeClient, &clusterrole)
	if err == nil {
		if opts.Log != nil {
			opts.Log("ClusterRole successfully installed",
				"gvr", gvr.String(), "name", clusterrole.Name, "namespace", clusterrole.Namespace)
		}
	}

	clusterrolebinding, err := rbactools.CreateClusterRoleBinding(gvr, rbacNSName, path.Join(opts.RBACFolderPath, "clusterrolebinding.yaml"))
	if err != nil {
		if opts.Log != nil {
			opts.Log("Error creating clusterrolebinding", "error", err)
		}
	}
	err = rbactools.InstallClusterRoleBinding(ctx, opts.KubeClient, &clusterrolebinding)
	if err == nil {
		if opts.Log != nil {
			opts.Log("ClusterRoleBinding successfully installed",
				"gvr", gvr.String(), "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace)
		}
	}

	role, err := rbactools.CreateRole(gvr, rbacNSName, path.Join(opts.RBACFolderPath, "compositiondefinition-role.yaml"))
	if err != nil {
		if opts.Log != nil {
			opts.Log("Error creating role", "error", err)
		}
	}

	err = rbactools.InstallRole(ctx, opts.KubeClient, &role)
	if err == nil {
		if opts.Log != nil {
			opts.Log("Role successfully installed",
				"gvr", gvr.String(), "name", role.Name, "namespace", role.Namespace)
		}
	}

	rolebinding, err := rbactools.CreateRoleBinding(gvr, rbacNSName, path.Join(opts.RBACFolderPath, "compositiondefinition-rolebinding.yaml"))
	if err != nil {
		if opts.Log != nil {
			opts.Log("Error creating rolebinding", "error", err)
		}
	}

	err = rbactools.InstallRoleBinding(ctx, opts.KubeClient, &rolebinding)
	if err == nil {
		if opts.Log != nil {
			opts.Log("RoleBinding successfully installed",
				"gvr", gvr.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace)
		}
	}

	cmNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + configmapResourceSuffix,
	}
	cm, err := configmap.CreateConfigmap(gvr, cmNSName, opts.ConfigmapTemplatePath)
	if err != nil {
		return err, rbacErr
	}

	err = configmap.InstallConfigmap(ctx, opts.KubeClient, &cm)
	if err == nil {
		if opts.Log != nil {
			opts.Log("Configmap successfully installed",
				"gvr", gvr.String(), "name", cm.Name, "namespace", cm.Namespace)
		}
	} else {
		return fmt.Errorf("Error installing configmap: %v", err), rbacErr
	}

	deploymentNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + controllerResourceSuffix,
	}
	dep, err := deployment.CreateDeployment(gvr, deploymentNSName, opts.DeploymentTemplatePath)
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
