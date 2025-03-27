package rbactools

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestCreateClusterRole(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterroles",
	}
	nn := types.NamespacedName{
		Name:      "test-clusterrole",
		Namespace: "test-namespace",
	}
	path := "testdata/clusterrole_template.yaml"

	clusterRole, err := CreateClusterRole(gvr, nn, path)
	assert.NoError(t, err)

	expectedRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apiextensions.k8s.io"},
			Resources: []string{"customresourcedefinitions"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create", "patch", "update"},
		},
	}

	assert.Equal(t, "rbac.authorization.k8s.io/v1", clusterRole.APIVersion)
	assert.Equal(t, "ClusterRole", clusterRole.Kind)
	assert.Equal(t, gvr.Resource+"-"+gvr.Version, clusterRole.Name)
	assert.Equal(t, expectedRules, clusterRole.Rules)
}

func TestCreateClusterRole_FileNotFound(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterroles",
	}
	nn := types.NamespacedName{
		Name:      "test-clusterrole",
		Namespace: "test-namespace",
	}
	path := "nonexistent_template.yaml"

	_, err := CreateClusterRole(gvr, nn, path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read clusterrole template file")
}

func TestCreateClusterRole_InvalidTemplate(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterroles",
	}
	nn := types.NamespacedName{
		Name:      "test-clusterrole",
		Namespace: "test-namespace",
	}
	path := "testdata/invalid_template.yaml"

	// Create a temporary file with an invalid ClusterRole template
	templateContent := `
invalid yaml content
`
	err := os.WriteFile(path, []byte(templateContent), 0644)
	assert.NoError(t, err)
	defer os.Remove(path)

	_, err = CreateClusterRole(gvr, nn, path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode clusterrole")
}
