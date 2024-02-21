package generator

import (
	"io"

	"github.com/krateoplatformops/crdgen"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func FakeGroupVersionKind(kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   defaultGroup,
		Version: "v0-0-0",
		Kind:    kind,
	}
}

func StreamJsonSchemaGetter(r io.Reader) crdgen.JsonSchemaGetter {
	return &fakeJsonSchemaGetter{
		fin: r,
	}
}

var _ crdgen.JsonSchemaGetter = (*fakeJsonSchemaGetter)(nil)

type fakeJsonSchemaGetter struct {
	fin io.Reader
}

func (g *fakeJsonSchemaGetter) Get() ([]byte, error) {
	return io.ReadAll(g.fin)
}
