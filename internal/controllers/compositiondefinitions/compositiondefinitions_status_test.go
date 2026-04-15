package compositiondefinitions

import (
	"testing"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRefreshCompositionDefinitionStatusUpdatesVersionInfoDuringUpdate(t *testing.T) {
	cr := &compositiondefinitionsv1alpha1.CompositionDefinition{
		Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
			ApiVersion: "composition.krateo.io/v1-0-5-3",
			Kind:       "KrateoPortalBlueprintPage",
			Resource:   "krateoportalblueprintpages",
			Managed: compositiondefinitionsv1alpha1.Managed{
				Group: "composition.krateo.io",
				Kind:  "KrateoPortalBlueprintPage",
				VersionInfo: []compositiondefinitionsv1alpha1.VersionDetail{
					{
						Version: "v1-0-5-3",
						Served:  true,
						Stored:  false,
						Chart: &compositiondefinitionsv1alpha1.ChartInfoProps{
							Repo:    "krateo-portal-blueprint-page",
							Url:     "https://nexus.insiel.it/repository/helm-hosted/",
							Version: "1.0.5-3",
						},
					},
					{
						Version: "vacuum",
						Served:  false,
						Stored:  true,
					},
				},
			},
		},
		Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
			Chart: &compositiondefinitionsv1alpha1.ChartInfo{
				Repo:    "krateo-portal-blueprint-page",
				Url:     "https://nexus.insiel.it/repository/helm-hosted/",
				Version: "1.0.5-4",
			},
		},
	}

	crd := &apiextensionsv1.CustomResourceDefinition{
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "composition.krateo.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "KrateoPortalBlueprintPage",
				Plural: "krateoportalblueprintpages",
			},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1-0-5-3",
					Served:  true,
					Storage: false,
				},
				{
					Name:    "vacuum",
					Served:  false,
					Storage: true,
				},
				{
					Name:    "v1-0-5-4",
					Served:  true,
					Storage: false,
				},
			},
		},
	}

	gvr := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v1-0-5-4",
		Resource: "krateoportalblueprintpages",
	}
	gvk := schema.GroupVersionKind{
		Group:   "composition.krateo.io",
		Version: "v1-0-5-4",
		Kind:    "KrateoPortalBlueprintPage",
	}

	if err := refreshCompositionDefinitionStatus(cr, crd, gvr, gvk, "oci://repo/krateo-portal-blueprint-page:1.0.5-4", logging.NewNopLogger()); err != nil {
		t.Fatalf("refreshCompositionDefinitionStatus() error = %v", err)
	}

	if cr.Status.ApiVersion != "composition.krateo.io/v1-0-5-4" {
		t.Fatalf("expected status apiVersion to be updated, got %q", cr.Status.ApiVersion)
	}
	if cr.Status.Kind != "KrateoPortalBlueprintPage" {
		t.Fatalf("expected status kind to be updated, got %q", cr.Status.Kind)
	}
	if cr.Status.Resource != "krateoportalblueprintpages" {
		t.Fatalf("expected status resource to be updated, got %q", cr.Status.Resource)
	}
	if cr.Status.PackageURL != "oci://repo/krateo-portal-blueprint-page:1.0.5-4" {
		t.Fatalf("expected package URL to be updated, got %q", cr.Status.PackageURL)
	}
	if cr.Status.Managed.Group != "composition.krateo.io" {
		t.Fatalf("expected managed group to be updated, got %q", cr.Status.Managed.Group)
	}
	if cr.Status.Managed.Kind != "KrateoPortalBlueprintPage" {
		t.Fatalf("expected managed kind to be updated, got %q", cr.Status.Managed.Kind)
	}

	if len(cr.Status.Managed.VersionInfo) != 3 {
		t.Fatalf("expected 3 version entries, got %d", len(cr.Status.Managed.VersionInfo))
	}

	var foundNew bool
	for _, vi := range cr.Status.Managed.VersionInfo {
		switch vi.Version {
		case "v1-0-5-4":
			foundNew = true
			if vi.Chart == nil {
				t.Fatal("expected chart info for the new version")
			}
			if vi.Chart.Version != "1.0.5-4" {
				t.Fatalf("expected new version chart to be refreshed, got %q", vi.Chart.Version)
			}
		case "v1-0-5-3":
			if vi.Chart == nil {
				t.Fatal("expected historical version chart info to be preserved")
			}
			if vi.Chart.Version != "1.0.5-3" {
				t.Fatalf("expected historical version chart to be preserved, got %q", vi.Chart.Version)
			}
		case "vacuum":
			if vi.Chart != nil {
				t.Fatal("expected vacuum version chart to remain nil")
			}
		}
	}

	if !foundNew {
		t.Fatal("expected new version entry to be added to status")
	}
}
