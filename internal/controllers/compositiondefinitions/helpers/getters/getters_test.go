package getters

import (
	"context"
	"testing"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
					deploy.CompositionVersionLabel: "v0-3-0",
				},
			},
		}}
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)

	gvr := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-3-0",
		Resource: "fireworksapps",
	}

	err := UpdateCompositionsVersion(context.Background(), dyn, gvr, "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}

	updatedComp, err := dyn.Resource(gvr).Namespace(obj1.GetNamespace()).Get(context.Background(), "test-composition", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated composition: %v", err)
	}

	labels, _, err := unstructured.NestedStringMap(updatedComp.Object, "metadata", "labels")
	if err != nil {
		t.Fatalf("failed to get labels from updated composition: %v", err)
	}

	if labels[deploy.CompositionVersionLabel] != "v2" {
		t.Errorf("expected composition version label 'v2', got '%s'", labels[deploy.CompositionVersionLabel])
	}
}
func TestGetCompositionDefinitions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = compositiondefinitionsv1alpha1.SchemeBuilder.AddToScheme(scheme)

	cli := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-composition-1",
				Namespace: "demo-system",
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "test-group/v1-0-0",
				Kind:       "TestKind",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "test-group",
					Kind:  "TestKind",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-composition-2",
				Namespace: "default",
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "test-group/v2-0-0",
				Kind:       "OtherKind",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "test-group",
					Kind:  "OtherKind",
				},
			},
		},

		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-composition-3",
				Namespace: "krateo-system",
			},
		},
	).Build()

	gk := schema.GroupKind{
		Group: "test-group",
		Kind:  "TestKind",
	}

	compositions, err := GetCompositionDefinitions(context.Background(), cli, gk)
	if err != nil {
		t.Fatalf("getCompositionDefinitions failed: %v", err)
	}

	if len(compositions) != 1 {
		t.Fatalf("expected 1 composition, got %d", len(compositions))
	}

	if compositions[0].Name != "test-composition-1" {
		t.Errorf("expected composition name 'test-composition-1', got '%s'", compositions[0].Name)
	}
}
func TestGetCompositionDefinitionsWithVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = compositiondefinitionsv1alpha1.SchemeBuilder.AddToScheme(scheme)

	cli := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-1",
				Namespace: "ns1",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "1.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g1/v1-0-0",
				Kind:       "Kind1",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g1",
					Kind:  "Kind1",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-2",
				Namespace: "ns2",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "2.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g1/v2-0-0",
				Kind:       "Kind1",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g1",
					Kind:  "Kind1",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-3",
				Namespace: "ns3",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "1.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g2/v1-0-0",
				Kind:       "Kind2",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g2",
					Kind:  "Kind2",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-4",
				Namespace: "ns4",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "4.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g2/v4-0-0",
				Kind:       "Kind2",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g2",
					Kind:  "Kind2",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-5",
				Namespace: "ns5",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "4.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g2/v4-0-0",
				Kind:       "Kind2",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g2",
					Kind:  "Kind2",
				},
			},
		},
	).Build()

	gk := schema.GroupKind{Group: "g1", Kind: "Kind1"}

	// Test: should return only cd-1 for version 1.0.0
	comps, err := GetCompositionDefinitionsWithVersion(context.Background(), cli, gk.WithVersion("v1-0-0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("expected 1 result, got %d", len(comps))
	}
	if comps[0].Name != "cd-1" {
		t.Errorf("expected cd-1, got %s", comps[0].Name)
	}

	// Test: should return only cd-2 for version 2.0.0
	comps, err = GetCompositionDefinitionsWithVersion(context.Background(), cli, gk.WithVersion("v2-0-0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("expected 1 result, got %d", len(comps))
	}
	if comps[0].Name != "cd-2" {
		t.Errorf("expected cd-2, got %s", comps[0].Name)
	}

	// Test: should return none for version that does not exist
	comps, err = GetCompositionDefinitionsWithVersion(context.Background(), cli, gk.WithVersion("v3-0-0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comps) != 0 {
		t.Fatalf("expected 0 results, got %d", len(comps))
	}

	// Test: should return none for group/kind mismatch
	gk2 := schema.GroupKind{Group: "g2", Kind: "Kind2"}
	comps, err = GetCompositionDefinitionsWithVersion(context.Background(), cli, gk2.WithVersion("v1-0-0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comps) != 1 || comps[0].Name != "cd-3" {
		t.Errorf("expected only cd-3, got %+v", comps)
	}

	comps, err = GetCompositionDefinitionsWithVersion(context.Background(), cli, gk2.WithVersion("v4-0-0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comps) != 2 {
		t.Fatalf("expected 2 results, got %d", len(comps))
	}

}
