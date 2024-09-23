package rbactools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUninstallClusterRoleBinding(t *testing.T) {
	ctx := context.TODO()

	// Create a fake client
	fakeClient := fake.NewClientBuilder().Build()

	// Create a ClusterRoleBinding object
	clusterRoleBinding := CreateClusterRoleBinding(types.NamespacedName{
		Name: "test-clusterrolebinding",
	})

	// Install the ClusterRoleBinding
	err := InstallClusterRoleBinding(ctx, fakeClient, &clusterRoleBinding)
	require.NoError(t, err)

	// Uninstall the ClusterRoleBinding
	err = UninstallClusterRoleBinding(ctx, UninstallOptions{
		KubeClient: fakeClient,
		NamespacedName: types.NamespacedName{
			Name: "test-clusterrolebinding",
		},
		Log: nil,
	})
	require.NoError(t, err)

	// Verify that the ClusterRoleBinding is uninstalled
	crb := &rbacv1.ClusterRoleBinding{}
	err = fakeClient.Get(ctx, client.ObjectKeyFromObject(&clusterRoleBinding), crb)
	assert.True(t, apierrors.IsNotFound(err))
}
func TestPopulateClusterRoleBinding(t *testing.T) {
	tests := []struct {
		name     string
		tmp      *rbacv1.ClusterRoleBinding
		obj      *rbacv1.ClusterRoleBinding
		expected []rbacv1.Subject
	}{
		{
			name: "No subjects in tmp",
			tmp:  &rbacv1.ClusterRoleBinding{},
			obj: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{
					{Kind: "User", Name: "user1", Namespace: "default"},
				},
			},
			expected: []rbacv1.Subject{
				{Kind: "User", Name: "user1", Namespace: "default"},
			},
		},
		{
			name: "No new subjects in obj",
			tmp: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{
					{Kind: "User", Name: "user1", Namespace: "default"},
				},
			},
			obj: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{
					{Kind: "User", Name: "user1", Namespace: "default"},
				},
			},
			expected: []rbacv1.Subject{
				{Kind: "User", Name: "user1", Namespace: "default"},
			},
		},
		{
			name: "New subjects in obj",
			tmp: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{
					{Kind: "User", Name: "user1", Namespace: "default"},
				},
			},
			obj: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{
					{Kind: "User", Name: "user2", Namespace: "default"},
				},
			},
			expected: []rbacv1.Subject{
				{Kind: "User", Name: "user1", Namespace: "default"},
				{Kind: "User", Name: "user2", Namespace: "default"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			populateClusterRoleBinding(tt.tmp, tt.obj)
			assert.Equal(t, tt.expected, tt.tmp.Subjects)
		})
	}
}
