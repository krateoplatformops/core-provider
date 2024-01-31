package generator

import (
	"io"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func FakeGroupVersionKindGetter(kind string) GroupVersionKindGetter {
	return &fakeGroupVersionKindGetter{
		kind: kind,
	}
}

func StreamValuesSchemaGetter(r io.Reader) ValuesSchemaGetter {
	return &fakeValuesSchemaGetter{
		fin: r,
	}
}

var _ GroupVersionKindGetter = (*fakeGroupVersionKindGetter)(nil)

type fakeGroupVersionKindGetter struct {
	kind string
}

func (g *fakeGroupVersionKindGetter) GVK() (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{
		Group:   defaultGroup,
		Version: "v0-0-0",
		Kind:    g.kind,
	}, nil
}

var _ ValuesSchemaGetter = (*fakeValuesSchemaGetter)(nil)

type fakeValuesSchemaGetter struct {
	fin io.Reader
}

func (g *fakeValuesSchemaGetter) ValuesSchemaBytes() ([]byte, error) {
	return io.ReadAll(g.fin)
}
