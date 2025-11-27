package status

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
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

	UpdateVersionInfo(cr, crd, gvr)

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
