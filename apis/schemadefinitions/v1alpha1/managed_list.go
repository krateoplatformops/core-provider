package v1alpha1

import "github.com/krateoplatformops/provider-runtime/pkg/resource"

func (l *SchemaDefinitionList) GetItems() []resource.Managed {
	items := make([]resource.Managed, len(l.Items))
	for i := range l.Items {
		items[i] = &l.Items[i]
	}
	return items
}
