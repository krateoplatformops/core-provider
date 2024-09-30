package compositiondefinitions

import (
	"context"
	"testing"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestUpdateVersionInfo(t *testing.T) {
	cr := &compositiondefinitionsv1alpha1.CompositionDefinition{
		Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
			Managed: compositiondefinitionsv1alpha1.Managed{
				VersionInfo: []compositiondefinitionsv1alpha1.VersionDetail{},
			},
		},
		Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
			Chart: &compositiondefinitionsv1alpha1.ChartInfo{
				Repo:    "test-repo",
				Url:     "test-url",
				Version: "test-version",
			},
		},
	}

	crd := &apiextensionsv1.CustomResourceDefinition{
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
				},
			},
		},
	}

	gvr := schema.GroupVersionResource{
		Group:    "test-group",
		Version:  "v1",
		Resource: "test-resource",
	}

	updateVersionInfo(cr, crd, gvr)

	if len(cr.Status.Managed.VersionInfo) != 1 {
		t.Fatalf("expected 1 version info, got %d", len(cr.Status.Managed.VersionInfo))
	}

	vi := cr.Status.Managed.VersionInfo[0]
	if vi.Version != "v1" {
		t.Errorf("expected version 'v1', got '%s'", vi.Version)
	}
	if vi.Chart == nil {
		t.Fatal("expected chart info to be set")
	}
}

func TestUpdateCompositionsVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	obj1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "composition.krateo.io/v0-3-0",
			"kind":       "FireworksApp",
			"metadata": map[string]interface{}{
				"name":      "test-composition",
				"namespace": "default",
				"labels": map[string]interface{}{
					CompositionVersionLabel: "v0-3-0",
				},
			},
		}}
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)

	log := logging.NewNopLogger()

	opts := CompositionsInfo{
		GVR: schema.GroupVersionResource{
			Group:    "composition.krateo.io",
			Version:  "v0-3-0",
			Resource: "fireworksapps",
		},
		Namespace: "default",
	}

	err := updateCompositionsVersion(context.Background(), dyn, log, opts, "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}

	updatedComp, err := dyn.Resource(opts.GVR).Namespace(opts.Namespace).Get(context.Background(), "test-composition", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated composition: %v", err)
	}

	labels, _, err := unstructured.NestedStringMap(updatedComp.Object, "metadata", "labels")
	if err != nil {
		t.Fatalf("failed to get labels from updated composition: %v", err)
	}

	if labels[CompositionVersionLabel] != "v2" {
		t.Errorf("expected composition version label 'v2', got '%s'", labels[CompositionVersionLabel])
	}
}
