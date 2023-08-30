//go:build integration
// +build integration

package tools

import (
	"context"
	"testing"

	"github.com/krateoplatformops/core-provider/apis/definitions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	err = Deploy(context.TODO(), kube, DeployOptions{
		Spec:      nfo,
		Namespace: "default",
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

	err = Undeploy(context.TODO(), kube, gvr, "default")
	if err != nil {
		t.Fatal(err)
	}
}
