//go:build integration
// +build integration

package tools_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/krateoplatformops/core-provider/internal/tools"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	testCRD = "../../../testdata/dummycrd.yaml"
)

func Test_toGroupVersionResource(t *testing.T) {
	gvk := schema.FromAPIVersionAndKind("composition.krateo.io/v0-2-0", "DummyChart")

	expGVR := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-2-0",
		Resource: "dummycharts",
	}
	gotGVR := tools.ToGroupVersionResource(gvk)
	if !cmp.Equal(expGVR, gotGVR) {
		t.Fatalf("invalid GVR - expected: %s, got: %s", expGVR.String(), gotGVR.String())
	}
}

func Test_lookupCRD(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	gvk := schema.GroupVersionKind{
		Group:   "composition.krateo.io",
		Version: "v12-8-3",
		Kind:    "Postgresql",
	}

	ok, err := tools.LookupCRD(context.Background(), kube, tools.ToGroupVersionResource(gvk))
	if err != nil {
		t.Fatal(err)
	}

	if ok {
		t.Logf("crd: %v, exists", gvk)
	} else {
		t.Logf("crd: %v, does not exists", gvk)
	}
}
