package definitions

import (
	"context"
	"os"
	"path"
	"testing"

	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestLookupCRD(t *testing.T) {
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

	gvr := schema.GroupVersionResource{
		Group:    "core.krateo.io",
		Version:  "v1alpha1",
		Resource: "definitions",
	}

	ext := &external{kube: cli}
	ok, err := ext.lookupCRD(context.Background(), gvr)
	if err != nil {
		t.Fatal(err)
	}

	if ok {
		t.Logf("crd: %v, exists", gvr)
	} else {
		t.Logf("crd: %v, does not exists", gvr)
	}
}
