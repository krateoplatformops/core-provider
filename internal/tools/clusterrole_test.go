package tools_test

import (
	"context"
	"testing"

	"github.com/krateoplatformops/core-provider/internal/tools"
	"k8s.io/apimachinery/pkg/types"
)

func TestInstallClusterRole(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	obj := tools.CreateClusterRole(types.NamespacedName{
		Name:      "demo",
		Namespace: "default",
	})

	err = tools.InstallClusterRole(context.TODO(), kube, &obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUninstallClusterRole(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	err = tools.UninstallClusterRole(context.TODO(), tools.UninstallOptions{
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
