package tools

import (
	"context"
	"fmt"
	"testing"

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

	obj, err := CreateDeployment(schema.GroupVersionResource{
		Group:    "core.krateo.io",
		Version:  "v1-0-0",
		Resource: "dummies",
	}, types.NamespacedName{Name: "demo", Namespace: "default"})
	if err != nil {
		t.Fatal(err)
	}

	err = InstallDeployment(context.TODO(), kube, &obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUninstallDeployment(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	err = UninstallDeployment(context.TODO(), kube, types.NamespacedName{
		Name:      "demo",
		Namespace: "default",
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

	exists, ready, err := LookupDeployment(context.TODO(), kube, &obj)
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
