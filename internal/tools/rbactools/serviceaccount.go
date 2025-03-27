package rbactools

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

func CreateServiceAccount(gvr schema.GroupVersionResource, nn types.NamespacedName, path string, additionalvalues ...string) (corev1.ServiceAccount, error) {
	templateF, err := os.ReadFile(path)
	if err != nil {
		return corev1.ServiceAccount{}, fmt.Errorf("failed to read ServiceAccount template file: %w", err)
	}

	values := templates.Values(templates.Renderoptions{
		Group:     gvr.Group,
		Version:   gvr.Version,
		Resource:  gvr.Resource,
		Namespace: nn.Namespace,
		Name:      nn.Name,
	})

	if len(additionalvalues)%2 != 0 {
		return corev1.ServiceAccount{}, fmt.Errorf("additionalvalues must be in pairs: %w", err)
	}
	for i := 0; i < len(additionalvalues); i += 2 {
		values[additionalvalues[i]] = additionalvalues[i+1]
	}

	template := templates.Template(string(templateF))
	dat, err := template.RenderDeployment(values)
	if err != nil {
		return corev1.ServiceAccount{}, err
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := corev1.ServiceAccount{}
	_, _, err = s.Decode(dat, nil, &res)
	if err != nil {
		return corev1.ServiceAccount{}, fmt.Errorf("failed to decode ServiceAccount binding: %w", err)
	}

	return res, err
}
