//go:build integration
// +build integration

package definitions

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
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
	gotGVR := toGroupVersionResource(gvk)
	if !cmp.Equal(expGVR, gotGVR) {
		t.Fatalf("invalid GVR - expected: %s, got: %s", expGVR.String(), gotGVR.String())
	}
}

func Test_lookupCRD(t *testing.T) {
	cli := setupKubeClient(t)

	gvk := schema.GroupVersionKind{
		Group:   "core.krateo.io",
		Version: "v0-2-0",
		Kind:    "DummyChart",
	}

	ok, err := lookupCRD(context.Background(), cli, gvk)
	if err != nil {
		t.Fatal(err)
	}

	if ok {
		t.Logf("crd: %v, exists", gvk)
	} else {
		t.Logf("crd: %v, does not exists", gvk)
	}
}

func Test_installCRD(t *testing.T) {
	dat, err := os.ReadFile(testCRD)
	if err != nil {
		t.Fatal(err)
	}

	crd, err := unmarshalCRD(dat)
	if err != nil {
		t.Fatal(err)
	}

	cli := setupKubeClient(t)
	err = installCRD(context.Background(), cli, crd)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_unmarshalCRD(t *testing.T) {
	dat, err := os.ReadFile(testCRD)
	if err != nil {
		t.Fatal(err)
	}

	crd, err := unmarshalCRD(dat)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%v\n", crd.Spec.Names.Categories)
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

	_ = apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)

	cli, err := client.New(cfg, client.Options{})
	if err != nil {
		t.Fatal(err)
	}

	return cli
}
