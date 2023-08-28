package tools

import (
	"context"
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLookupDeployment(t *testing.T) {
	cli := setupKubeClient(t)

	obj := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgresqls-v12-8-3-controller",
			Namespace: "krateo-system",
		},
	}

	exists, ready, err := LookupDeployment(context.TODO(), cli, &obj)
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
