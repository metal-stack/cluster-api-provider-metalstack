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

package scope

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MachineScopeParams defines the input parameters used to create a new MachineScope.
type MachineScopeParams struct {
	Client            client.Client
	Logger            logr.Logger
	Cluster           *clusterv1.Cluster
	Machine           *clusterv1.Machine
	MetalStackCluster *infrav1.MetalStackCluster
	MetalStackMachine *infrav1.MetalStackMachine
}

// NewMachineScope creates a new MachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration
// both MetalStackClusterReconciler and MetalStackMachineReconciler.
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("Client is required when creating a MachineScope")
	}
	if params.Machine == nil {
		return nil, errors.New("Machine is required when creating a MachineScope")
	}
	if params.Cluster == nil {
		return nil, errors.New("Cluster is required when creating a MachineScope")
	}
	if params.MetalStackCluster == nil {
		return nil, errors.New("MetalStackCluster is required when creating a MachineScope")
	}
	if params.MetalStackMachine == nil {
		return nil, errors.New("MetalStackMachine is required when creating a MachineScope")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.MetalStackMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &MachineScope{
		client:            params.Client,
		Cluster:           params.Cluster,
		Machine:           params.Machine,
		MetalStackCluster: params.MetalStackCluster,
		MetalStackMachine: params.MetalStackMachine,
		Logger:            params.Logger,
		patchHelper:       helper,
	}, nil
}

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster           *clusterv1.Cluster
	Machine           *clusterv1.Machine
	MetalStackCluster *infrav1.MetalStackCluster
	MetalStackMachine *infrav1.MetalStackMachine
}

// Close the MachineScope by updating the machine spec, machine status.
func (m *MachineScope) Close() error {
	return m.patchHelper.Patch(context.TODO(), m.MetalStackMachine)
}

// Name returns the MetalStackMachine name
func (m *MachineScope) Name() string {
	return m.MetalStackMachine.Name
}

// Namespace returns the MetalStackMachine namespace
func (m *MachineScope) Namespace() string {
	return m.MetalStackMachine.Namespace
}

// IsControlPlane returns true if the machine is a control plane.
func (m *MachineScope) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// Role returns the machine role from the labels.
func (m *MachineScope) Role() string {
	if util.IsControlPlaneMachine(m.Machine) {
		return infrav1.MasterTag
	}
	return infrav1.WorkerTag
}

// GetProviderID returns the DOMachine providerID from the spec.
func (m *MachineScope) GetProviderID() string {
	if m.MetalStackMachine.Spec.ProviderID != nil {
		return *m.MetalStackMachine.Spec.ProviderID
	}
	return ""
}

// SetProviderID sets the DOMachine providerID in spec from machine id.
func (m *MachineScope) SetProviderID(machineID string) {
	pid := fmt.Sprintf("metalstack://%s", machineID)
	m.MetalStackMachine.Spec.ProviderID = pointer.StringPtr(pid)
}

// SetPrivateNetworkID sets private NetworkID in spec from machine id.
func (m *MachineScope) SetPrivateNetworkID(privateNetworkID string) {
	*m.MetalStackCluster.Spec.PrivateNetworkID = privateNetworkID
}

// GetInstanceID returns the DOMachine droplet instance id by parsing Spec.ProviderID.
func (m *MachineScope) GetInstanceID() string {
	parsed, err := noderefutil.NewProviderID(m.GetProviderID())
	if err != nil {
		return ""
	}
	return parsed.ID()
}

// GetInstanceStatus returns the MetalStackMachine machine instance status from the status.
func (m *MachineScope) GetInstanceStatus() *infrav1.MetalStackResourceStatus {
	return m.MetalStackMachine.Status.InstanceStatus
}

// SetInstanceStatus sets the MetalStackMachine machine id.
func (m *MachineScope) SetInstanceStatus(v infrav1.MetalStackResourceStatus) {
	m.MetalStackMachine.Status.InstanceStatus = &v
}

// SetReady sets the MetalStackMachine Ready Status
func (m *MachineScope) SetReady() {
	m.MetalStackMachine.Status.Ready = true
}

// SetErrorMessage sets the MetalStackMachine status error message.
func (m *MachineScope) SetErrorMessage(v error) {
	m.MetalStackMachine.Status.ErrorMessage = pointer.StringPtr(v.Error())
}

// SetErrorReason sets the MetalStackMachine status error reason.
func (m *MachineScope) SetErrorReason(v capierrors.MachineStatusError) {
	m.MetalStackMachine.Status.ErrorReason = &v
}

// SetAddresses sets the address status.
func (m *MachineScope) SetAddresses(addrs []corev1.NodeAddress) {
	m.MetalStackMachine.Status.Addresses = addrs
}

// AdditionalTags returns Tags from the scope's MetalStackMachine. The returned value will never be nil.
func (m *MachineScope) Tags() infrav1.Tags {
	if m.MetalStackMachine.Spec.Tags == nil {
		m.MetalStackMachine.Spec.Tags = infrav1.Tags{}
	}

	return m.MetalStackMachine.Spec.Tags.DeepCopy()
}

// GetRawBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetRawBootstrapData() ([]byte, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.Namespace(), Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	if err := m.client.Get(context.TODO(), key, secret); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve bootstrap data secret for MetalStackMachine %s/%s", m.Namespace(), m.Name())
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}
