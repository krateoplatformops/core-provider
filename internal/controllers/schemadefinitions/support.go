package schemadefinitions

import (
	"fmt"
	"io"
	"net/http"

	"github.com/krateoplatformops/core-provider/apis/schemadefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/ptr"
	"github.com/krateoplatformops/crdgen"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func toGVK(cr *v1alpha1.SchemaDefinition) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   defaultGroup,
		Version: ptr.Deref(cr.Spec.Schema.Version, defaultVersion),
		Kind:    cr.Spec.Schema.Kind,
	}
}

var _ crdgen.JsonSchemaGetter = (*urlJsonSchemaGetter)(nil)

func UrlJsonSchemaGetter(u string) crdgen.JsonSchemaGetter {
	return &urlJsonSchemaGetter{u}
}

type urlJsonSchemaGetter struct {
	uri string
}

func (f *urlJsonSchemaGetter) Get() ([]byte, error) {
	res, err := http.Get(f.uri)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	return io.ReadAll(http.MaxBytesReader(nil, res.Body, 512*1024))
}
