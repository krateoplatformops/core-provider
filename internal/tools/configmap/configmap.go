package configmap

import (
	"context"
	"fmt"
	"os"

	"github.com/avast/retry-go"
	"github.com/krateoplatformops/core-provider/internal/templates"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UninstallOptions struct {
	KubeClient     client.Client
	NamespacedName types.NamespacedName
	Log            func(msg string, keysAndValues ...any)
}

func UninstallConfigmap(ctx context.Context, opts UninstallOptions) error {
	return retry.Do(
		func() error {
			cm := corev1.ConfigMap{}
			err := opts.KubeClient.Get(ctx, opts.NamespacedName, &cm, &client.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}

				return err
			}

			err = opts.KubeClient.Delete(ctx, &cm, &client.DeleteOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}

				return err
			}

			if opts.Log != nil {
				opts.Log("Configmap successfully uninstalled",
					"name", cm.GetName(), "namespace", cm.GetNamespace())
			}

			return nil
		},
	)
}

func InstallConfigmap(ctx context.Context, kube client.Client, obj *corev1.ConfigMap) error {
	return retry.Do(
		func() error {
			tmp := corev1.ConfigMap{}
			err := kube.Get(ctx, client.ObjectKeyFromObject(obj), &tmp)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return kube.Create(ctx, obj)
				}

				return err
			}

			return nil
		},
	)
}

func CreateConfigmap(gvr schema.GroupVersionResource, nn types.NamespacedName, configmapTemplatePath string) (corev1.ConfigMap, error) {
	values := templates.Values(templates.Renderoptions{
		Group:     gvr.Group,
		Version:   gvr.Version,
		Resource:  gvr.Resource,
		Namespace: nn.Namespace,
		Name:      nn.Name,
	})

	templateF, err := os.ReadFile(configmapTemplatePath)
	if err != nil {
		return corev1.ConfigMap{}, fmt.Errorf("failed to read configmap template file: %w", err)
	}

	template := templates.Template(string(templateF))
	dat, err := template.RenderDeployment(values)
	if err != nil {
		return corev1.ConfigMap{}, err
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := corev1.ConfigMap{}
	_, _, err = s.Decode(dat, nil, &res)
	if err != nil {
		return corev1.ConfigMap{}, fmt.Errorf("failed to decode configmap: %w", err)
	}

	return res, err
}

func LookupDeployment(ctx context.Context, kube client.Client, obj *appsv1.Deployment) (bool, bool, error) {
	err := kube.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}

		return false, false, err
	}

	ready := obj.Spec.Replicas != nil && *obj.Spec.Replicas == obj.Status.ReadyReplicas

	return true, ready, nil
}
