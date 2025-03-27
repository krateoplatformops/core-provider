package rbactools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestCreateRole(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "roles",
	}
	nn := types.NamespacedName{
		Name:      "test-role",
		Namespace: "test-namespace",
	}
	path := "testdata/role_template.yaml"

	clusterRole, err := CreateRole(gvr, nn, path, "secretName", "test-value")
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
		{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			Verbs:         []string{"get", "list", "watch"},
			ResourceNames: []string{"test-value"},
		},
	}

	assert.Equal(t, "rbac.authorization.k8s.io/v1", clusterRole.APIVersion)
	assert.Equal(t, "Role", clusterRole.Kind)
	assert.Equal(t, gvr.Resource+"-"+gvr.Version, clusterRole.Name)
	assert.Equal(t, expectedRules, clusterRole.Rules)
}
