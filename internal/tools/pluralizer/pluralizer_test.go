//go:build integration
// +build integration

package pluralizer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGVKtoGVR(t *testing.T) {
	tests := []struct {
		name       string
		gvk        schema.GroupVersionKind
		response   string
		statusCode int
		want       schema.GroupVersionResource
		wantErr    bool
	}{
		{
			name: "valid response",
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			response:   `{"plural": "deployments", "singular": "deployment", "shorts": ["deploy"]}`,
			statusCode: http.StatusOK,
			want: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			wantErr: false,
		},
		{
			name: "invalid JSON response",
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			response:   `invalid json`,
			statusCode: http.StatusOK,
			want:       schema.GroupVersionResource{},
			wantErr:    true,
		},
		{
			name: "non-200 status code",
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			response:   `{"plural": "deployments", "singular": "deployment", "shorts": ["deploy"]}`,
			statusCode: http.StatusInternalServerError,
			want:       schema.GroupVersionResource{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			urlPlurals := server.URL
			p := New(&urlPlurals, server.Client())

			got, err := p.GVKtoGVR(tt.gvk)
			if (err != nil) != tt.wantErr {
				t.Errorf("GVKtoGVR() error = %v, wantErr %v - name: %s", err, tt.wantErr, tt.name)
				return
			}
			if got != tt.want {
				t.Errorf("GVKtoGVR() got = %v, want %v - name: %s", got, tt.want, tt.name)
			}
		})
	}
}
