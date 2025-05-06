package convertible

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

type Hub struct {
	*unstructured.Unstructured
}

var _ conversion.Hub = &Hub{}

func (h Hub) Hub() {}

func (h Hub) DeepCopyObject() runtime.Object {
	return h.Unstructured.DeepCopyObject()
}

func (h Hub) GetObjectKind() schema.ObjectKind {
	return h.Unstructured.GetObjectKind()
}
