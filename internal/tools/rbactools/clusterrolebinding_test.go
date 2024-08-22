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
