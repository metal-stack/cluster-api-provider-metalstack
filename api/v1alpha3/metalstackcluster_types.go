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
	"github.com/metal-stack/metal-lib/pkg/tag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterapi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MetalStackClusterSpec defines the desired state of MetalStackCluster
type MetalStackClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	ControlPlaneEndpoint v1alpha3.APIEndpoint `json:"controlPlaneEndpoint"`

	// ProjectID is the projectID of the project in which K8s cluster should be deployed
	ProjectID string `json:"projectID"`

	// Partition is the physical location where the cluster will be created
	Partition string `json:"partition"`

	// Firewall is cluster's firewall config
	Firewall Firewall `json:"firewall"`

	// PrivateNetworkID is the id of the network which connects machines together. Shouldn't be filled manually in manifest.
	// MetalStackCluster controller will allocate network and set it's ID in this field.
	// +optional
	PrivateNetworkID *string `json:"privateNetworkID,omitempty"`
}

// MetalStackClusterStatus defines the observed state of MetalStackCluster
type MetalStackClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready denotes that the cluster (infrastructure) is ready.
	// +optional
	Ready bool `json:"ready"`

	// ControlPlaneIPAllocated denotes that IP for Control Plane was allocated successfully.
	ControlPlaneIPAllocated bool `json:"controlPlaneIPAllocated"`

	// PrivateNetworkAllocated denotes that network was allocated successfully.
	PrivateNetworkAllocated bool `json:"privateNetworkAllocated"`

	// todo: Consider CR Firewall.
	// +optional
	FirewallReady bool `json:"firewallReady,omitempty"`

	// FailureReason indicates there is a fatal problem reconciling the provider’s infrastructure.
	// Meant to be suitable for programmatic interpretation
	// +optional
	FailureReason *capierrors.ClusterStatusError `json:"failureReason,omitempty"`

	// FailureMessage indicates there is a fatal problem reconciling the provider’s infrastructure.
	// Meant to be a more descriptive value than failureReason
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
}

// +kubebuilder:subresource:status
// +kubebuilder:object:root=true

// MetalStackCluster is the Schema for the MetalStackclusters API
type MetalStackCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetalStackClusterSpec   `json:"spec,omitempty"`
	Status MetalStackClusterStatus `json:"status,omitempty"`
}

func (cluster *MetalStackCluster) ControlPlaneTags() []string {
	return []string{
		tag.ClusterID + "=" + cluster.Name,
		clusterapi.MachineControlPlaneLabelName + "=true",
	}
}

// +kubebuilder:object:root=true

// MetalStackClusterList contains a list of MetalStackCluster
type MetalStackClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetalStackCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetalStackCluster{}, &MetalStackClusterList{})
}
