//go:build integration
// +build integration

package tools

import (
	"context"
	"testing"
)

func Test_Deploy(t *testing.T) {
	kube := setupKubeClient(t)
	err := Deploy(context.TODO(), kube, DeployOptions{
		Group:     "core.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "krateo-system",
	})
	if err != nil {
		t.Fatal(err)
	}
}
