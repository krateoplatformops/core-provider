//go:build integration
// +build integration

package tools_test

import (
	"context"
	"os"
	"testing"

	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestDeploy(t *testing.T) {
	nfo := &v1alpha1.ChartInfo{
		Url:     "oci://registry-1.docker.io/bitnamicharts/postgresql",
		Version: "12.8.3",
	}

	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	err = tools.Deploy(context.TODO(), tools.DeployOptions{
		KubeClient: kube,
		Spec:       nfo,
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "postgresql-repo",
		},
		CDCImageTag: os.Getenv("CDC_IMAGE_TAG"),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUndeploy(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v12-8-3",
		Resource: "postgresqls",
	}

	err = tools.Undeploy(context.TODO(), tools.UndeployOptions{
		KubeClient: kube,
		GVR:        gvr,
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "postgresql-repo",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
