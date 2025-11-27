package generation

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAppendVersion(t *testing.T) {
	tests := []struct {
		name     string
		crd      apiextensionsv1.CustomResourceDefinition
		toAdd    apiextensionsv1.CustomResourceDefinition
		expected apiextensionsv1.CustomResourceDefinition
	}{
		{
			name: "Append new versions",
			crd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1"},
					},
				},
			},
			toAdd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha2"},
					},
				},
			},
			expected: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1", Served: true, Storage: false},
						{Name: "v1alpha2", Served: true, Storage: false},
						{
							Name:    "vacuum",
							Served:  false,
							Storage: true,
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Type:        "object",
									Description: "This is a vacuum version to storage different versions",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"apiVersion": {
											Type:        "string",
											Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources",
										},
										"kind": {
											Type: "string",
										},
										"metadata": {
											Type: "object",
										},
										"spec": {
											Type:                   "object",
											XPreserveUnknownFields: &[]bool{true}[0],
										},
										"status": {
											Type:                   "object",
											XPreserveUnknownFields: &[]bool{true}[0],
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Append existing version",
			crd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1", Served: true, Storage: true},
					},
				},
			},
			toAdd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1"},
					},
				},
			},
			expected: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1", Served: true, Storage: true},
					},
				},
			},
		},
		{
			name: "Append version with existing vacuum",
			crd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1", Served: true, Storage: true},
					},
				},
			},
			toAdd: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha2"},
					},
				},
			},
			expected: apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1", Served: true, Storage: false},
						{Name: "v1alpha2", Served: true, Storage: false},
						{
							Name:    "vacuum",
							Served:  false,
							Storage: true,
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Type:        "object",
									Description: "This is a vacuum version to storage different versions",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"apiVersion": {
											Type:        "string",
											Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources",
										},
										"kind": {
											Type: "string",
										},
										"metadata": {
											Type: "object",
										},
										"spec": {
											Type:                   "object",
											XPreserveUnknownFields: &[]bool{true}[0],
										},
										"status": {
											Type:                   "object",
											XPreserveUnknownFields: &[]bool{true}[0],
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AppendVersion(tt.crd, tt.toAdd)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			ok := assert.ElementsMatch(t, result.Spec.Versions, tt.expected.Spec.Versions)
			if !ok {
				t.Log("Result:")
				t.Log(result.Spec.Versions)

				t.Log("Expected:")
				t.Log(tt.expected.Spec.Versions)

				t.Fatalf("Slice elements do not match")
			}

			if diff := cmp.Diff(result, &tt.expected); len(diff) > 0 {
				t.Fatalf("Unexpected result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	makeCRD := func(name string, versions ...apiextensionsv1.CustomResourceDefinitionVersion) *apiextensionsv1.CustomResourceDefinition {
		return &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural:   "foos",
					Singular: "foo",
					Kind:     "Foo",
				},
				Group:    "example.test",
				Scope:    apiextensionsv1.NamespaceScoped,
				Versions: versions,
			},
		}
	}

	t.Run("error when input version has nil schema", func(t *testing.T) {
		crd := makeCRD("crd",
			apiextensionsv1.CustomResourceDefinitionVersion{
				Name:   "v1",
				Schema: makeSchema(map[string]apiextensionsv1.JSONSchemaProps{"status": {Type: "object"}}),
				Served: true, Storage: true,
			},
		)
		ver := apiextensionsv1.CustomResourceDefinitionVersion{
			Name:   "v1",
			Schema: &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: nil}, // nil OpenAPI
		}
		if err := UpdateStatus(crd, ver); err == nil {
			t.Fatalf("expected error for nil schema on input version")
		}
	})

	t.Run("update status across all versions when version exists", func(t *testing.T) {
		// existing CRD with v1 and v2 each having a status property to be replaced
		crd := makeCRD("crd",
			apiextensionsv1.CustomResourceDefinitionVersion{
				Name: "v1",
				Schema: makeSchema(map[string]apiextensionsv1.JSONSchemaProps{
					"status": {Type: "object", Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"phase": {Type: "string"},
					}},
					"spec": {Type: "object"},
				}),
				Served: true, Storage: true,
			},
			apiextensionsv1.CustomResourceDefinitionVersion{
				Name: "v2",
				Schema: makeSchema(map[string]apiextensionsv1.JSONSchemaProps{
					"status": {Type: "object", Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"ready": {Type: "boolean"},
					}},
					"spec": {Type: "object"},
				}),
				Served: true, Storage: false,
			},
		)

		// incoming version carries the canonical/new status schema to propagate
		newStatusVersion := apiextensionsv1.CustomResourceDefinitionVersion{
			Name: "v2",
			Schema: makeSchema(map[string]apiextensionsv1.JSONSchemaProps{
				"status": {Type: "object", Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"conditions": {Type: "array"},
					"observed":   {Type: "string"},
				}},
			}),
		}

		err := UpdateStatus(crd, newStatusVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Verifica che lo schema di status sia stato aggiornato in tutte le versioni con schema valido
		want := newStatusVersion.Schema.OpenAPIV3Schema.Properties["status"]
		gotV1 := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"]
		gotV2 := crd.Spec.Versions[1].Schema.OpenAPIV3Schema.Properties["status"]

		if gotV1.Type != want.Type || len(gotV1.Properties) != len(want.Properties) {
			t.Fatalf("v1 status not updated, got=%v want=%v", gotV1, want)
		}
		if gotV2.Type != want.Type || len(gotV2.Properties) != len(want.Properties) {
			t.Fatalf("v2 status not updated, got=%v want=%v", gotV2, want)
		}
		// Assicura che altre proprietà rimangano intatte
		if _, ok := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]; !ok {
			t.Fatalf("v1 spec property should remain")
		}
	})

	t.Run("no panic if some versions have nil schema; others updated", func(t *testing.T) {
		crd := makeCRD("crd",
			apiextensionsv1.CustomResourceDefinitionVersion{
				Name:   "v1",
				Schema: nil, // deve essere ignorato
				Served: true, Storage: true,
			},
			apiextensionsv1.CustomResourceDefinitionVersion{
				Name: "v2",
				Schema: makeSchema(map[string]apiextensionsv1.JSONSchemaProps{
					"status": {Type: "object"},
				}),
				Served: true, Storage: false,
			},
		)
		newStatusVersion := apiextensionsv1.CustomResourceDefinitionVersion{
			Name: "v2",
			Schema: makeSchema(map[string]apiextensionsv1.JSONSchemaProps{
				"status": {Type: "object", Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"healthy": {Type: "boolean"},
				}},
			}),
		}
		err := UpdateStatus(crd, newStatusVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := crd.Spec.Versions[1].Schema.OpenAPIV3Schema.Properties["status"]
		if _, ok := got.Properties["healthy"]; !ok {
			t.Fatalf("v2 status should be updated with 'healthy'")
		}
	})
}

