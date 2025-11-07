package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	contexttools "github.com/krateoplatformops/core-provider/internal/tools/context"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"

	"github.com/krateoplatformops/core-provider/internal/tools/kube/watcher"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

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
	Namespace              string
	GVR                    schema.GroupVersionResource
	Spec                   *definitionsv1alpha1.ChartInfo
	RBACFolderPath         string
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
	DynClient              dynamic.Interface
	Namespace              string
	Spec                   *definitionsv1alpha1.ChartInfo
	RBACFolderPath         string
	DeploymentTemplatePath string
	ConfigmapTemplatePath  string
	JsonSchemaTemplatePath string
	ServiceTemplatePath    string
	JsonSchemaBytes        []byte
	// DryRunServer is used to determine if the deployment should be applied in dry-run mode. This is ignored in lookup mode
	DryRunServer bool
}

func resourceNamer(resourceName string, chartVersion string) string {
	return fmt.Sprintf("%s-%s", resourceName, chartVersion)
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

func installRBACResources(ctx context.Context, kubeClient client.Client, clusterrole rbacv1.ClusterRole, clusterrolebinding rbacv1.ClusterRoleBinding, role rbacv1.Role, rolebinding rbacv1.RoleBinding, sa corev1.ServiceAccount, hsh *hasher.ObjectHash, applyOpts kubecli.ApplyOptions) error {
	if hsh == nil {
		return fmt.Errorf("hasher is required")
	}
	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())
	err := kubecli.Apply(ctx, kubeClient, &clusterrole, applyOpts)
	if err != nil {
		log.Error(err, "installing clusterrole", "name", clusterrole.Name, "namespace", clusterrole.Namespace)
		return err
	}

	err = hsh.SumHash(clusterrole.ObjectMeta.Name, clusterrole.ObjectMeta.Namespace, clusterrole.Rules)
	if err != nil {
		return fmt.Errorf("error hashing clusterrole: %v", err)
	}
	log.Debug("ClusterRole successfully hashed", "name", clusterrole.Name, "namespace", clusterrole.Namespace, "digest", hsh.GetHash())

	err = kubecli.Apply(ctx, kubeClient, &clusterrolebinding, applyOpts)
	if err != nil {
		log.Error(err, "installing clusterrolebinding", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace)
		return err
	}
	err = hsh.SumHash(clusterrolebinding.ObjectMeta.Name, clusterrolebinding.ObjectMeta.Namespace, clusterrolebinding.Subjects, clusterrolebinding.RoleRef)
	if err != nil {
		return fmt.Errorf("error hashing clusterrolebinding: %v", err)
	}
	log.Debug("ClusterRoleBinding successfully installed", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace, "digest", hsh.GetHash())

	err = kubecli.Apply(ctx, kubeClient, &role, applyOpts)
	if err != nil {
		log.Error(err, "installing role", "name", role.Name, "namespace", role.Namespace)
		return err
	}
	err = hsh.SumHash(role.ObjectMeta.Name, role.ObjectMeta.Namespace, role.Rules)
	if err != nil {
		return fmt.Errorf("error hashing role: %v", err)
	}
	log.Debug("Role successfully installed", "name", role.Name, "namespace", role.Namespace, "digest", hsh.GetHash())

	err = kubecli.Apply(ctx, kubeClient, &rolebinding, applyOpts)
	if err != nil {
		log.Error(err, "installing rolebinding", "name", rolebinding.Name, "namespace", rolebinding.Namespace)
		return err
	}
	err = hsh.SumHash(rolebinding.ObjectMeta.Name, rolebinding.ObjectMeta.Namespace, rolebinding.Subjects, rolebinding.RoleRef)
	if err != nil {
		return fmt.Errorf("error hashing rolebinding: %v", err)
	}

	log.Debug("RoleBinding successfully installed", "name", rolebinding.Name, "namespace", rolebinding.Namespace, "digest", hsh.GetHash())

	err = kubecli.Apply(ctx, kubeClient, &sa, applyOpts)
	if err != nil {
		log.Error(err, "installing serviceaccount", "name", sa.Name, "namespace", sa.Namespace)
		return err
	}
	err = hsh.SumHash(sa.ObjectMeta.Name, sa.ObjectMeta.Namespace)
	if err != nil {
		return fmt.Errorf("error hashing serviceaccount: %v", err)
	}
	log.Debug("ServiceAccount successfully installed", "name", sa.Name, "namespace", sa.Namespace, "digest", hsh.GetHash())

	return nil
}

