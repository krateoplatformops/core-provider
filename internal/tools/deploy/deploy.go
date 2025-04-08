package deploy

import (
	"context"
	"errors"
	"fmt"
	"path"

	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/configmap"
	crd "github.com/krateoplatformops/core-provider/internal/tools/crd"
	deployment "github.com/krateoplatformops/core-provider/internal/tools/deployment"
	hasher "github.com/krateoplatformops/core-provider/internal/tools/hash"
	kubecli "github.com/krateoplatformops/core-provider/internal/tools/kube"
	"github.com/krateoplatformops/core-provider/internal/tools/rbactools"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

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
	controllerResourceSuffix = "-controller"
	configmapResourceSuffix  = "-configmap"
)

var (
	ErrCompositionStillExist = errors.New(CompositionStillExistErr)
)

type UndeployOptions struct {
	DiscoveryClient        discovery.CachedDiscoveryInterface
	KubeClient             client.Client
	DynamicClient          dynamic.Interface
	NamespacedName         types.NamespacedName
	GVR                    schema.GroupVersionResource
	Spec                   *definitionsv1alpha1.ChartInfo
	RBACFolderPath         string
	Log                    func(msg string, keysAndValues ...any)
	SkipCRD                bool
	DeploymentTemplatePath string
	ConfigmapTemplatePath  string
}

type DeployOptions struct {
	GVR                    schema.GroupVersionResource
	DiscoveryClient        discovery.CachedDiscoveryInterface
	KubeClient             client.Client
	NamespacedName         types.NamespacedName
	Spec                   *definitionsv1alpha1.ChartInfo
	RBACFolderPath         string
	DeploymentTemplatePath string
	ConfigmapTemplatePath  string
	Log                    func(msg string, keysAndValues ...any)
	// DryRunServer is used to determine if the deployment should be applied in dry-run mode. This is ignored in lookup mode
	DryRunServer bool
}

func logError(log func(msg string, keysAndValues ...any), msg string, err error) {
	if log != nil {
		log(msg, "error", err)
	}
}

func createRBACResources(gvr schema.GroupVersionResource, rbacNSName types.NamespacedName, rbacFolderPath string) (corev1.ServiceAccount, rbacv1.ClusterRole, rbacv1.ClusterRoleBinding, rbacv1.Role, rbacv1.RoleBinding, error) {
	sa, err := rbactools.CreateServiceAccount(gvr, rbacNSName, path.Join(rbacFolderPath, "serviceaccount.yaml"))
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	clusterrole, err := rbactools.CreateClusterRole(gvr, rbacNSName, path.Join(rbacFolderPath, "clusterrole.yaml"))
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	clusterrolebinding, err := rbactools.CreateClusterRoleBinding(gvr, rbacNSName, path.Join(rbacFolderPath, "clusterrolebinding.yaml"), "serviceAccount", sa.Name, "saNamespace", sa.Namespace)
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	role, err := rbactools.CreateRole(gvr, rbacNSName, path.Join(rbacFolderPath, "compositiondefinition-role.yaml"))
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	rolebinding, err := rbactools.CreateRoleBinding(gvr, rbacNSName, path.Join(rbacFolderPath, "compositiondefinition-rolebinding.yaml"), "serviceAccount", sa.Name, "saNamespace", sa.Namespace)
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	return sa, clusterrole, clusterrolebinding, role, rolebinding, nil
}

