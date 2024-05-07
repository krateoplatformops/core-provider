//go:build integration
// +build integration

package tools_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/krateoplatformops/core-provider/internal/tools"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestInstallDeployment(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	obj, err := tools.CreateDeployment(
		schema.GroupVersionResource{
			Group:    "composition.krateo.io",
			Version:  "v12-8-3",
			Resource: "postgresqls",
		},
		types.NamespacedName{Name: "demo", Namespace: "default"},
		os.Getenv("CDC_IMAGE_TAG"),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = tools.InstallDeployment(context.TODO(), kube, &obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUninstallDeployment(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	err = tools.UninstallDeployment(context.TODO(), tools.UninstallOptions{
		KubeClient: kube,
		NamespacedName: types.NamespacedName{
			Name:      "postgresqls-v12-8-3-controller",
			Namespace: "krateo-system",
		},
		Log: func(msg string, keysAndValues ...any) {
			fmt.Print(msg)
			for _, v := range keysAndValues {
				fmt.Printf("%s ", v)
			}
			fmt.Println()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLookupDeployment(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	obj := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgresqls-v12-8-3-controller",
			Namespace: "krateo-system",
		},
	}

	exists, ready, err := tools.LookupDeployment(context.TODO(), kube, &obj)
	if err != nil {
		t.Fatal(err)
	}

	if exists {
		fmt.Println("Deployment", obj.Name, " exists")
	}

	if ready {
		fmt.Println("Deployment", obj.Name, " is ready")
	}

}
