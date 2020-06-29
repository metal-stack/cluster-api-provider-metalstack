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

// MetalMachineTemplateSpec defines the desired state of MetalMachineTemplate
type MetalMachineTemplateSpec struct {
	Template MetalMachineTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=metalmachinetemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// MetalMachineTemplate is the Schema for the metalmachinetemplates API
type MetalMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MetalMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// MetalMachineTemplateList contains a list of metalMachineTemplate
type MetalMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetalMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetalMachineTemplate{}, &MetalMachineTemplateList{})
}
