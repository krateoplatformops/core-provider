//go:build !ignore_autogenerated

/*
Copyright 2023 Krateo SRL.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchemaDefinition) DeepCopyInto(out *SchemaDefinition) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchemaDefinition.
func (in *SchemaDefinition) DeepCopy() *SchemaDefinition {
	if in == nil {
		return nil
	}
	out := new(SchemaDefinition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SchemaDefinition) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchemaDefinitionList) DeepCopyInto(out *SchemaDefinitionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SchemaDefinition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchemaDefinitionList.
func (in *SchemaDefinitionList) DeepCopy() *SchemaDefinitionList {
	if in == nil {
		return nil
	}
	out := new(SchemaDefinitionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SchemaDefinitionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchemaDefinitionSpec) DeepCopyInto(out *SchemaDefinitionSpec) {
	*out = *in
	out.ManagedSpec = in.ManagedSpec
	in.Schema.DeepCopyInto(&out.Schema)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchemaDefinitionSpec.
func (in *SchemaDefinitionSpec) DeepCopy() *SchemaDefinitionSpec {
	if in == nil {
		return nil
	}
	out := new(SchemaDefinitionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchemaDefinitionStatus) DeepCopyInto(out *SchemaDefinitionStatus) {
	*out = *in
	in.ManagedStatus.DeepCopyInto(&out.ManagedStatus)
	if in.Digest != nil {
		in, out := &in.Digest, &out.Digest
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchemaDefinitionStatus.
func (in *SchemaDefinitionStatus) DeepCopy() *SchemaDefinitionStatus {
	if in == nil {
		return nil
	}
	out := new(SchemaDefinitionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchemaInfo) DeepCopyInto(out *SchemaInfo) {
	*out = *in
	if in.Version != nil {
		in, out := &in.Version, &out.Version
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchemaInfo.
func (in *SchemaInfo) DeepCopy() *SchemaInfo {
	if in == nil {
		return nil
	}
	out := new(SchemaInfo)
	in.DeepCopyInto(out)
	return out
}