func installRBACResources(ctx context.Context, kubeClient client.Client, clusterrole rbacv1.ClusterRole, clusterrolebinding rbacv1.ClusterRoleBinding, role rbacv1.Role, rolebinding rbacv1.RoleBinding, sa corev1.ServiceAccount, log func(msg string, keysAndValues ...any), hsh *hasher.ObjectHash, applyOpts kubecli.ApplyOptions) error {
	err := kubecli.Apply(ctx, kubeClient, &clusterrole, applyOpts)
	if err != nil {
		logError(log, "Error installing clusterrole", err)
		return err
	}
	log("ClusterRole successfully installed", "name", clusterrole.Name, "namespace", clusterrole.Namespace)

	err = kubecli.Apply(ctx, kubeClient, &clusterrolebinding, applyOpts)
	if err != nil {
		logError(log, "Error installing clusterrolebinding", err)
		return err
	}
	log("ClusterRoleBinding successfully installed", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace)

	err = kubecli.Apply(ctx, kubeClient, &role, applyOpts)
	if err != nil {
		logError(log, "Error installing role", err)
		return err
	}
	log("Role successfully installed", "name", role.Name, "namespace", role.Namespace)

	err = kubecli.Apply(ctx, kubeClient, &rolebinding, applyOpts)
	if err != nil {
		logError(log, "Error installing rolebinding", err)
		return err
	}
	log("RoleBinding successfully installed", "name", rolebinding.Name, "namespace", rolebinding.Namespace)

	err = kubecli.Apply(ctx, kubeClient, &sa, applyOpts)
	if err != nil {
		logError(log, "Error installing serviceaccount", err)
		return err
	}
	log("ServiceAccount successfully installed", "name", sa.Name, "namespace", sa.Namespace)

	if hsh != nil {
		err = hsh.SumHash(
			clusterrole,
			clusterrolebinding,
			role,
			rolebinding,
			sa,
		)
		if err != nil {
			return fmt.Errorf("error hashing rbac resources: %v", err)
		}
	}

	return nil
}

func lookupRBACResources(ctx context.Context, kubeClient client.Client, clusterrole rbacv1.ClusterRole, clusterrolebinding rbacv1.ClusterRoleBinding, role rbacv1.Role, rolebinding rbacv1.RoleBinding, sa corev1.ServiceAccount, log func(msg string, keysAndValues ...any), hsh *hasher.ObjectHash) error {
	err := kubecli.Get(ctx, kubeClient, &clusterrole)
	if err != nil {
		logError(log, "Error getting clusterrole", err)
		return err
	}
	log("ClusterRole successfully fetched", "name", clusterrole.Name, "namespace", clusterrole.Namespace)

	err = kubecli.Get(ctx, kubeClient, &clusterrolebinding)
	if err != nil {
		logError(log, "Error getting clusterrolebinding", err)
		return err
	}
	log("ClusterRoleBinding successfully fetched", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace)

	err = kubecli.Get(ctx, kubeClient, &role)
	if err != nil {
		logError(log, "Error getting role", err)
		return err
	}
	log("Role successfully fetched", "name", role.Name, "namespace", role.Namespace)

	err = kubecli.Get(ctx, kubeClient, &rolebinding)
	if err != nil {
		logError(log, "Error getting rolebinding", err)
		return err
	}
	log("RoleBinding successfully fetched", "name", rolebinding.Name, "namespace", rolebinding.Namespace)

	err = kubecli.Get(ctx, kubeClient, &sa)
	if err != nil {
		logError(log, "Error getting serviceaccount", err)
		return err
	}
	log("ServiceAccount successfully fetched", "name", sa.Name, "namespace", sa.Namespace)

	if hsh != nil {
		err = hsh.SumHash(
			clusterrole,
			clusterrolebinding,
			role,
			rolebinding,
			sa,
		)
		if err != nil {
			return fmt.Errorf("error hashing rbac resources: %v", err)
		}
	}

	return nil
}

func uninstallRBACResources(ctx context.Context, kubeClient client.Client, clusterrole rbacv1.ClusterRole, clusterrolebinding rbacv1.ClusterRoleBinding, role rbacv1.Role, rolebinding rbacv1.RoleBinding, sa corev1.ServiceAccount, log func(msg string, keysAndValues ...any)) error {
	if log == nil {
		return fmt.Errorf("log function is required")
	}
	err := kubecli.Uninstall(ctx, kubeClient, &clusterrole, kubecli.UninstallOptions{})
	if err != nil {
		logError(log, "Error uninstalling clusterrole", err)
		return err
	}
	log("ClusterRole successfully uninstalled", "name", clusterrole.Name, "namespace", clusterrole.Namespace)

	err = kubecli.Uninstall(ctx, kubeClient, &clusterrolebinding, kubecli.UninstallOptions{})
	if err != nil {
		logError(log, "Error uninstalling clusterrolebinding", err)
	}
	log("ClusterRoleBinding successfully uninstalled", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace)

	err = kubecli.Uninstall(ctx, kubeClient, &role, kubecli.UninstallOptions{})
	if err != nil {
		logError(log, "Error uninstalling role", err)
		return err
	}
	log("Role successfully uninstalled", "name", role.Name, "namespace", role.Namespace)

	err = kubecli.Uninstall(ctx, kubeClient, &rolebinding, kubecli.UninstallOptions{})
	if err != nil {
		logError(log, "Error uninstalling rolebinding", err)
		return err
	}
	log("RoleBinding successfully uninstalled", "name", rolebinding.Name, "namespace", rolebinding.Namespace)

	err = kubecli.Uninstall(ctx, kubeClient, &sa, kubecli.UninstallOptions{})
	if err != nil {
		logError(log, "Error uninstalling serviceaccount", err)
		return err
	}
	log("ServiceAccount successfully uninstalled", "name", sa.Name, "namespace", sa.Namespace)

	return nil
}

