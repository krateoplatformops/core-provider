//go:build integration
// +build integration

package tools

import (
	"context"
	"testing"

	"github.com/krateoplatformops/core-provider/apis/definitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
)

func Test_Deploy(t *testing.T) {
	pkg, err := chartfs.ForSpec(&v1alpha1.ChartInfo{
		Url:     "oci://registry-1.docker.io/bitnamicharts",
		Version: "12.8.3",
		Name:    "postgresql",
	})
	if err != nil {
		t.Fatal(err)
	}

	kube := setupKubeClient(t)
	err = Deploy(context.TODO(), kube, DeployOptions{
		ChartFS:   pkg,
		Namespace: "krateo-system",
	})
	if err != nil {
		t.Fatal(err)
	}
}
