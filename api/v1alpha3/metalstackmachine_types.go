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
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	clustererr "sigs.k8s.io/cluster-api/errors"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MetalStackMachineSpec defines the desired state of MetalStackMachine
type MetalStackMachineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Image string `json:"image"`

	// +optional
	NetworkIDs []string `json:"networkIDs"`
	Partition  *string  `json:"partition"`

	// ID of the machine. This field is required by Cluster API.
	// +optional
	ProjectID *string `json:"projectID,omitempty"`
	// +optional
	ProviderID *string `json:"providerID"`
	// +optional
	Type *string `json:"type"`

	MachineType string   `json:"machineType"`
	SSHKeys     []string `json:"sshKeys,omitempty"`

	// HardwareReservationID is the unique machine hardware reservation ID or `next-available` to
	// automatically let the MetalStack api determine one.
	// +optional
	HardwareReservationID string `json:"hardwareReservationID,omitempty"`

	// Tags is an optional set of tags to add to MetalStack resources managed by the MetalStack provider.
	// +optional
	Tags Tags `json:"tags,omitempty"`
}

var ProviderIDNotSet = errors.New("ProviderID of the MetalStackMachineSpec not set")

func (spec *MetalStackMachineSpec) ParsedProviderID() (string, error) {
	unparsed := spec.ProviderID
	if unparsed == nil {
		return "", ProviderIDNotSet
	}
	parsed, err := noderefutil.NewProviderID(*unparsed)
	if err != nil {
		return "", err
	}
	return parsed.ID(), nil
}

func (spec *MetalStackMachineSpec) SetProviderID(ID string) {
	spec.ProjectID = pointer.StringPtr(fmt.Sprintf("metalstack://%v", ID))
}

// MetalStackMachineStatus defines the observed state of MetalStackMachine
type MetalStackMachineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Addresses contains the MetalStack machine associated addresses.
	// +optional
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

	// +optional
	Allocated bool `json:"allocated"`

	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	ErrorReason *clustererr.MachineStatusError `json:"errorReason,omitempty"`

	// ErrorMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`

	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// +optional
	FailureReason *clustererr.MachineStatusError `json:"failureReason,omitempty"`

	// InstanceStatus is the status of the MetalStack machine instance for this machine.
	// +optional
	InstanceStatus *MetalStackResourceStatus `json:"instanceStatus,omitempty"`

	// +optional
	LLDP bool `json:"lldp,omitempty"`

	// +optional
	Liveliness *string `json:"liveliness,omitempty"`

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`
}

// func (st *MetalStackMachineStatus) Erroneous() bool {
// 	return st.ErrorMessage != nil || st.ErrorReason != nil
// }

func (st *MetalStackMachineStatus) Failed() bool {
	return st.FailureMessage != nil || st.FailureReason != nil
}

func (st *MetalStackMachineStatus) SetFailure(msg string, err clustererr.MachineStatusError) {
	st.FailureMessage = &msg
	st.FailureReason = &err
}

// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=metalstackmachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this MetalStackMachine belongs"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.instanceState",description="MetalStack instance state"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:printcolumn:name="InstanceID",type="string",JSONPath=".spec.providerID",description="MetalStack instance ID"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this MetalStackMachine"

// MetalStackMachine is the Schema for the metalstackmachines API
type MetalStackMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetalStackMachineSpec   `json:"spec,omitempty"`
	Status MetalStackMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MetalStackMachineList contains a list of MetalStackMachine
type MetalStackMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetalStackMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetalStackMachine{}, &MetalStackMachineList{})
}