func lookupRBACResources(ctx context.Context, kubeClient client.Client, clusterrole rbacv1.ClusterRole, clusterrolebinding rbacv1.ClusterRoleBinding, role rbacv1.Role, rolebinding rbacv1.RoleBinding, sa corev1.ServiceAccount, hsh *hasher.ObjectHash) error {
	if hsh == nil {
		return fmt.Errorf("hasher is required")
	}
	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())

	err := kubecli.Get(ctx, kubeClient, &clusterrole)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("ClusterRole not found", "name", clusterrole.Name, "namespace", clusterrole.Namespace)
			clusterrole = rbacv1.ClusterRole{}
		} else {
			return fmt.Errorf("error getting clusterrole: %w", err)
		}
	}
	err = hsh.SumHash(clusterrole.ObjectMeta.Name, clusterrole.ObjectMeta.Namespace, clusterrole.Rules)
	if err != nil {
		return fmt.Errorf("error hashing clusterrole: %v", err)
	}
	log.Debug("ClusterRole successfully fetched", "name", clusterrole.Name, "namespace", clusterrole.Namespace, "digest", hsh.GetHash())

	err = kubecli.Get(ctx, kubeClient, &clusterrolebinding)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("ClusterRoleBinding not found", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace)
			clusterrolebinding = rbacv1.ClusterRoleBinding{}
		} else {
			return fmt.Errorf("error getting clusterrolebinding: %w", err)
		}
	}
	err = hsh.SumHash(clusterrolebinding.ObjectMeta.Name, clusterrolebinding.ObjectMeta.Namespace, clusterrolebinding.Subjects, clusterrolebinding.RoleRef)
	if err != nil {
		return fmt.Errorf("error hashing clusterrolebinding: %v", err)
	}
	log.Debug("ClusterRoleBinding successfully fetched", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace, "digest", hsh.GetHash())

	err = kubecli.Get(ctx, kubeClient, &role)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Role not found", "name", role.Name, "namespace", role.Namespace)
			role = rbacv1.Role{}
		} else {
			return fmt.Errorf("error getting role: %w", err)
		}
	}
	err = hsh.SumHash(role.ObjectMeta.Name, role.ObjectMeta.Namespace, role.Rules)
	if err != nil {
		return fmt.Errorf("error hashing role: %v", err)
	}
	log.Debug("Role successfully fetched", "name", role.Name, "namespace", role.Namespace, "digest", hsh.GetHash())

	err = kubecli.Get(ctx, kubeClient, &rolebinding)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("RoleBinding not found", "name", rolebinding.Name, "namespace", rolebinding.Namespace)
			rolebinding = rbacv1.RoleBinding{}
		} else {
			return fmt.Errorf("error getting rolebinding: %w", err)
		}
	}
	err = hsh.SumHash(rolebinding.ObjectMeta.Name, rolebinding.ObjectMeta.Namespace, rolebinding.Subjects, rolebinding.RoleRef)
	if err != nil {
		return fmt.Errorf("error hashing rolebinding: %v", err)
	}
	log.Debug("RoleBinding successfully fetched", "name", rolebinding.Name, "namespace", rolebinding.Namespace, "digest", hsh.GetHash())

	err = kubecli.Get(ctx, kubeClient, &sa)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("ServiceAccount not found", "name", sa.Name, "namespace", sa.Namespace)
			sa = corev1.ServiceAccount{}
		} else {
			return fmt.Errorf("error getting serviceaccount: %w", err)
		}
	}
	err = hsh.SumHash(sa.ObjectMeta.Name, sa.ObjectMeta.Namespace)
	if err != nil {
		return fmt.Errorf("error hashing serviceaccount: %v", err)
	}
	log.Debug("ServiceAccount successfully fetched", "name", sa.Name, "namespace", sa.Namespace, "digest", hsh.GetHash())

	return nil
}

