//go:build integration
// +build integration

package crdutil

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/tools/clientcmd"
)

const (
	testCRD = "../../crds/core.krateo.io_formdefinitions.yaml"
)

func TestLookup(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	gvk := schema.GroupVersionKind{
		Group:   "core.krateo.io",
		Version: "v1alpha1",
		Kind:    "FormDefinition",
	}

	gvr := InferGroupResource(gvk.GroupKind()).WithVersion(gvk.Version)

	ok, err := Lookup(context.Background(), kube, gvr)
	if err != nil {
		t.Fatal(err)
	}

	if ok {
		t.Logf("crd: %v, exists", gvk)
	} else {
		t.Logf("crd: %v, does not exists", gvk)
	}
}

func TestInferGroupResource(t *testing.T) {
	table := []struct {
		gk   schema.GroupKind
		want schema.GroupResource
	}{
		{
			gk:   schema.GroupKind{Group: "core.krateo.io", Kind: "CardTemplate"},
			want: schema.GroupResource{Group: "core.krateo.io", Resource: "cardtemplates"},
		},
	}

	for i, tc := range table {
		got := InferGroupResource(tc.gk)
		if diff := cmp.Diff(got, tc.want); len(diff) > 0 {
			t.Fatalf("[tc: %d] diff: %s", i, diff)
		}
	}
}

func setupKubeClient() (client.Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", path.Join(home, ".kube/config"))
	if err != nil {
		return nil, err
	}

	return client.New(cfg, client.Options{})
}
