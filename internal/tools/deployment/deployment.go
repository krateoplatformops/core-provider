package deployment

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/krateoplatformops/core-provider/internal/templates"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
)

func CreateDeployment(gvr schema.GroupVersionResource, nn types.NamespacedName, templatePath string, additionalvalues ...string) (appsv1.Deployment, error) {
	values := templates.Values(templates.Renderoptions{
		Group:     gvr.Group,
		Version:   gvr.Version,
		Resource:  gvr.Resource,
		Namespace: nn.Namespace,
		Name:      nn.Name,
	})

	if len(additionalvalues)%2 != 0 {
		return appsv1.Deployment{}, fmt.Errorf("additionalvalues must be in pairs")
	}
	for i := 0; i < len(additionalvalues); i += 2 {
		values[additionalvalues[i]] = additionalvalues[i+1]
	}

	templateF, err := os.ReadFile(templatePath)
	if err != nil {
		return appsv1.Deployment{}, fmt.Errorf("failed to read template file: %w", err)
	}

	template := templates.Template(string(templateF))
	dat, err := template.RenderDeployment(values)
	if err != nil {
		return appsv1.Deployment{}, err
	}

	if !clientsetscheme.Scheme.IsGroupRegistered("apps") {
		_ = appsv1.AddToScheme(clientsetscheme.Scheme)
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := appsv1.Deployment{}
	_, _, err = s.Decode(dat, nil, &res)
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

func RestartDeployment(ctx context.Context, kube client.Client, obj *appsv1.Deployment) error {
	patch := client.MergeFrom(obj.DeepCopy())

	// Set the annotation to trigger a rollout
	if obj.Spec.Template.Annotations == nil {
		obj.Spec.Template.Annotations = map[string]string{}
	}
	obj.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	// patch the deployment
	return kube.Patch(ctx, obj, patch)
}

func CleanFromRestartAnnotation(obj *appsv1.Deployment) {
	if obj.Spec.Template.Annotations != nil {
		delete(obj.Spec.Template.Annotations, "kubectl.kubernetes.io/restartedAt")
	}
}
