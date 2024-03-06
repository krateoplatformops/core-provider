package v1alpha1

import (
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
)

// GetCondition of this CompositionDefinition.
func (mg *CompositionDefinition) GetCondition(ct rtv1.ConditionType) rtv1.Condition {
	return mg.Status.GetCondition(ct)
}

// GetDeletionPolicy of this CompositionDefinition.
func (mg *CompositionDefinition) GetDeletionPolicy() rtv1.DeletionPolicy {
	return mg.Spec.DeletionPolicy
}

// SetConditions of this CompositionDefinition.
func (mg *CompositionDefinition) SetConditions(c ...rtv1.Condition) {
	mg.Status.SetConditions(c...)
}

// SetDeletionPolicy of this CompositionDefinition.
func (mg *CompositionDefinition) SetDeletionPolicy(r rtv1.DeletionPolicy) {
	mg.Spec.DeletionPolicy = r
}
