//go:build integration
// +build integration

package tools

import (
	"context"
	"testing"

	"github.com/krateoplatformops/core-provider/apis/definitions/v1alpha1"
)

func Test_Deploy(t *testing.T) {
	nfo := &v1alpha1.ChartInfo{
		Url:     "oci://registry-1.docker.io/bitnamicharts/postgresql",
		Version: "12.8.3",
	}

	kube := setupKubeClient(t)
	err := Deploy(context.TODO(), kube, DeployOptions{
		Spec:      nfo,
		Namespace: "krateo-system",
	})
	if err != nil {
		t.Fatal(err)
	}
}