func TestSetServedStorage(t *testing.T) {
	tests := []struct {
		name     string
		crd      *apiextensionsv1.CustomResourceDefinition
		version  string
		served   bool
		storage  bool
		expected *apiextensionsv1.CustomResourceDefinition
	}{
		{
			name: "Set served and storage to true",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1"},
					},
				},
			},
			version: "v1alpha1",
			served:  true,
			storage: true,
			expected: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1", Served: true, Storage: true},
					},
				},
			},
		},
		{
			name: "Set served and storage to false",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1"},
					},
				},
			},
			version: "v1alpha1",
			served:  false,
			storage: false,
			expected: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1", Served: false, Storage: false},
					},
				},
			},
		},
		{
			name: "Version not found",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1"},
					},
				},
			},
			version: "v1alpha2",
			served:  true,
			storage: true,
			expected: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{Name: "v1alpha1"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetServedStorage(tt.crd, tt.version, tt.served, tt.storage)
			if diff := cmp.Diff(tt.crd, tt.expected); len(diff) > 0 {
				t.Fatalf("Unexpected result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestGenerateCRD_Success(t *testing.T) {
	spec := []byte(`{
        "type":"object",
        "properties":{
            "spec":{
                "type":"object",
                "properties":{
                    "foo":{"type":"string"}
                }
            }
        }
    }`)

	gvk := schema.GroupVersionKind{
		Group:   "widgets.example.org",
		Version: "v1beta1",
		Kind:    "Widget",
	}

	crd, err := GenerateCRD(spec, gvk)
	if err != nil {
		t.Fatalf("GenerateCRD returned error: %v", err)
	}
	if crd == nil {
		t.Fatalf("expected non-nil CRD")
	}

	if crd.Spec.Group != gvk.Group {
		t.Fatalf("unexpected group: got %s want %s", crd.Spec.Group, gvk.Group)
	}
	if crd.Spec.Names.Kind != gvk.Kind {
		t.Fatalf("unexpected kind: got %s want %s", crd.Spec.Names.Kind, gvk.Kind)
	}
	if len(crd.Spec.Versions) == 0 {
		t.Fatalf("generated CRD has no versions")
	}
	if crd.Spec.Versions[0].Name != gvk.Version {
		t.Fatalf("unexpected version name: got %s want %s", crd.Spec.Versions[0].Name, gvk.Version)
	}
	if crd.Spec.Versions[0].Schema == nil || crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
		t.Fatalf("generated CRD version schema is nil")
	}
	// ensure the provided spec schema produced a "spec" property
	if _, ok := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]; !ok {
		t.Fatalf("generated schema does not contain 'spec' property")
	}
}
func TestUpdateCABundle(t *testing.T) {
	tests := []struct {
		name      string
		crd       *apiextensionsv1.CustomResourceDefinition
		caBundle  []byte
		expectErr bool
		expected  *apiextensionsv1.CustomResourceDefinition
	}{
		{
			name: "Update CABundle successfully",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Conversion: &apiextensionsv1.CustomResourceConversion{
						Webhook: &apiextensionsv1.WebhookConversion{
							ClientConfig: &apiextensionsv1.WebhookClientConfig{},
						},
					},
				},
			},
			caBundle:  []byte("test-ca-bundle"),
			expectErr: false,
			expected: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Conversion: &apiextensionsv1.CustomResourceConversion{
						Webhook: &apiextensionsv1.WebhookConversion{
							ClientConfig: &apiextensionsv1.WebhookClientConfig{
								CABundle: []byte("test-ca-bundle"),
							},
						},
					},
				},
			},
		},
		{
			name: "Nil Conversion field",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{},
			},
			caBundle:  []byte("test-ca-bundle"),
			expectErr: true,
		},
		{
			name: "Nil Webhook field",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Conversion: &apiextensionsv1.CustomResourceConversion{},
				},
			},
			caBundle:  []byte("test-ca-bundle"),
			expectErr: true,
		},
		{
			name: "Nil ClientConfig field",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Conversion: &apiextensionsv1.CustomResourceConversion{
						Webhook: &apiextensionsv1.WebhookConversion{},
					},
				},
			},
			caBundle:  []byte("test-ca-bundle"),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UpdateCABundle(tt.crd, tt.caBundle)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, tt.crd)
			}
		})
	}
}