func Deploy(ctx context.Context, kube client.Client, opts DeployOptions) (digest string, err error) {
	rbacNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + controllerResourceSuffix,
	}

	applyOpts := kubecli.ApplyOptions{}

	if opts.DryRunServer {
		applyOpts.DryRun = []string{"All"}
	}

	if opts.Log == nil {
		return "", fmt.Errorf("log function is required")
	}

	sa, clusterrole, clusterrolebinding, role, rolebinding, err := createRBACResources(opts.GVR, rbacNSName, opts.RBACFolderPath)
	if err != nil {
		return "", err
	}

	hsh := hasher.NewFNVObjectHash()
	if opts.Spec.Credentials != nil {
		secretNSName := types.NamespacedName{
			Namespace: opts.Spec.Credentials.PasswordRef.Namespace,
			Name:      opts.NamespacedName.Name + controllerResourceSuffix,
		}

		role, err := rbactools.CreateRole(opts.GVR, secretNSName, path.Join(opts.RBACFolderPath, "secret-role.yaml"), "secretName", opts.Spec.Credentials.PasswordRef.Name)
		if err != nil {
			logError(opts.Log, "Error creating role", err)
			return "", err
		}

		err = kubecli.Apply(ctx, kube, &role, applyOpts)
		if err != nil {
			logError(opts.Log, "Error installing role", err)
			return "", err
		}
		opts.Log("Role successfully installed", "gvr", opts.GVR.String(), "name", role.Name, "namespace", role.Namespace)

		rolebinding, err := rbactools.CreateRoleBinding(opts.GVR, secretNSName, path.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"), "serviceAccount", sa.Name, "saNamespace", sa.Namespace)
		if err != nil {
			logError(opts.Log, "Error creating rolebinding", err)
			return "", err
		}

		err = kubecli.Apply(ctx, kube, &rolebinding, applyOpts)
		if err != nil {
			logError(opts.Log, "Error installing rolebinding", err)
			return "", err
		}
		opts.Log("RoleBinding successfully installed", "gvr", opts.GVR.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace)

		err = hsh.SumHash(
			rolebinding,
			role,
		)
		if err != nil {
			return "", fmt.Errorf("error hashing rolebinding: %v", err)
		}
	}

	err = installRBACResources(ctx, opts.KubeClient, clusterrole, clusterrolebinding, role, rolebinding, sa, opts.Log, &hsh, applyOpts)
	if err != nil {
		return "", err
	}

	cmNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + configmapResourceSuffix,
	}

	cm, err := configmap.CreateConfigmap(opts.GVR, cmNSName, opts.ConfigmapTemplatePath,
		"composition_controller_sa_name", sa.Name,
		"composition_controller_sa_namespace", sa.Namespace)
	if err != nil {
		return "", err
	}

	err = kubecli.Apply(ctx, opts.KubeClient, &cm, applyOpts)
	if err != nil {
		logError(opts.Log, "Error installing configmap", err)
		return "", err
	}
	opts.Log("Configmap successfully installed", "gvr", opts.GVR.String(), "name", cm.Name, "namespace", cm.Namespace)

	deploymentNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + controllerResourceSuffix,
	}
	dep, err := deployment.CreateDeployment(
		opts.GVR,
		deploymentNSName,
		opts.DeploymentTemplatePath,
		"serviceAccountName", sa.Name)
	if err != nil {
		return "", err
	}
	err = kubecli.Apply(ctx, opts.KubeClient, &dep, applyOpts)
	if err != nil {
		logError(opts.Log, "Error installing deployment", err)
		return "", err
	}

	if !opts.DryRunServer {
		// Deployment needs to be restarted if the hash changes to get the new configmap
		err = kubecli.Get(ctx, opts.KubeClient, &dep)
		if err != nil {
			logError(opts.Log, "Error installing deployment", err)
			return "", err
		}
		// restart only if deployment is presently running
		if dep.Status.ReadyReplicas == dep.Status.Replicas {
			err = deployment.RestartDeployment(ctx, opts.KubeClient, &dep)
			if err != nil {
				logError(opts.Log, "Error restarting deployment", err)
				return "", err
			}
		}
	}

	deployment.CleanFromRestartAnnotation(&dep)

	err = hsh.SumHash(
		dep.Spec,
		cm,
	)
	if err != nil {
		return "", fmt.Errorf("error hashing deployment: %v", err)
	}

	opts.Log("Deployment successfully installed", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace)

	return hsh.GetHash(), nil
}

