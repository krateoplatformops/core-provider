package pluralizer

import (
	"github.com/krateoplatformops/plumbing/cache"
	"github.com/krateoplatformops/plumbing/kubeutil/plurals"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PluralizerInterface interface {
	GVKtoGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error)
}

type Pluralizer struct {
	cache *cache.TTLCache[string, plurals.Info]
}

func New(cached bool) *Pluralizer {
	if cached {
		return &Pluralizer{
			cache: cache.NewTTL[string, plurals.Info](),
		}
	}

	return &Pluralizer{}
}

func (p Pluralizer) GVKtoGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	info, err := plurals.Get(gvk, plurals.GetOptions{})
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: info.Plural,
	}, nil
}
