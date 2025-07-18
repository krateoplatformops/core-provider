package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	crd "github.com/krateoplatformops/core-provider/internal/tools/crd"
	deployment "github.com/krateoplatformops/core-provider/internal/tools/deployment"
	hasher "github.com/krateoplatformops/core-provider/internal/tools/hash"
	kubecli "github.com/krateoplatformops/core-provider/internal/tools/kube"
	"github.com/krateoplatformops/core-provider/internal/tools/objects"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	ServiceTemplatePath    string
	ConfigmapTemplatePath  string
	JsonSchemaTemplatePath string
	JsonSchemaBytes        []byte
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
	JsonSchemaTemplatePath string
	ServiceTemplatePath    string
	JsonSchemaBytes        []byte
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
	sa := corev1.ServiceAccount{}
	err := objects.CreateK8sObject(&sa, gvr, rbacNSName, filepath.Join(rbacFolderPath, "serviceaccount.yaml"))
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	clusterrole := rbacv1.ClusterRole{}
	err = objects.CreateK8sObject(&clusterrole, gvr, rbacNSName, filepath.Join(rbacFolderPath, "clusterrole.yaml"))
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	clusterrolebinding := rbacv1.ClusterRoleBinding{}
	err = objects.CreateK8sObject(&clusterrolebinding, gvr, rbacNSName, filepath.Join(rbacFolderPath, "clusterrolebinding.yaml"), "serviceAccount", sa.Name, "saNamespace", sa.Namespace)
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	role := rbacv1.Role{}
	err = objects.CreateK8sObject(&role, gvr, rbacNSName, filepath.Join(rbacFolderPath, "compositiondefinition-role.yaml"))
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	rolebinding := rbacv1.RoleBinding{}
	err = objects.CreateK8sObject(&rolebinding, gvr, rbacNSName, filepath.Join(rbacFolderPath, "compositiondefinition-rolebinding.yaml"), "serviceAccount", sa.Name, "saNamespace", sa.Namespace)
	if err != nil {
		return corev1.ServiceAccount{}, rbacv1.ClusterRole{}, rbacv1.ClusterRoleBinding{}, rbacv1.Role{}, rbacv1.RoleBinding{}, err
	}

	return sa, clusterrole, clusterrolebinding, role, rolebinding, nil
}

func installRBACResources(ctx context.Context, kubeClient client.Client, clusterrole rbacv1.ClusterRole, clusterrolebinding rbacv1.ClusterRoleBinding, role rbacv1.Role, rolebinding rbacv1.RoleBinding, sa corev1.ServiceAccount, log func(msg string, keysAndValues ...any), hsh *hasher.ObjectHash, applyOpts kubecli.ApplyOptions) error {
	if hsh == nil {
		return fmt.Errorf("hasher is required")
	}
	err := kubecli.Apply(ctx, kubeClient, &clusterrole, applyOpts)
	if err != nil {
		logError(log, "Error installing clusterrole", err)
		return err
	}

	err = hsh.SumHash(clusterrole.ObjectMeta.Name, clusterrole.ObjectMeta.Namespace, clusterrole.Rules)
	if err != nil {
		return fmt.Errorf("error hashing clusterrole: %v", err)
	}
	log("ClusterRole successfully hashed", "name", clusterrole.Name, "namespace", clusterrole.Namespace, "digest", hsh.GetHash())

	err = kubecli.Apply(ctx, kubeClient, &clusterrolebinding, applyOpts)
	if err != nil {
		logError(log, "Error installing clusterrolebinding", err)
		return err
	}
	err = hsh.SumHash(clusterrolebinding.ObjectMeta.Name, clusterrolebinding.ObjectMeta.Namespace, clusterrolebinding.Subjects, clusterrolebinding.RoleRef)
	if err != nil {
		return fmt.Errorf("error hashing clusterrolebinding: %v", err)
	}
	log("ClusterRoleBinding successfully installesd", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace, "digest", hsh.GetHash())

	err = kubecli.Apply(ctx, kubeClient, &role, applyOpts)
	if err != nil {
		logError(log, "Error installing role", err)
		return err
	}
	err = hsh.SumHash(role.ObjectMeta.Name, role.ObjectMeta.Namespace, role.Rules)
	if err != nil {
		return fmt.Errorf("error hashing role: %v", err)
	}
	log("Role successfully installed", "name", role.Name, "namespace", role.Namespace, "digest", hsh.GetHash())

	err = kubecli.Apply(ctx, kubeClient, &rolebinding, applyOpts)
	if err != nil {
		logError(log, "Error installing rolebinding", err)
		return err
	}
	err = hsh.SumHash(rolebinding.ObjectMeta.Name, rolebinding.ObjectMeta.Namespace, rolebinding.Subjects, rolebinding.RoleRef)
	if err != nil {
		return fmt.Errorf("error hashing rolebinding: %v", err)
	}

	log("RoleBinding successfully installed", "name", rolebinding.Name, "namespace", rolebinding.Namespace, "digest", hsh.GetHash())

	err = kubecli.Apply(ctx, kubeClient, &sa, applyOpts)
	if err != nil {
		logError(log, "Error installing serviceaccount", err)
		return err
	}
	err = hsh.SumHash(sa.ObjectMeta.Name, sa.ObjectMeta.Namespace)
	if err != nil {
		return fmt.Errorf("error hashing serviceaccount: %v", err)
	}
	log("ServiceAccount successfully installed", "name", sa.Name, "namespace", sa.Namespace, "digest", hsh.GetHash())

	return nil
}

