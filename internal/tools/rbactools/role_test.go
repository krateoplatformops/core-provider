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

func TestInitRole(t *testing.T) {
	resource := "example"
	opts := types.NamespacedName{
		Name:      "test-role",
		Namespace: "test-namespace",
	}

	expectedRole := rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "role",
				"app.kubernetes.io/instance":   "manager-role",
				"app.kubernetes.io/component":  "rbac",
				"app.kubernetes.io/created-by": "example",
				"app.kubernetes.io/part-of":    "example",
				"app.kubernetes.io/managed-by": "kustomize",
			},
		},
		Rules: []rbacv1.PolicyRule{},
	}

	role := InitRole(resource, opts)

	if assert.ObjectsAreEqual(expectedRole, role) == false {
		t.Errorf("InitRole() returned unexpected result.\nExpected: %+v\nGot: %+v", expectedRole, role)
	}
}
func TestUninstallRole(t *testing.T) {
	opts := UninstallOptions{
		KubeClient:     fake.NewClientBuilder().Build(),
		NamespacedName: types.NamespacedName{Name: "test-role", Namespace: "test-namespace"},
		Log:            nil,
	}

	role := InitRole("example", opts.NamespacedName)

	err := InstallRole(context.Background(), opts.KubeClient, &role)
	assert.NoError(t, err, "InstallRole() returned an unexpected error")

	err = UninstallRole(context.Background(), opts)
	assert.NoError(t, err, "UninstallRole() returned an unexpected error")
}