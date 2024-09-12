package rbacgen

// FakeDiscovery implements discovery.DiscoveryInterface and sometimes calls testing.Fake.Invoke with an action, but doesn't respect the return value if any.
// Need to implement the methods of the interface to make it work.

import (
	"context"
	"testing"
	"time"

	definitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	fakediscovery "github.com/krateoplatformops/core-provider/internal/tools/fake/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// clientgofake "k8s.io/client-go/kubernetes/fake"
	cligotesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPopulateRBAC(t *testing.T) {
	fakecli := fake.NewClientBuilder().Build()

	discovery := fakediscovery.NewCachedDiscoveryClient(&fakediscovery.FakeDiscovery{
		Fake: &cligotesting.Fake{
			Resources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Group:   "",
							Version: "v1",
							Kind:    "Secret",
						},
						{
							Group:   "",
							Version: "v1",
							Kind:    "ConfigMap",
						},
					},
				},
			},
		},
	}, "test", 1*time.Second)

	pkg, err := chartfs.ForSpec(context.Background(), fakecli, &definitionsv1alpha1.ChartInfo{
		Url:     "https://raw.githubusercontent.com/matteogastaldello/private-charts/main",
		Version: "0.1.0",
		Repo:    "test-chart",
	})
	require.NoError(t, err)
	deployName := "test-deploy"
	deployNamespace := "test-namespace"
	secretName := "test-secret"
	secretNamespace := "test-secret-namespace"

	rbacGenerator := NewRbacGenerator(discovery, pkg, deployName, deployNamespace, secretName, secretNamespace)

	resourceName := "test-resource"
	rbacMap, rbacErr := rbacGenerator.PopulateRBAC(resourceName)

	assert.NoError(t, rbacErr)
	assert.NotNil(t, rbacMap)
	assert.NotEmpty(t, rbacMap)

	// Assert specific RBAC objects
	deployNamespaceRBAC, ok := rbacMap[deployNamespace]
	assert.True(t, ok)
	assert.NotNil(t, deployNamespaceRBAC.Role)
	assert.NotNil(t, deployNamespaceRBAC.RoleBinding)
	assert.NotNil(t, deployNamespaceRBAC.ServiceAccount)

	secretNamespaceRBAC, ok := rbacMap[secretNamespace]
	assert.True(t, ok)
	assert.NotNil(t, secretNamespaceRBAC.Role)
	assert.NotNil(t, secretNamespaceRBAC.RoleBinding)

	// Assert composition rules in deploy namespace
	compositionRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"core.krateo.io"},
			Resources: []string{"compositiondefinitions", "compositiondefinitions/status"},
			Verbs:     []string{"*"},
		},
		{
			APIGroups: []string{"composition.krateo.io"},
			Resources: []string{resourceName, resourceName + "/status"},
			Verbs:     []string{"*"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"*"},
		},
	}

	assert.ElementsMatch(t, compositionRules, deployNamespaceRBAC.Role.Rules)
}