func Undeploy(ctx context.Context, kube client.Client, opts UndeployOptions) error {
	if opts.Log == nil {
		return fmt.Errorf("log function is required")
	}

	rbacNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + controllerResourceSuffix,
	}

	sa, clusterrole, clusterrolebinding, role, rolebinding, err := createRBACResources(opts.GVR, rbacNSName, opts.RBACFolderPath)
	if err != nil {
		return err
	}

	if !opts.SkipCRD {
		err := crd.Uninstall(ctx, opts.KubeClient, opts.GVR.GroupResource())
		if err != nil {
			opts.Log("Error uninstalling CRD", "name", opts.GVR.GroupResource().String(), "error", err)
			return err
		}
		opts.Log("CRD successfully uninstalled", "name", opts.GVR.GroupResource().String())

		labelreq, err := labels.NewRequirement(CompositionVersionLabel, selection.Equals, []string{opts.GVR.Version})
		if err != nil {
			return err
		}
		selector := labels.NewSelector().Add(*labelreq)

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

	deploymentNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + controllerResourceSuffix,
	}
	dep, err := deployment.CreateDeployment(
		opts.GVR,
		deploymentNSName,
		opts.DeploymentTemplatePath,
		"serviceAccountName", sa.Name)
	if err != nil {
		return err
	}

	err = kubecli.Uninstall(ctx, opts.KubeClient, &dep, kubecli.UninstallOptions{})
	if err != nil {
		logError(opts.Log, "Error uninstalling deployment", err)
		return err
	}
	opts.Log("Deployment successfully uninstalled", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace)

	cmNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + configmapResourceSuffix,
	}

	cm, err := configmap.CreateConfigmap(opts.GVR, cmNSName, opts.ConfigmapTemplatePath,
		"composition_controller_sa_name", sa.Name,
		"composition_controller_sa_namespace", sa.Namespace)
	if err != nil {
		return err
	}

	err = kubecli.Uninstall(ctx, opts.KubeClient, &cm, kubecli.UninstallOptions{})
	if err != nil {
		logError(opts.Log, "Error uninstalling configmap", err)
		return err
	}
	opts.Log("Configmap successfully uninstalled", "gvr", opts.GVR.String(), "name", cm.Name, "namespace", cm.Namespace)

	err = uninstallRBACResources(ctx, opts.KubeClient, clusterrole, clusterrolebinding, role, rolebinding, sa, opts.Log)
	if err != nil {
		return err
	}

	if opts.Spec.Credentials != nil {
		secretNSName := types.NamespacedName{
			Namespace: opts.Spec.Credentials.PasswordRef.Namespace,
			Name:      opts.NamespacedName.Name + controllerResourceSuffix,
		}

		role, err := rbactools.CreateRole(opts.GVR, secretNSName, path.Join(opts.RBACFolderPath, "secret-role.yaml"), "secretName", opts.Spec.Credentials.PasswordRef.Name)
		if err != nil {
			logError(opts.Log, "Error creating role", err)
			return err
		}

		err = kubecli.Uninstall(ctx, opts.KubeClient, &role, kubecli.UninstallOptions{})
		if err != nil {
			logError(opts.Log, "Error uninstalling role", err)
			return err
		}
		opts.Log("Role successfully uninstalled", "gvr", opts.GVR.String(), "name", role.Name, "namespace", role.Namespace)

		rolebinding, err := rbactools.CreateRoleBinding(opts.GVR, secretNSName, path.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"))
		if err != nil {
			logError(opts.Log, "Error creating rolebinding", err)
			return err
		}

		err = kubecli.Uninstall(ctx, opts.KubeClient, &rolebinding, kubecli.UninstallOptions{})
		if err != nil {
			logError(opts.Log, "Error uninstalling rolebinding", err)
			return err
		}
		opts.Log("RoleBinding successfully uninstalled", "gvr", opts.GVR.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace)
	}

	opts.Log("RBAC resources successfully uninstalled", "gvr", opts.GVR.String())

	return err
}

