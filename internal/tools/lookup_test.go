//go:build integration
// +build integration

package tools

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
	testCRD = "../../../testdata/dummycrd.yaml"
)

func Test_toGroupVersionResource(t *testing.T) {
	gvk := schema.FromAPIVersionAndKind("composition.krateo.io/v0-2-0", "DummyChart")

	expGVR := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-2-0",
		Resource: "dummycharts",
	}
	gotGVR := ToGroupVersionResource(gvk)
	if !cmp.Equal(expGVR, gotGVR) {
		t.Fatalf("invalid GVR - expected: %s, got: %s", expGVR.String(), gotGVR.String())
	}
}

func Test_lookupCRD(t *testing.T) {
	cli := setupKubeClient(t)

	gvk := schema.GroupVersionKind{
		Group:   "composition.krateo.io",
		Version: "v12-8-3",
		Kind:    "Postgresql",
	}

	ok, err := LookupCRD(context.Background(), cli, ToGroupVersionResource(gvk))
	if err != nil {
		t.Fatal(err)
	}

	if ok {
		t.Logf("crd: %v, exists", gvk)
	} else {
		t.Logf("crd: %v, does not exists", gvk)
	}
}

func setupKubeClient(t *testing.T) client.Client {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", path.Join(home, ".kube/config"))
	if err != nil {
		t.Fatal(err)
	}

	cli, err := client.New(cfg, client.Options{})
	if err != nil {
		t.Fatal(err)
	}

	return cli
}
