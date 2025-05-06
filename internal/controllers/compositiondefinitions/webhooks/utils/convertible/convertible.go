package convertible

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

type Convertible struct {
	*unstructured.Unstructured
}

var _ conversion.Convertible = &Convertible{}

// ConvertTo converts this version to the Hub version.
func (src *Convertible) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*Hub)

	dst.Object["metadata"] = src.Object["metadata"]
	dst.Object["spec"] = src.Object["spec"]
	dst.Object["status"] = src.Object["status"]

	return nil
}

// ConvertFrom converts from the Hub version to this version.
func (dst *Convertible) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*Hub)

	dst.Object["metadata"] = src.Object["metadata"]
	dst.Object["spec"] = src.Object["spec"]
	dst.Object["status"] = src.Object["status"]

	return nil
}

func (c Convertible) DeepCopyObject() runtime.Object {
	return c.Unstructured.DeepCopyObject()
}

func (c Convertible) GetObjectKind() schema.ObjectKind {
	return c.Unstructured.GetObjectKind()
}

func CreateEmptyConvertible(apiVersion string, kind string) *Convertible {
	un := &unstructured.Unstructured{}
	un.SetAPIVersion(apiVersion)
	un.SetKind(kind)
	return &Convertible{Unstructured: un}
}