func lookupRBACResources(ctx context.Context, kubeClient client.Client, clusterrole rbacv1.ClusterRole, clusterrolebinding rbacv1.ClusterRoleBinding, role rbacv1.Role, rolebinding rbacv1.RoleBinding, sa corev1.ServiceAccount, log func(msg string, keysAndValues ...any), hsh *hasher.ObjectHash) error {
	if hsh == nil {
		return fmt.Errorf("hasher is required")
	}
	err := kubecli.Get(ctx, kubeClient, &clusterrole)
	if err != nil {
		logError(log, "Error getting clusterrole", err)
		return err
	}
	err = hsh.SumHash(clusterrole.ObjectMeta.Name, clusterrole.ObjectMeta.Namespace, clusterrole.Rules)
	if err != nil {
		return fmt.Errorf("error hashing clusterrole: %v", err)
	}
	log("ClusterRole successfully fetched", "name", clusterrole.Name, "namespace", clusterrole.Namespace, "digest", hsh.GetHash())

	err = kubecli.Get(ctx, kubeClient, &clusterrolebinding)
	if err != nil {
		logError(log, "Error getting clusterrolebinding", err)
		return err
	}
	err = hsh.SumHash(clusterrolebinding.ObjectMeta.Name, clusterrolebinding.ObjectMeta.Namespace, clusterrolebinding.Subjects, clusterrolebinding.RoleRef)
	if err != nil {
		return fmt.Errorf("error hashing clusterrolebinding: %v", err)
	}
	log("ClusterRoleBinding successfully fetched", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace, "digest", hsh.GetHash())

	err = kubecli.Get(ctx, kubeClient, &role)
	if err != nil {
		logError(log, "Error getting role", err)
		return err
	}
	err = hsh.SumHash(role.ObjectMeta.Name, role.ObjectMeta.Namespace, role.Rules)
	if err != nil {
		return fmt.Errorf("error hashing role: %v", err)
	}
	log("Role successfully fetched", "name", role.Name, "namespace", role.Namespace, "digest", hsh.GetHash())

	err = kubecli.Get(ctx, kubeClient, &rolebinding)
	if err != nil {
		logError(log, "Error getting rolebinding", err)
		return err
	}
	err = hsh.SumHash(rolebinding.ObjectMeta.Name, rolebinding.ObjectMeta.Namespace, rolebinding.Subjects, rolebinding.RoleRef)
	if err != nil {
		return fmt.Errorf("error hashing rolebinding: %v", err)
	}
	log("RoleBinding successfully fetched", "name", rolebinding.Name, "namespace", rolebinding.Namespace, "digest", hsh.GetHash())

	err = kubecli.Get(ctx, kubeClient, &sa)
	if err != nil {
		logError(log, "Error getting serviceaccount", err)
		return err
	}
	err = hsh.SumHash(sa.ObjectMeta.Name, sa.ObjectMeta.Namespace)
	if err != nil {
		return fmt.Errorf("error hashing serviceaccount: %v", err)
	}
	log("ServiceAccount successfully fetched", "name", sa.Name, "namespace", sa.Namespace, "digest", hsh.GetHash())

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
	applyOpts := kubecli.ApplyOptions{}

	if opts.DryRunServer {
		applyOpts.DryRun = []string{"All"}
	}

	if opts.Log == nil {
		return "", fmt.Errorf("log function is required")
	}

	sa, clusterrole, clusterrolebinding, role, rolebinding, err := createRBACResources(opts.GVR, getCDCrbacNN(opts.NamespacedName), opts.RBACFolderPath)
	if err != nil {
		return "", err
	}

	hsh := hasher.NewFNVObjectHash()
	if opts.Spec.Credentials != nil {
		role := rbacv1.Role{}
		err = objects.CreateK8sObject(&role,
			opts.GVR,
			getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: opts.NamespacedName.Name}),
			filepath.Join(opts.RBACFolderPath, "secret-role.yaml"),
			"secretName", opts.Spec.Credentials.PasswordRef.Name)
		if err != nil {
			logError(opts.Log, "Error creating role", err)
			return "", err
		}

		err = kubecli.Apply(ctx, kube, &role, applyOpts)
		if err != nil {
			logError(opts.Log, "Error installing role", err)
			return "", err
		}
		err = hsh.SumHash(role.ObjectMeta.Name, role.ObjectMeta.Namespace, role.Rules)
		if err != nil {
			return "", fmt.Errorf("error hashing role: %v", err)
		}

		opts.Log("Role successfully hashed", "gvr", opts.GVR.String(), "name", role.Name, "namespace", role.Namespace, "digest", hsh.GetHash())

		rolebinding := rbacv1.RoleBinding{}
		err := objects.CreateK8sObject(&rolebinding,
			opts.GVR,
			getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: opts.NamespacedName.Name}),
			filepath.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"),
			"serviceAccount", sa.Name,
			"saNamespace", sa.Namespace)
		if err != nil {
			logError(opts.Log, "Error creating rolebinding", err)
			return "", err
		}

		err = kubecli.Apply(ctx, kube, &rolebinding, applyOpts)
		if err != nil {
			logError(opts.Log, "Error installing rolebinding", err)
			return "", err
		}
		err = hsh.SumHash(rolebinding.ObjectMeta.Name, rolebinding.ObjectMeta.Namespace, rolebinding.Subjects, rolebinding.RoleRef)
		if err != nil {
			return "", fmt.Errorf("error hashing rolebinding: %v", err)
		}
		opts.Log("RoleBinding successfully installed", "gvr", opts.GVR.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace, "digest", hsh.GetHash())
	}

	err = installRBACResources(ctx, opts.KubeClient, clusterrole, clusterrolebinding, role, rolebinding, sa, opts.Log, &hsh, applyOpts)
	if err != nil {
		return "", err
	}

	jsonSchemaConfigmap := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&jsonSchemaConfigmap, opts.GVR, getJsonSchemaConfigmapNN(opts.NamespacedName), opts.JsonSchemaTemplatePath,
		"schema", string(opts.JsonSchemaBytes),
	)
	if err != nil {
		return "", fmt.Errorf("error creating ConfigMap for JSON schema: %w", err)
	}
	err = kubecli.Apply(ctx, opts.KubeClient, &jsonSchemaConfigmap, applyOpts)
	if err != nil {
		return "", fmt.Errorf("error applying ConfigMap for JSON schema: %w", err)
	}
	err = hsh.SumHash(jsonSchemaConfigmap.ObjectMeta.Name, jsonSchemaConfigmap.ObjectMeta.Namespace)
	if err != nil {
		return "", fmt.Errorf("error hashing JSON schema configmap: %v", err)
	}
	opts.Log("JSON Schema ConfigMap successfully installed", "gvr", opts.GVR.String(), "name", jsonSchemaConfigmap.Name, "namespace", jsonSchemaConfigmap.Namespace, "digest", hsh.GetHash())

	cm := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&cm, opts.GVR, getCDCConfigmapNN(opts.NamespacedName), opts.ConfigmapTemplatePath,
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
	err = hsh.SumHash(cm.ObjectMeta.Name, cm.ObjectMeta.Namespace, cm.Data)
	if err != nil {
		return "", fmt.Errorf("error hashing configmap: %v", err)
	}
	opts.Log("Configmap successfully installed", "gvr", opts.GVR.String(), "name", cm.Name, "namespace", cm.Namespace, "digest", hsh.GetHash())

	dep := appsv1.Deployment{}
	err = objects.CreateK8sObject(
		&dep,
		opts.GVR,
		getCDCDeploymentNN(opts.NamespacedName),
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

	err = hsh.SumHash(dep.ObjectMeta.Name, dep.ObjectMeta.Namespace, dep.Spec)
	if err != nil {
		return "", fmt.Errorf("error hashing deployment spec: %v", err)
	}
	opts.Log("Deployment successfully installed", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace, "digest", hsh.GetHash())

	_, err = os.Stat(opts.ServiceTemplatePath)
	if err == nil {
		svc := corev1.Service{}
		err = objects.CreateK8sObject(&svc, opts.GVR, getCDCDeploymentNN(opts.NamespacedName), opts.ServiceTemplatePath)
		if err != nil {
			logError(opts.Log, "Error creating service", err)
			return "", err
		}

		err = kubecli.Apply(ctx, opts.KubeClient, &svc, applyOpts)
		if err != nil {
			logError(opts.Log, "Error installing service", err)
			return "", err
		}
		err = hsh.SumHash(svc.ObjectMeta.Name, svc.ObjectMeta.Namespace, svc.Spec)
		if err != nil {
			return "", fmt.Errorf("error hashing service: %v", err)
		}
		opts.Log("Service successfully installed", "gvr", opts.GVR.String(), "name", svc.Name, "namespace", svc.Namespace, "digest", hsh.GetHash())
	}

	return hsh.GetHash(), nil
}

