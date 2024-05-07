//go:build integration
// +build integration

package tools_test

import (
	"context"
	"testing"

	"github.com/krateoplatformops/core-provider/internal/tools"
	"k8s.io/apimachinery/pkg/types"
)

func TestInstallClusterRoleBinding(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	obj := tools.CreateClusterRoleBinding(types.NamespacedName{
		Name:      "demo",
		Namespace: "default",
	})

	err = tools.InstallClusterRoleBinding(context.TODO(), kube, &obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUninstallClusterRoleBinding(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	err = tools.UninstallClusterRoleBinding(context.TODO(), tools.UninstallOptions{
		KubeClient: kube,
		NamespacedName: types.NamespacedName{
			Name:      "demo",
			Namespace: "default",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
