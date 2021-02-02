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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const MetalStackFirewallFinalizer = "metalstackfirewall.infrastructure.cluster.x-k8s.io"

// MetalStackFirewallSpec defines the desired state of MetalStackFirewall
type MetalStackFirewallSpec struct {
	// OS image
	Image string `json:"image,omitempty"`

	// Machine type(currently specifies only size)
	MachineType string `json:"machineType"`

	// ProviderID sepecifies the machine on which the firewall should be deployed
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// public SSH keys for machine
	// +optional
	SSHKeys []string `json:"sshKeys,omitempty"`
}

func (s *MetalStackFirewallSpec) ParsedProviderID() (string, error) {
	unparsed := s.ProviderID
	if unparsed == nil {
		return "", ProviderIDNotSet
	}
	parsed, err := noderefutil.NewProviderID(*unparsed)
	if err != nil {
		return "", err
	}
	return parsed.ID(), nil
}

func (s *MetalStackFirewallSpec) SetProviderID(ID string) {
	s.ProviderID = pointer.StringPtr("metalstack://" + ID)
}

// MetalStackFirewallStatus defines the observed state of MetalStackFirewall
type MetalStackFirewallStatus struct {
	Ready bool `json:"ready,omitempty"`
}

// +kubebuilder:object:root=true

// MetalStackFirewall is the Schema for the metalstackfirewalls API
type MetalStackFirewall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetalStackFirewallSpec   `json:"spec,omitempty"`
	Status MetalStackFirewallStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MetalStackFirewallList contains a list of MetalStackFirewall
type MetalStackFirewallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetalStackFirewall `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetalStackFirewall{}, &MetalStackFirewallList{})
}
