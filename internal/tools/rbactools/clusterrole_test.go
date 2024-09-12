package rbactools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInitClusterRole(t *testing.T) {
	opts := types.NamespacedName{
		Name:      "test-clusterrole",
		Namespace: "test-namespace",
	}

	clusterRole := InitClusterRole(opts)

	expectedRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apiextensions.k8s.io"},
			Resources: []string{"customresourcedefinitions"},
			Verbs:     []string{"get", "list"},
		},
	}
	assert.Equal(t, "rbac.authorization.k8s.io/v1", clusterRole.APIVersion)
	assert.Equal(t, "ClusterRole", clusterRole.Kind)
	assert.Equal(t, opts.Name, clusterRole.Name)
	assert.Equal(t, clusterRole.Rules, expectedRules)
}
func TestUninstallClusterRole(t *testing.T) {
	opts := UninstallOptions{
		NamespacedName: types.NamespacedName{
			Name:      "test-clusterrole",
			Namespace: "test-namespace",
		},
		KubeClient: fake.NewClientBuilder().Build(),
		Log:        nil,
	}

	err := UninstallClusterRole(context.TODO(), opts)

	assert.NoError(t, err)
}
func TestInstallClusterRole(t *testing.T) {
	ctx := context.TODO()
	kube := fake.NewClientBuilder().Build()
	obj := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-clusterrole",
		},
		Rules: []rbacv1.PolicyRule{},
	}

	err := InstallClusterRole(ctx, kube, obj)

	assert.NoError(t, err)
}
