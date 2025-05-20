package mutation

import (
	"context"
	"net/http"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func TestNewWebhookHandler(t *testing.T) {
	cli := fake.NewClientBuilder().Build()
	handler := NewWebhookHandler(cli)

	t.Run("should return error for invalid JSON", func(t *testing.T) {
		req := webhook.AdmissionRequest{
			AdmissionRequest: v1.AdmissionRequest{
				Object: runtime.RawExtension{Raw: []byte("invalid-json")},
			},
		}
		resp := handler.Handle(context.Background(), req)
		if resp.Result.Code != http.StatusBadRequest {
			t.Errorf("expected status code %d, got %d", http.StatusBadRequest, resp.Result.Code)
		}
	})

	t.Run("should return error if CRD is not found", func(t *testing.T) {
		req := webhook.AdmissionRequest{
			AdmissionRequest: v1.AdmissionRequest{
				Kind: metav1.GroupVersionKind{
					Group:   "example.com",
					Version: "v1",
					Kind:    "Example",
				},
				Resource: metav1.GroupVersionResource{
					Group:    "example.com",
					Version:  "v1",
					Resource: "examples",
				},
				Object: runtime.RawExtension{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"Example"}`)},
			},
		}
		resp := handler.Handle(context.Background(), req)
		if resp.Result.Code != http.StatusBadRequest {
			t.Errorf("expected status code %d, got %d", http.StatusBadRequest, resp.Result.Code)
		}
		assert.Contains(t, resp.Result.Message, "CRD not found")
	})

	t.Run("should add labels if none exist", func(t *testing.T) {
		crd := &apiextensionsv1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				Kind:       "CustomResourceDefinition",
				APIVersion: "apiextensions.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "examples.example.com",
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"field1": {
												Type:    "string",
												Default: &apiextensionsv1.JSON{Raw: []byte(`"default-value"`)},
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
		cli.Create(context.Background(), crd)

		req := webhook.AdmissionRequest{
			AdmissionRequest: v1.AdmissionRequest{
				Kind: metav1.GroupVersionKind{
					Group:   "example.com",
					Version: "v1",
					Kind:    "Example",
				},
				Resource: metav1.GroupVersionResource{
					Group:    "example.com",
					Version:  "v1",
					Resource: "examples",
				},
				Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"Example","metadata":{"name":"test"}}`)},
				Operation: v1.Create,
			},
		}
		resp := handler.Handle(context.Background(), req)
		if resp.Result.Code != http.StatusOK {
			t.Errorf("expected status code %d, got %d", http.StatusOK, resp.Result.Code)
		}

		patch := resp.Patches

		assert.Len(t, patch, 3)
		assert.Equal(t, "add", patch[0].Operation)
		assert.Equal(t, "/spec", patch[0].Path)
		assert.Equal(t, map[string]interface{}(map[string]interface{}{"field1": "default-value"}), patch[0].Value)
		assert.Equal(t, "add", patch[1].Operation)
		assert.Equal(t, "/metadata/labels", patch[1].Path)
		assert.Equal(t, map[string]string(map[string]string{}), patch[1].Value)
		assert.Equal(t, "add", patch[2].Operation)
		assert.Equal(t, "/metadata/labels/krateo.io~1composition-version", patch[2].Path)
		assert.Equal(t, "v1", patch[2].Value)
	})

	t.Run("should add krateo.io/composition-version label if labels exist", func(t *testing.T) {
		req := webhook.AdmissionRequest{
			AdmissionRequest: v1.AdmissionRequest{
				Kind: metav1.GroupVersionKind{
					Group:   "example.com",
					Version: "v1",
					Kind:    "Example",
				},
				Resource: metav1.GroupVersionResource{
					Group:    "example.com",
					Version:  "v1",
					Resource: "examples",
				},
				Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"Example","metadata":{"name":"test","labels":{"existing":"label"}}}`)},
				Operation: v1.Create,
			},
		}
		resp := handler.Handle(context.Background(), req)
		if resp.Result.Code != http.StatusOK {
			t.Errorf("expected status code %d, got %d", http.StatusOK, resp.Result.Code)
		}

		patch := resp.Patches

		assert.Len(t, patch, 2)
		assert.Equal(t, "add", patch[0].Operation)
		assert.Equal(t, "/spec", patch[0].Path)
		assert.Equal(t, map[string]interface{}(map[string]interface{}{"field1": "default-value"}), patch[0].Value)
		assert.Equal(t, "add", patch[1].Operation)
		assert.Equal(t, "/metadata/labels/krateo.io~1composition-version", patch[1].Path)
		assert.Equal(t, "v1", patch[1].Value)
	})

	t.Run("should not overwrite value of field with default if field is set", func(t *testing.T) {
		req := webhook.AdmissionRequest{
			AdmissionRequest: v1.AdmissionRequest{
				Kind: metav1.GroupVersionKind{
					Group:   "example.com",
					Version: "v1",
					Kind:    "Example",
				},
				Resource: metav1.GroupVersionResource{
					Group:    "example.com",
					Version:  "v1",
					Resource: "examples",
				},
				Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"Example","metadata":{"name":"test"},"spec":{"field1":"custom-value"}}`)},
				Operation: v1.Create,
			},
		}
		resp := handler.Handle(context.Background(), req)
		if resp.Result.Code != http.StatusOK {
			t.Errorf("expected status code %d, got %d", http.StatusOK, resp.Result.Code)
		}

		patch := resp.Patches

		assert.Len(t, patch, 2)
		assert.Equal(t, "add", patch[0].Operation)
		assert.Equal(t, "/metadata/labels", patch[0].Path)
		assert.Equal(t, map[string]string(map[string]string{}), patch[0].Value)
		assert.Equal(t, "add", patch[1].Operation)
		assert.Equal(t, "/metadata/labels/krateo.io~1composition-version", patch[1].Path)
		assert.Equal(t, "v1", patch[1].Value)
	})

	t.Run("Update operation should not add labels", func(t *testing.T) {
		req := webhook.AdmissionRequest{
			AdmissionRequest: v1.AdmissionRequest{
				Kind: metav1.GroupVersionKind{
					Group:   "example.com",
					Version: "v1",
					Kind:    "Example",
				},
				Resource: metav1.GroupVersionResource{
					Group:    "example.com",
					Version:  "v1",
					Resource: "examples",
				},
				Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"Example","metadata":{"name":"test"}}`)},
				Operation: v1.Update,
			},
		}
		resp := handler.Handle(context.Background(), req)
		if resp.Result.Code != http.StatusOK {
			t.Errorf("expected status code %d, got %d", http.StatusOK, resp.Result.Code)
		}

		assert.Len(t, resp.Patches, 1)
		assert.Equal(t, "add", resp.Patches[0].Operation)
		assert.Equal(t, "/spec", resp.Patches[0].Path)
		assert.Equal(t, map[string]interface{}(map[string]interface{}{"field1": "default-value"}), resp.Patches[0].Value)
	})
}
