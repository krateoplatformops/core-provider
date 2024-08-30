//go:build integration
// +build integration

package crd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
func TestAppendVersion(t *testing.T) {
	crd := &apiextensionsv1.CustomResourceDefinition{
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "v1alpha1",
				},
			},
		},
	}

	toAdd := &apiextensionsv1.CustomResourceDefinition{
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "v1alpha2",
				},
				{
					Name: "v1alpha3",
				},
			},
		},
	}

	expected := &apiextensionsv1.CustomResourceDefinition{
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  false,
					Storage: false,
				},
				{
					Name:    "v1alpha2",
					Served:  false,
					Storage: false,
				},
				{
					Name:    "v1alpha3",
					Served:  true,
					Storage: true,
				},
			},
		},
	}

	expected2 := &apiextensionsv1.CustomResourceDefinition{
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  false,
					Storage: false,
				},
				{
					Name:    "v1alpha2",
					Served:  true,
					Storage: true,
				},
			},
		},
	}

	result, err := AppendVersion(*crd, *toAdd)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
	if diff := cmp.Diff(result, expected); len(diff) > 0 {
		t.Fatalf("Unexpected result (-got +want):\n%s", diff)
	}

	crd = &apiextensionsv1.CustomResourceDefinition{
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "v1alpha1",
				},
			},
		},
	}

	result, err = AppendVersion(*crd, *expected2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if diff := cmp.Diff(result, expected2); len(diff) > 0 {
		t.Fatalf("Unexpected result (-got +want):\n%s", diff)
	}
}

func TestGet(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    "core.krateo.io",
		Version:  "v1alpha1",
		Resource: "FormDefinition",
	}

	crd, err := Get(context.Background(), kube, gvr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if crd != nil {
		t.Logf("CRD exists: %v", crd.Name)
	} else {
		t.Logf("CRD does not exist")
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
