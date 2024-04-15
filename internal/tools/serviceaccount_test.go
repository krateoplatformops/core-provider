package tools_test

import (
	"context"
	"testing"

	"github.com/krateoplatformops/core-provider/internal/tools"
	"k8s.io/apimachinery/pkg/types"
)

func TestInstallServiceAccount(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	obj := tools.CreateServiceAccount(types.NamespacedName{
		Name:      "demo",
		Namespace: "default",
	})

	err = tools.InstallServiceAccount(context.TODO(), kube, &obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUninstallServiceAccount(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	err = tools.UninstallServiceAccount(context.TODO(), tools.UninstallOptions{
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