func Undeploy(ctx context.Context, kube client.Client, opts UndeployOptions) error {
	if opts.Log == nil {
		return fmt.Errorf("log function is required")
	}

	sa, clusterrole, clusterrolebinding, role, rolebinding, err := createRBACResources(opts.GVR, getCDCrbacNN(opts.NamespacedName), opts.RBACFolderPath)
	if err != nil {
		return err
	}

	jsonSchemaConfigmap := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&jsonSchemaConfigmap, opts.GVR, getJsonSchemaConfigmapNN(opts.NamespacedName), opts.JsonSchemaTemplatePath,
		"schema", string(opts.JsonSchemaBytes),
	)
	if err != nil {
		return fmt.Errorf("error creating ConfigMap for JSON schema: %w", err)
	}
	err = kubecli.Uninstall(ctx, opts.KubeClient, &jsonSchemaConfigmap, kubecli.UninstallOptions{})
	if err != nil {
		logError(opts.Log, "Error uninstalling ConfigMap for JSON schema", err)
		return err
	}
	opts.Log("JSON Schema ConfigMap successfully uninstalled", "gvr", opts.GVR.String(), "name", jsonSchemaConfigmap.Name, "namespace", jsonSchemaConfigmap.Namespace)

	dep := appsv1.Deployment{}
	err = objects.CreateK8sObject(
		&dep,
		opts.GVR,
		getCDCDeploymentNN(opts.NamespacedName),
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

	cm := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&cm, opts.GVR, getCDCConfigmapNN(opts.NamespacedName), opts.ConfigmapTemplatePath,
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

	_, err = os.Stat(opts.ServiceTemplatePath)
	if err == nil {
		svc := corev1.Service{}
		err = objects.CreateK8sObject(&svc, opts.GVR, getCDCDeploymentNN(opts.NamespacedName), opts.ServiceTemplatePath)
		if err != nil {
			logError(opts.Log, "Error creating service", err)
			return err
		}

		err = kubecli.Uninstall(ctx, opts.KubeClient, &svc, kubecli.UninstallOptions{})
		if err != nil {
			logError(opts.Log, "Error uninstalling service", err)
			return err
		}
		opts.Log("Service successfully uninstalled", "gvr", opts.GVR.String(), "name", svc.Name, "namespace", svc.Namespace)
	}

	if opts.Spec.Credentials != nil {
		role := rbacv1.Role{}
		err = objects.CreateK8sObject(&role, opts.GVR, getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: opts.NamespacedName.Name}), filepath.Join(opts.RBACFolderPath, "secret-role.yaml"), "secretName", opts.Spec.Credentials.PasswordRef.Name)
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

		rolebinding := rbacv1.RoleBinding{}
		err = objects.CreateK8sObject(&rolebinding, opts.GVR, getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: opts.NamespacedName.Name}), filepath.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"))
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

	if !opts.SkipCRD {
		err := crd.Uninstall(ctx, opts.KubeClient, opts.GVR.GroupResource())
		if err != nil {
			opts.Log("Error uninstalling CRD", "name", opts.GVR.GroupResource().String(), "error", err)
			return err
		}
		opts.Log("CRD successfully uninstalled", "name", opts.GVR.GroupResource().String())
	}

	return nil
}