func uninstallRBACResources(ctx context.Context, kubeClient client.Client, clusterrole rbacv1.ClusterRole, clusterrolebinding rbacv1.ClusterRoleBinding, role rbacv1.Role, rolebinding rbacv1.RoleBinding, sa corev1.ServiceAccount) error {
	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())

	err := kubecli.Uninstall(ctx, kubeClient, &clusterrole, kubecli.UninstallOptions{})
	if err != nil {
		log.Error(err, "Error uninstalling clusterrole", "name", clusterrole.Name, "namespace", clusterrole.Namespace)
		return err
	}
	log.Debug("ClusterRole successfully uninstalled", "name", clusterrole.Name, "namespace", clusterrole.Namespace)

	err = kubecli.Uninstall(ctx, kubeClient, &clusterrolebinding, kubecli.UninstallOptions{})
	if err != nil {
		log.Error(err, "Error uninstalling clusterrolebinding", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace)
		return err
	}
	log.Debug("ClusterRoleBinding successfully uninstalled", "name", clusterrolebinding.Name, "namespace", clusterrolebinding.Namespace)

	err = kubecli.Uninstall(ctx, kubeClient, &role, kubecli.UninstallOptions{})
	if err != nil {
		log.Error(err, "uninstalling role", "name", role.Name, "namespace", role.Namespace)
		return err
	}
	log.Debug("Role successfully uninstalled", "name", role.Name, "namespace", role.Namespace)

	err = kubecli.Uninstall(ctx, kubeClient, &rolebinding, kubecli.UninstallOptions{})
	if err != nil {
		log.Error(err, "uninstalling rolebinding", "name", rolebinding.Name, "namespace", rolebinding.Namespace)
		return err
	}
	log.Debug("RoleBinding successfully uninstalled", "name", rolebinding.Name, "namespace", rolebinding.Namespace)

	err = kubecli.Uninstall(ctx, kubeClient, &sa, kubecli.UninstallOptions{})
	if err != nil {
		log.Error(err, "uninstalling serviceaccount", "name", sa.Name, "namespace", sa.Namespace)
		return err
	}
	log.Debug("ServiceAccount successfully uninstalled", "name", sa.Name, "namespace", sa.Namespace)

	return nil
}

