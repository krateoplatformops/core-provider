package configmap

import (
	"fmt"
	"os"

	"github.com/krateoplatformops/core-provider/internal/templates"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
)

func CreateConfigmap(gvr schema.GroupVersionResource, nn types.NamespacedName, configmapTemplatePath string, additionalvalues ...string) (corev1.ConfigMap, error) {
	values := templates.Values(templates.Renderoptions{
		Group:     gvr.Group,
		Version:   gvr.Version,
		Resource:  gvr.Resource,
		Namespace: nn.Namespace,
		Name:      nn.Name,
	})

	if len(additionalvalues)%2 != 0 {
		return corev1.ConfigMap{}, fmt.Errorf("additionalvalues must be in pairs")
	}
	for i := 0; i < len(additionalvalues); i += 2 {
		values[additionalvalues[i]] = additionalvalues[i+1]
	}

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
