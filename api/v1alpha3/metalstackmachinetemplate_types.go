/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MetalStackMachineTemplateSpec defines the desired state of MetalStackMachineTemplate
type MetalStackMachineTemplateSpec struct {
	Template MetalStackMachineTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=metalstackmachinetemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// MetalStackMachineTemplate is the Schema for the metalstackmachinetemplates API
type MetalStackMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MetalStackMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// MetalStackMachineTemplateList contains a list of metalstackMachineTemplate
type MetalStackMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetalStackMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetalStackMachineTemplate{}, &MetalStackMachineTemplateList{})
}