func Deploy(ctx context.Context, kube client.Client, opts DeployOptions) (digest string, err error) {
	applyOpts := kubecli.ApplyOptions{}

	namespacedName := types.NamespacedName{
		Namespace: opts.Namespace,
		Name:      resourceNamer(opts.GVR.Resource, opts.GVR.Version),
	}

	if opts.DryRunServer {
		applyOpts.DryRun = []string{"All"}
	}

	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())

	sa, clusterrole, clusterrolebinding, role, rolebinding, err := createRBACResources(opts.GVR, getCDCrbacNN(namespacedName), opts.RBACFolderPath)
	if err != nil {
		return "", err
	}

	hsh := hasher.NewFNVObjectHash()
	if opts.Spec.Credentials != nil {
		role := rbacv1.Role{}
		err = objects.CreateK8sObject(&role,
			opts.GVR,
			getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: namespacedName.Name}),
			filepath.Join(opts.RBACFolderPath, "secret-role.yaml"),
			"secretName", opts.Spec.Credentials.PasswordRef.Name)
		if err != nil {
			log.Error(err, "creating role")
			return "", err
		}

		err = kubecli.Apply(ctx, kube, &role, applyOpts)
		if err != nil {
			log.Error(err, "installing role")
			return "", err
		}
		err = hsh.SumHash(role.ObjectMeta.Name, role.ObjectMeta.Namespace, role.Rules)
		if err != nil {
			return "", fmt.Errorf("error hashing role: %v", err)
		}

		log.Debug("Role successfully hashed", "gvr", opts.GVR.String(), "name", role.Name, "namespace", role.Namespace, "digest", hsh.GetHash())

		rolebinding := rbacv1.RoleBinding{}
		err := objects.CreateK8sObject(&rolebinding,
			opts.GVR,
			getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: namespacedName.Name}),
			filepath.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"),
			"serviceAccount", sa.Name,
			"saNamespace", sa.Namespace)
		if err != nil {
			log.Error(err, "creating rolebinding")
			return "", err
		}

		err = kubecli.Apply(ctx, kube, &rolebinding, applyOpts)
		if err != nil {
			log.Error(err, "installing rolebinding")
			return "", err
		}
		err = hsh.SumHash(rolebinding.ObjectMeta.Name, rolebinding.ObjectMeta.Namespace, rolebinding.Subjects, rolebinding.RoleRef)
		if err != nil {
			return "", fmt.Errorf("error hashing rolebinding: %v", err)
		}
		log.Debug("RoleBinding successfully installed", "gvr", opts.GVR.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace, "digest", hsh.GetHash())
	}

	err = installRBACResources(ctx, opts.KubeClient, clusterrole, clusterrolebinding, role, rolebinding, sa, &hsh, applyOpts)
	if err != nil {
		return "", err
	}

	jsonSchemaConfigmap := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&jsonSchemaConfigmap, opts.GVR, getJsonSchemaConfigmapNN(namespacedName), opts.JsonSchemaTemplatePath,
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
	log.Debug("JSON Schema ConfigMap successfully installed", "gvr", opts.GVR.String(), "name", jsonSchemaConfigmap.Name, "namespace", jsonSchemaConfigmap.Namespace, "digest", hsh.GetHash())

	cm := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&cm, opts.GVR, getCDCConfigmapNN(namespacedName), opts.ConfigmapTemplatePath,
		"composition_controller_sa_name", sa.Name,
		"composition_controller_sa_namespace", sa.Namespace)
	if err != nil {
		return "", err
	}
	err = kubecli.Apply(ctx, opts.KubeClient, &cm, applyOpts)
	if err != nil {
		log.Error(err, "installing configmap")
		return "", err
	}
	err = hsh.SumHash(cm.ObjectMeta.Name, cm.ObjectMeta.Namespace, cm.Data)
	if err != nil {
		return "", fmt.Errorf("error hashing configmap: %v", err)
	}
	log.Debug("Configmap successfully installed", "gvr", opts.GVR.String(), "name", cm.Name, "namespace", cm.Namespace, "digest", hsh.GetHash())

	dep := appsv1.Deployment{}
	err = objects.CreateK8sObject(
		&dep,
		opts.GVR,
		getCDCDeploymentNN(namespacedName),
		opts.DeploymentTemplatePath,
		"serviceAccountName", sa.Name)
	if err != nil {
		return "", err
	}
	err = kubecli.Apply(ctx, opts.KubeClient, &dep, applyOpts)
	if err != nil {
		log.Error(err, "installing deployment")
		return "", err
	}

	deployment.CleanFromRestartAnnotation(&dep)

	err = hsh.SumHash(dep.ObjectMeta.Name, dep.ObjectMeta.Namespace, dep.Spec)
	if err != nil {
		return "", fmt.Errorf("error hashing deployment spec: %v", err)
	}
	log.Debug("Deployment successfully installed", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace, "digest", hsh.GetHash())

	_, err = os.Stat(opts.ServiceTemplatePath)
	if err == nil {
		svc := corev1.Service{}
		err = objects.CreateK8sObject(&svc, opts.GVR, getCDCDeploymentNN(namespacedName), opts.ServiceTemplatePath)
		if err != nil {
			log.Error(err, "creating service")
			return "", err
		}

		err = kubecli.Apply(ctx, opts.KubeClient, &svc, applyOpts)
		if err != nil {
			log.Error(err, "installing service")
			return "", err
		}
		err = hsh.SumHash(svc.ObjectMeta.Name, svc.ObjectMeta.Namespace, svc.Spec)
		if err != nil {
			return "", fmt.Errorf("error hashing service: %v", err)
		}
		log.Debug("Service successfully installed", "gvr", opts.GVR.String(), "name", svc.Name, "namespace", svc.Namespace, "digest", hsh.GetHash())
	}

	if !opts.DryRunServer {
		// Wait for deployment to be ready after creation
		err := watcher.NewWatcher(opts.DynClient,
			appsv1.SchemeGroupVersion.WithResource("deployments"),
			1*time.Minute,
			deployment.IsReady).WatchResource(ctx, dep.GetNamespace(), dep.GetName())
		if err != nil {
			log.Error(err, "waiting for deployment to be ready")
			return "", fmt.Errorf("error waiting for deployment to be ready: %w", err)
		}

		// Deployment needs to be restarted if the hash changes to get the new configmap
		err = deployment.RestartDeployment(ctx, opts.KubeClient, &dep)
		if err != nil {
			log.Error(err, "restarting deployment")
			return "", err
		}

		// Wait for deployment to be ready after restart
		err = watcher.NewWatcher(opts.DynClient,
			appsv1.SchemeGroupVersion.WithResource("deployments"),
			1*time.Minute,
			deployment.IsReady).WatchResource(ctx, dep.GetNamespace(), dep.GetName())
		if err != nil {
			log.Error(err, "waiting for deployment to be ready")
			return "", fmt.Errorf("waiting for deployment to be ready: %w", err)
		}
	}

	return hsh.GetHash(), nil
}

