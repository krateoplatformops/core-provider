package defaults

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPopulateDefaultsFromCRD(t *testing.T) {
	tests := []struct {
		name      string
		crd       *apiextensionsv1.CustomResourceDefinition
		cr        *unstructured.Unstructured
		expected  map[string]interface{}
		expectErr bool
	}{
		{
			name: "No schema for version",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name:   "v1",
							Schema: nil,
						},
					},
				},
			},
			cr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Example",
					"spec":       map[string]interface{}{},
				},
			},
			expected:  map[string]interface{}{},
			expectErr: true,
		},
		{
			name: "Apply default values",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name: "v1",
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"spec": {
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"replicas": {
													Type:    "integer",
													Default: &apiextensionsv1.JSON{Raw: []byte("3")},
												},
												"replicacount": {
													Type:    "integer",
													Default: &apiextensionsv1.JSON{Raw: []byte("4")},
												},
												"nested": {
													Type: "object",
													Properties: map[string]apiextensionsv1.JSONSchemaProps{
														"nestedfield": {
															Type:    "integer",
															Default: &apiextensionsv1.JSON{Raw: []byte("5")},
														},
														"nesteddefault": {
															Type:    "string",
															Default: &apiextensionsv1.JSON{Raw: []byte("ciao")},
														},
														"nestedfloat": {
															Type:    "number",
															Default: &apiextensionsv1.JSON{Raw: []byte("3.14")},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			cr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Example",
					"spec": map[string]interface{}{
						"replicacount": int64(1),
						"nested": map[string]interface{}{
							"nestedfield":   int64(2),
							"nesteddefault": "krateo",
						},
					},
				},
			},
			expected: map[string]interface{}{
				"replicas":     int64(3),
				"replicacount": int64(1),
				"nested": map[string]interface{}{
					"nestedfield":   int64(2),
					"nesteddefault": "krateo",
					"nestedfloat":   float64(3.14),
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid apiVersion format",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{},
			},
			cr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "invalid",
					"kind":       "Example",
					"spec":       map[string]interface{}{},
				},
			},
			expected:  nil,
			expectErr: true,
		},
		{
			name: "No spec field in schema",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name: "v1",
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{},
							},
						},
					},
				},
			},
			cr: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Example",
					"spec":       map[string]interface{}{},
				},
			},
			expected:  map[string]interface{}{},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := runtime.Object(tt.cr)
			err := PopulateDefaultsFromCRD(tt.crd, &cr)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				result := tt.cr.Object["spec"].(map[string]interface{})
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