// This function is used to lookup the current state of the deployment and return the hash of the current state
// This is used to determine if the deployment needs to be updated or not
func Lookup(ctx context.Context, kube client.Client, opts DeployOptions) (digest string, err error) {
	rbacNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + controllerResourceSuffix,
	}

	if opts.Log == nil {
		return "", fmt.Errorf("log function is required")
	}

	sa, clusterrole, clusterrolebinding, role, rolebinding, err := createRBACResources(opts.GVR, rbacNSName, opts.RBACFolderPath)
	if err != nil {
		return "", err
	}

	hsh := hasher.NewFNVObjectHash()
	if opts.Spec.Credentials != nil {
		secretNSName := types.NamespacedName{
			Namespace: opts.Spec.Credentials.PasswordRef.Namespace,
			Name:      opts.NamespacedName.Name + controllerResourceSuffix,
		}

		role, err := rbactools.CreateRole(opts.GVR, secretNSName, path.Join(opts.RBACFolderPath, "secret-role.yaml"), "secretName", opts.Spec.Credentials.PasswordRef.Name)
		if err != nil {
			logError(opts.Log, "Error creating role", err)
			return "", err
		}

		err = kubecli.Get(ctx, kube, &role)
		if err != nil {
			logError(opts.Log, "Error fetching role", err)
			return "", err
		}
		opts.Log("Role successfully fetched", "gvr", opts.GVR.String(), "name", role.Name, "namespace", role.Namespace)

		rolebinding, err := rbactools.CreateRoleBinding(opts.GVR, secretNSName, path.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"), "serviceAccount", sa.Name, "saNamespace", sa.Namespace)
		if err != nil {
			logError(opts.Log, "Error creating rolebinding", err)
			return "", err
		}

		err = kubecli.Get(ctx, kube, &rolebinding)
		if err != nil {
			logError(opts.Log, "Error fetching rolebinding", err)
			return "", err
		}
		opts.Log("RoleBinding successfully fetched", "gvr", opts.GVR.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace)

		err = hsh.SumHash(
			rolebinding,
			role,
		)
		if err != nil {
			return "", fmt.Errorf("error hashing rolebinding: %v", err)
		}
	}

	err = lookupRBACResources(ctx, opts.KubeClient, clusterrole, clusterrolebinding, role, rolebinding, sa, opts.Log, &hsh)
	if err != nil {
		return "", err
	}

	cmNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + configmapResourceSuffix,
	}

	cm, err := configmap.CreateConfigmap(opts.GVR, cmNSName, opts.ConfigmapTemplatePath,
		"composition_controller_sa_name", sa.Name,
		"composition_controller_sa_namespace", sa.Namespace)
	if err != nil {
		return "", err
	}

	err = kubecli.Get(ctx, opts.KubeClient, &cm)
	if err != nil {
		logError(opts.Log, "Error fetching configmap", err)
		return "", err
	}
	opts.Log("Configmap successfully fetched", "gvr", opts.GVR.String(), "name", cm.Name, "namespace", cm.Namespace)

	deploymentNSName := types.NamespacedName{
		Namespace: opts.NamespacedName.Namespace,
		Name:      opts.NamespacedName.Name + controllerResourceSuffix,
	}
	dep, err := deployment.CreateDeployment(
		opts.GVR,
		deploymentNSName,
		opts.DeploymentTemplatePath,
		"serviceAccountName", sa.Name)
	if err != nil {
		return "", err
	}

	err = kubecli.Get(ctx, opts.KubeClient, &dep)
	if err != nil {
		logError(opts.Log, "Error fetching deployment", err)
		return "", err
	}
	opts.Log("Deployment successfully fetched", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace)

	deployment.CleanFromRestartAnnotation(&dep)

	err = hsh.SumHash(
		dep.Spec,
		cm,
	)
	if err != nil {
		return "", fmt.Errorf("error hashing deployment: %v", err)
	}

	return hsh.GetHash(), nil
}