func Undeploy(ctx context.Context, kube client.Client, opts UndeployOptions) error {
	namespacedName := types.NamespacedName{
		Namespace: opts.Namespace,
		Name:      resourceNamer(opts.GVR.Resource, opts.GVR.Version),
	}

	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())

	sa, clusterrole, clusterrolebinding, role, rolebinding, err := createRBACResources(opts.GVR, getCDCrbacNN(namespacedName), opts.RBACFolderPath)
	if err != nil {
		return err
	}

	jsonSchemaConfigmap := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&jsonSchemaConfigmap, opts.GVR, getJsonSchemaConfigmapNN(namespacedName), opts.JsonSchemaTemplatePath,
		"schema", string(opts.JsonSchemaBytes),
	)
	if err != nil {
		return fmt.Errorf("error creating ConfigMap for JSON schema: %w", err)
	}
	err = kubecli.Uninstall(ctx, opts.KubeClient, &jsonSchemaConfigmap, kubecli.UninstallOptions{})
	if err != nil {
		log.Error(err, "Error uninstalling ConfigMap for JSON schema")
		return err
	}
	log.Debug("JSON Schema ConfigMap successfully uninstalled", "gvr", opts.GVR.String(), "name", jsonSchemaConfigmap.Name, "namespace", jsonSchemaConfigmap.Namespace)

	dep := appsv1.Deployment{}
	err = objects.CreateK8sObject(
		&dep,
		opts.GVR,
		getCDCDeploymentNN(namespacedName),
		opts.DeploymentTemplatePath,
		"serviceAccountName", sa.Name)
	if err != nil {
		return err
	}

	err = kubecli.Uninstall(ctx, opts.KubeClient, &dep, kubecli.UninstallOptions{})
	if err != nil {
		log.Error(err, "Error uninstalling deployment", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace)
		return err
	}
	log.Debug("Deployment successfully uninstalled", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace)

	cm := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&cm, opts.GVR, getCDCConfigmapNN(namespacedName), opts.ConfigmapTemplatePath,
		"composition_controller_sa_name", sa.Name,
		"composition_controller_sa_namespace", sa.Namespace)
	if err != nil {
		return err
	}

	err = kubecli.Uninstall(ctx, opts.KubeClient, &cm, kubecli.UninstallOptions{})
	if err != nil {
		log.Error(err, "uninstalling configmap")
		return err
	}
	log.Debug("Configmap successfully uninstalled", "gvr", opts.GVR.String(), "name", cm.Name, "namespace", cm.Namespace)

	err = uninstallRBACResources(ctx, opts.KubeClient, clusterrole, clusterrolebinding, role, rolebinding, sa)
	if err != nil {
		return err
	}

	_, err = os.Stat(opts.ServiceTemplatePath)
	if err == nil {
		svc := corev1.Service{}
		err = objects.CreateK8sObject(&svc, opts.GVR, getCDCDeploymentNN(namespacedName), opts.ServiceTemplatePath)
		if err != nil {
			log.Error(err, "creating service")
			return err
		}

		err = kubecli.Uninstall(ctx, opts.KubeClient, &svc, kubecli.UninstallOptions{})
		if err != nil {
			log.Error(err, "uninstalling service")
			return err
		}
		log.Debug("Service successfully uninstalled", "gvr", opts.GVR.String(), "name", svc.Name, "namespace", svc.Namespace)
	}

	if opts.Spec.Credentials != nil {
		role := rbacv1.Role{}
		err = objects.CreateK8sObject(&role, opts.GVR, getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: namespacedName.Name}), filepath.Join(opts.RBACFolderPath, "secret-role.yaml"), "secretName", opts.Spec.Credentials.PasswordRef.Name)
		if err != nil {
			log.Error(err, "creating role")
			return err
		}

		err = kubecli.Uninstall(ctx, opts.KubeClient, &role, kubecli.UninstallOptions{})
		if err != nil {
			log.Error(err, "uninstalling role")
			return err
		}
		log.Debug("Role successfully uninstalled", "gvr", opts.GVR.String(), "name", role.Name, "namespace", role.Namespace)

		rolebinding := rbacv1.RoleBinding{}
		err = objects.CreateK8sObject(&rolebinding, opts.GVR, getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: namespacedName.Name}), filepath.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"))
		if err != nil {
			log.Error(err, "creating rolebinding")
			return err
		}

		err = kubecli.Uninstall(ctx, opts.KubeClient, &rolebinding, kubecli.UninstallOptions{})
		if err != nil {
			log.Error(err, "uninstalling rolebinding")
			return err
		}
		log.Debug("RoleBinding successfully uninstalled", "gvr", opts.GVR.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace)
	}

	log.Debug("RBAC resources successfully uninstalled", "gvr", opts.GVR.String())

	if !opts.SkipCRD {
		err := crd.Uninstall(ctx, opts.KubeClient, opts.GVR.GroupResource())
		if err != nil {
			log.Debug("Error uninstalling CRD", "name", opts.GVR.GroupResource().String(), "error", err)
			return err
		}
		log.Debug("CRD successfully uninstalled", "name", opts.GVR.GroupResource().String())
	}

	return nil
}