// This function is used to lookup the current state of the deployment and return the hash of the current state
// This is used to determine if the deployment needs to be updated or not
func Lookup(ctx context.Context, kube client.Client, opts DeployOptions) (digest string, err error) {
	if opts.Log == nil {
		return "", fmt.Errorf("log function is required")
	}

	sa, clusterrole, clusterrolebinding, role, rolebinding, err := createRBACResources(opts.GVR, getCDCrbacNN(opts.NamespacedName), opts.RBACFolderPath)
	if err != nil {
		return "", err
	}

	hsh := hasher.NewFNVObjectHash()
	if opts.Spec.Credentials != nil {
		role := rbacv1.Role{}
		err = objects.CreateK8sObject(&role, opts.GVR, getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: opts.NamespacedName.Name}), filepath.Join(opts.RBACFolderPath, "secret-role.yaml"), "secretName", opts.Spec.Credentials.PasswordRef.Name)
		if err != nil {
			logError(opts.Log, "Error creating role", err)
			return "", err
		}

		err = kubecli.Get(ctx, kube, &role)
		if err != nil {
			logError(opts.Log, "Error fetching role", err)
			return "", err
		}
		err = hsh.SumHash(role.ObjectMeta.Name, role.ObjectMeta.Namespace, role.Rules)
		if err != nil {
			return "", fmt.Errorf("error hashing role: %v", err)
		}
		opts.Log("Role successfully fetched", "gvr", opts.GVR.String(), "name", role.Name, "namespace", role.Namespace, "digest", hsh.GetHash())

		rolebinding := rbacv1.RoleBinding{}
		err = objects.CreateK8sObject(&rolebinding, opts.GVR, getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: opts.NamespacedName.Name}), filepath.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"), "serviceAccount", sa.Name, "saNamespace", sa.Namespace)
		if err != nil {
			logError(opts.Log, "Error creating rolebinding", err)
			return "", err
		}

		err = kubecli.Get(ctx, kube, &rolebinding)
		if err != nil {
			logError(opts.Log, "Error fetching rolebinding", err)
			return "", err
		}
		err = hsh.SumHash(rolebinding.ObjectMeta.Name, rolebinding.ObjectMeta.Namespace, rolebinding.Subjects, rolebinding.RoleRef)
		if err != nil {
			return "", fmt.Errorf("error hashing rolebinding: %v", err)
		}
		opts.Log("RoleBinding successfully fetched", "gvr", opts.GVR.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace, "digest", hsh.GetHash())
	}

	err = lookupRBACResources(ctx, opts.KubeClient, clusterrole, clusterrolebinding, role, rolebinding, sa, opts.Log, &hsh)
	if err != nil {
		return "", err
	}

	jsonSchemaConfigmap := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&jsonSchemaConfigmap, opts.GVR, getJsonSchemaConfigmapNN(opts.NamespacedName), opts.JsonSchemaTemplatePath,
		"schema", string(opts.JsonSchemaBytes),
	)
	if err != nil {
		return "", fmt.Errorf("error creating ConfigMap for JSON schema: %w", err)
	}
	err = kubecli.Get(ctx, opts.KubeClient, &jsonSchemaConfigmap)
	if err != nil {
		return "", fmt.Errorf("error fetching ConfigMap for JSON schema: %w", err)
	}
	err = hsh.SumHash(jsonSchemaConfigmap.ObjectMeta.Name, jsonSchemaConfigmap.ObjectMeta.Namespace)
	if err != nil {
		return "", fmt.Errorf("error hashing JSON schema configmap: %v", err)
	}
	opts.Log("JSON Schema ConfigMap successfully fetched", "gvr", opts.GVR.String(), "name", jsonSchemaConfigmap.Name, "namespace", jsonSchemaConfigmap.Namespace, "digest", hsh.GetHash())

	cm := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&cm, opts.GVR, getCDCConfigmapNN(opts.NamespacedName), opts.ConfigmapTemplatePath,
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
	err = hsh.SumHash(cm.ObjectMeta.Name, cm.ObjectMeta.Namespace, cm.Data)
	if err != nil {
		return "", fmt.Errorf("error hashing configmap: %v", err)
	}
	opts.Log("Configmap successfully fetched", "gvr", opts.GVR.String(), "name", cm.Name, "namespace", cm.Namespace, "digest", hsh.GetHash())

	dep := appsv1.Deployment{}
	err = objects.CreateK8sObject(
		&dep,
		opts.GVR,
		getCDCDeploymentNN(opts.NamespacedName),
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

	deployment.CleanFromRestartAnnotation(&dep)

	err = hsh.SumHash(dep.ObjectMeta.Name, dep.ObjectMeta.Namespace, dep.Spec)
	if err != nil {
		return "", fmt.Errorf("error hashing deployment spec: %v", err)
	}
	opts.Log("Deployment successfully fetched", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace, "digest", hsh.GetHash())

	_, err = os.Stat(opts.ServiceTemplatePath)
	if err == nil {
		svc := corev1.Service{}
		err = objects.CreateK8sObject(&svc, opts.GVR, getCDCDeploymentNN(opts.NamespacedName), opts.ServiceTemplatePath)
		if err != nil {
			logError(opts.Log, "Error creating service", err)
			return "", err
		}
		err = kubecli.Get(ctx, opts.KubeClient, &svc)
		if err != nil {
			logError(opts.Log, "Error fetching service", err)
			return "", err
		}
		err = hsh.SumHash(svc.ObjectMeta.Name, svc.ObjectMeta.Namespace, svc.Spec)
		if err != nil {
			return "", fmt.Errorf("error hashing service: %v", err)
		}
		opts.Log("Service successfully fetched", "gvr", opts.GVR.String(), "name", svc.Name, "namespace", svc.Namespace, "digest", hsh.GetHash())
	}

	return hsh.GetHash(), nil
}

func getCDCConfigmapNN(nn types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: nn.Namespace,
		Name:      nn.Name + configmapResourceSuffix,
	}
}

func getCDCDeploymentNN(nn types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: nn.Namespace,
		Name:      nn.Name + controllerResourceSuffix,
	}
}
func getCDCrbacNN(nn types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: nn.Namespace,
		Name:      nn.Name + controllerResourceSuffix,
	}
}
func getJsonSchemaConfigmapNN(nn types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: nn.Namespace,
		Name:      nn.Name + "-jsonschema" + configmapResourceSuffix,
	}
}

func getServiceNN(nn types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: nn.Namespace,
		Name:      nn.Name + "-service",
	}
}