func TestStatusEquality_SuccessEqual(t *testing.T) {
	props := map[string]apiextensionsv1.JSONSchemaProps{
		"phase": {Type: "string"},
	}
	crd1 := makeCRDWithStatus("crd1", "v1", props)
	crd2 := makeCRDWithStatus("crd2", "v1", props)

	ok, err := StatusEqual(crd1, crd2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected equal status schemas")
	}
}

func TestStatusEquality(t *testing.T) {
	tests := []struct {
		name          string
		crd1          *apiextensionsv1.CustomResourceDefinition
		crd2          *apiextensionsv1.CustomResourceDefinition
		wantEqual     bool
		wantErrSubstr string
	}{
		{
			name:      "equal-status",
			crd1:      makeCRDWithStatus("crd1", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}}),
			crd2:      makeCRDWithStatus("crd2", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}}),
			wantEqual: true,
		},
		{
			name:      "unordered-equal-status",
			crd1:      makeCRDWithStatus("crd1", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}, "retries": {Type: "integer"}}),
			crd2:      makeCRDWithStatus("crd2", "v1", map[string]apiextensionsv1.JSONSchemaProps{"retries": {Type: "integer"}, "phase": {Type: "string"}}),
			wantEqual: true,
		},
		{
			name:      "unordered-equal-status",
			crd1:      makeCRDWithStatus("crd1", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}, "retries": {Type: "integer"}}),
			crd2:      makeCRDWithStatus("crd2", "v2", map[string]apiextensionsv1.JSONSchemaProps{"retries": {Type: "integer"}, "phase": {Type: "string"}}),
			wantEqual: true,
		},
		{
			name: "unordered-equal-complex-multiple-level-status",
			crd1: makeCRDWithStatus("crd1", "v1", map[string]apiextensionsv1.JSONSchemaProps{
				"phase": {Type: "string"},
				"details": {
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"lastUpdated": {Type: "string"},
						"info": {
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"count": {Type: "integer"},
								"notes": {Type: "string"},
							},
						},
					},
				},
			}),
			crd2: makeCRDWithStatus("crd2", "v1", map[string]apiextensionsv1.JSONSchemaProps{
				"details": {
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"info": {
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"notes": {Type: "string"},
								"count": {Type: "integer"},
							},
						},
						"lastUpdated": {Type: "string"},
					},
				},
				"phase": {Type: "string"},
			}),
			wantEqual: true,
		},
		{
			name:      "different-type-status",
			crd1:      makeCRDWithStatus("crd1", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}}),
			crd2:      makeCRDWithStatus("crd2", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "integer"}}),
			wantEqual: false,
		},
		{
			name: "different-status",
			crd1: makeCRDWithStatus("crd1", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}}),
			crd2: makeCRDWithStatus("crd2", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}, "retries": {Type: "integer"}}),
		},
		{
			name:          "schema-nil-crd1",
			crd1:          makeCRDWithNilSchema("crd1", "v1"),
			crd2:          makeCRDWithStatus("crd2", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}}),
			wantErrSubstr: "crd1 has no version with status property",
		},
		{
			name:          "schema-nil-crd2",
			crd1:          makeCRDWithStatus("crd1", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}}),
			crd2:          makeCRDWithNilSchema("crd2", "v1"),
			wantErrSubstr: "crd2 has no version with status property",
		},
		{
			name:          "openapi-nil-crd1",
			crd1:          makeCRDWithOpenAPINil("crd1", "v1"),
			crd2:          makeCRDWithStatus("crd2", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}}),
			wantErrSubstr: "crd1 has no version with status property",
		},
		{
			name:          "openapi-nil-crd2",
			crd1:          makeCRDWithStatus("crd1", "v1", map[string]apiextensionsv1.JSONSchemaProps{"phase": {Type: "string"}}),
			crd2:          makeCRDWithOpenAPINil("crd2", "v1"),
			wantErrSubstr: "crd2 has no version with status property",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := StatusEqual(tc.crd1, tc.crd2)

			if tc.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantEqual {
				t.Fatalf("want equal=%v, got %v", tc.wantEqual, got)
			}
		})
	}
}