// This function is used to lookup the current state of the deployment and return the hash of the current state
// This is used to determine if the deployment needs to be updated or not
func Lookup(ctx context.Context, kube client.Client, opts DeployOptions) (digest string, err error) {
	namespacedName := types.NamespacedName{
		Namespace: opts.Namespace,
		Name:      resourceNamer(opts.GVR.Resource, opts.GVR.Version),
	}

	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())

	sa, clusterrole, clusterrolebinding, role, rolebinding, err := createRBACResources(opts.GVR, getCDCrbacNN(namespacedName), opts.RBACFolderPath)
	if err != nil {
		return "", err
	}

	hsh := hasher.NewFNVObjectHash()
	if opts.Spec.Credentials != nil {
		role := rbacv1.Role{}
		err = objects.CreateK8sObject(&role, opts.GVR, getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: namespacedName.Name}), filepath.Join(opts.RBACFolderPath, "secret-role.yaml"), "secretName", opts.Spec.Credentials.PasswordRef.Name)
		if err != nil {
			log.Error(err, "creating role")
			return "", err
		}

		err = kubecli.Get(ctx, kube, &role)
		if err != nil {
			log.Error(err, "fetching role")
			return "", err
		}
		err = hsh.SumHash(role.ObjectMeta.Name, role.ObjectMeta.Namespace, role.Rules)
		if err != nil {
			return "", fmt.Errorf("error hashing role: %v", err)
		}
		log.Debug("Role successfully fetched", "gvr", opts.GVR.String(), "name", role.Name, "namespace", role.Namespace, "digest", hsh.GetHash())

		rolebinding := rbacv1.RoleBinding{}
		err = objects.CreateK8sObject(&rolebinding, opts.GVR, getCDCrbacNN(types.NamespacedName{Namespace: opts.Spec.Credentials.PasswordRef.Namespace, Name: namespacedName.Name}), filepath.Join(opts.RBACFolderPath, "secret-rolebinding.yaml"), "serviceAccount", sa.Name, "saNamespace", sa.Namespace)
		if err != nil {
			log.Error(err, "creating rolebinding")
			return "", err
		}

		err = kubecli.Get(ctx, kube, &rolebinding)
		if err != nil {
			log.Error(err, "fetching rolebinding")
			return "", err
		}
		err = hsh.SumHash(rolebinding.ObjectMeta.Name, rolebinding.ObjectMeta.Namespace, rolebinding.Subjects, rolebinding.RoleRef)
		if err != nil {
			return "", fmt.Errorf("error hashing rolebinding: %v", err)
		}
		log.Debug("RoleBinding successfully fetched", "gvr", opts.GVR.String(), "name", rolebinding.Name, "namespace", rolebinding.Namespace, "digest", hsh.GetHash())
	}

	err = lookupRBACResources(ctx, opts.KubeClient, clusterrole, clusterrolebinding, role, rolebinding, sa, &hsh)
	if err != nil {
		return "", err
	}

	jsonSchemaConfigmap := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&jsonSchemaConfigmap, opts.GVR, getJsonSchemaConfigmapNN(namespacedName), opts.JsonSchemaTemplatePath,
		"schema", string(opts.JsonSchemaBytes),
	)
	if err != nil {
		return "", fmt.Errorf("error creating ConfigMap for JSON schema: %w", err)
	}
	err = kubecli.Get(ctx, opts.KubeClient, &jsonSchemaConfigmap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("JSON Schema ConfigMap not found", "gvr", opts.GVR.String(), "name", jsonSchemaConfigmap.Name, "namespace", jsonSchemaConfigmap.Namespace)
			jsonSchemaConfigmap = corev1.ConfigMap{}
		} else {
			log.Error(err, "fetching ConfigMap for JSON schema")
			return "", fmt.Errorf("error fetching ConfigMap for JSON schema: %w", err)
		}
	}
	err = hsh.SumHash(jsonSchemaConfigmap.ObjectMeta.Name, jsonSchemaConfigmap.ObjectMeta.Namespace)
	if err != nil {
		return "", fmt.Errorf("error hashing JSON schema configmap: %v", err)
	}
	log.Debug("JSON Schema ConfigMap successfully fetched", "gvr", opts.GVR.String(), "name", jsonSchemaConfigmap.Name, "namespace", jsonSchemaConfigmap.Namespace, "digest", hsh.GetHash())

	cm := corev1.ConfigMap{}
	err = objects.CreateK8sObject(&cm, opts.GVR, getCDCConfigmapNN(namespacedName), opts.ConfigmapTemplatePath,
		"composition_controller_sa_name", sa.Name,
		"composition_controller_sa_namespace", sa.Namespace)
	if err != nil {
		return "", err
	}
	err = kubecli.Get(ctx, opts.KubeClient, &cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Configmap not found", "gvr", opts.GVR.String(), "name", cm.Name, "namespace", cm.Namespace)
			cm = corev1.ConfigMap{}
		} else {
			log.Error(err, "fetching configmap")
			return "", fmt.Errorf("error fetching configmap: %w", err)
		}
	}
	err = hsh.SumHash(cm.ObjectMeta.Name, cm.ObjectMeta.Namespace, cm.Data)
	if err != nil {
		return "", fmt.Errorf("error hashing configmap: %v", err)
	}
	log.Debug("Configmap successfully fetched", "gvr", opts.GVR.String(), "name", cm.Name, "namespace", cm.Namespace, "digest", hsh.GetHash())

	dep := appsv1.Deployment{}
	err = objects.CreateK8sObject(
		&dep,
		opts.GVR,
		getCDCDeploymentNN(namespacedName),
		opts.DeploymentTemplatePath,
		"serviceAccountName", sa.Name)
	if err != nil {
		return "", err
	}

	err = kubecli.Get(ctx, opts.KubeClient, &dep)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Deployment not found", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace)
			dep = appsv1.Deployment{}
		} else {
			log.Error(err, "fetching deployment")
			return "", fmt.Errorf("error fetching deployment: %w", err)
		}
	}

	deployment.CleanFromRestartAnnotation(&dep)

	err = hsh.SumHash(dep.ObjectMeta.Name, dep.ObjectMeta.Namespace, dep.Spec)
	if err != nil {
		return "", fmt.Errorf("error hashing deployment spec: %v", err)
	}
	log.Debug("Deployment successfully fetched", "gvr", opts.GVR.String(), "name", dep.Name, "namespace", dep.Namespace, "digest", hsh.GetHash())

	_, err = os.Stat(opts.ServiceTemplatePath)
	if err == nil {
		svc := corev1.Service{}
		err = objects.CreateK8sObject(&svc, opts.GVR, getCDCDeploymentNN(namespacedName), opts.ServiceTemplatePath)
		if err != nil {
			log.Error(err, "creating service")
			return "", err
		}
		err = kubecli.Get(ctx, opts.KubeClient, &svc)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Debug("Service not found", "gvr", opts.GVR.String(), "name", svc.Name, "namespace", svc.Namespace)
				svc = corev1.Service{}
			} else {
				log.Error(err, "fetching service")
				return "", fmt.Errorf("error fetching service: %w", err)
			}
		}
		err = hsh.SumHash(svc.ObjectMeta.Name, svc.ObjectMeta.Namespace, svc.Spec)
		if err != nil {
			return "", fmt.Errorf("error hashing service: %v", err)
		}
		log.Debug("Service successfully fetched", "gvr", opts.GVR.String(), "name", svc.Name, "namespace", svc.Namespace, "digest", hsh.GetHash())
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
