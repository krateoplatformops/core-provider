package v1alpha1

import (
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
)

func (mg *SchemaDefinition) GetCondition(ct rtv1.ConditionType) rtv1.Condition {
	return mg.Status.GetCondition(ct)
}

func (mg *SchemaDefinition) GetDeletionPolicy() rtv1.DeletionPolicy {
	return mg.Spec.DeletionPolicy
}

func (mg *SchemaDefinition) SetConditions(c ...rtv1.Condition) {
	mg.Status.SetConditions(c...)
}

func (mg *SchemaDefinition) SetDeletionPolicy(r rtv1.DeletionPolicy) {
	mg.Spec.DeletionPolicy = r
}