func TestGetGVRFromGeneratedCRD_Success(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "example.test",
		Version: "v1",
		Kind:    "Foo",
	}
	specSchema := []byte(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title": "Foo",
		"type": "object"
	}`)

	gvr, err := GetGVRFromGeneratedCRD(specSchema, gvk)
	require.NoError(t, err)

	// Expect group and version to match the provided GVK and resource to be the pluralized kind
	assert.Equal(t, "example.test", gvr.Group)
	assert.Equal(t, "v1", gvr.Version)
	// crude pluralization expectation used elsewhere in tests: lowercase + "s"
	assert.Equal(t, "foos", gvr.Resource)
}

func TestGetGVRFromGeneratedCRD_GenerateError(t *testing.T) {

	emptyGVK := schema.GroupVersionKind{}

	_, err := GetGVRFromGeneratedCRD([]byte(`{}`), emptyGVK)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "error generating CRD for GVR fallback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func makeCRDWithStatus(name, version string, statusProps map[string]apiextensionsv1.JSONSchemaProps) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "foos",
				Singular: "foo",
				Kind:     "Foo",
			},
			Group: "example.test",
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: version,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"status": {
									Type:       "object",
									Properties: statusProps,
								},
							},
						},
					},
					Served:  true,
					Storage: true,
				},
			},
		},
	}
}

func makeCRDWithNilSchema(name, version string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "foos",
				Singular: "foo",
				Kind:     "Foo",
			},
			Group: "example.test",
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:   version,
					Schema: nil, // triggers "schema is nil"
				},
			},
		},
	}
}

func makeCRDWithOpenAPINil(name, version string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "foos",
				Singular: "foo",
				Kind:     "Foo",
			},
			Group: "example.test",
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: version,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: nil, // triggers "openapi schema is nil"
					},
				},
			},
		},
	}
}
func makeSchema(props map[string]apiextensionsv1.JSONSchemaProps) *apiextensionsv1.CustomResourceValidation {
	return &apiextensionsv1.CustomResourceValidation{
		OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
			Type:       "object",
			Properties: props,
		},
	}
}
