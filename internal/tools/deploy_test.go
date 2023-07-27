//go:build integration
// +build integration

package tools

import (
	"context"
	"testing"

	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
)

func Test_Deploy(t *testing.T) {
	pkg, err := chartfs.FromURL(testChartUrl)
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
