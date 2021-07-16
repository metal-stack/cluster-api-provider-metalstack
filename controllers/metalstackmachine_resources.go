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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/metal-stack/metal-go/api/models"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha4"
)

type metalStackMachineResources struct {
	logger logr.Logger
	client client.Client

	cluster      *capiv1.Cluster
	machine      *capiv1.Machine
	metalCluster *api.MetalStackCluster
	metalMachine *api.MetalStackMachine
}

func newMetalStackMachineResources(
	ctx context.Context,
	logger logr.Logger,
	k8sClient client.Client,
	namespacedName types.NamespacedName,
) (*metalStackMachineResources, error) {
	metalMachine := &api.MetalStackMachine{}
	if err := k8sClient.Get(ctx, namespacedName, metalMachine); err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	machine, err := util.GetOwnerMachine(ctx, k8sClient, metalMachine.ObjectMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve MetalStackMachine's owner machine: %w", err)
	}
	if machine == nil {
		logger.Info("Waiting for Machine Controller to set OwnerRef on MetalStackMachine")
		return nil, nil
	}

	if metalMachine.Status.Failed() {
		logger.Info("MetalStackMachine is failing")
		return nil, nil
	}

	cluster, err := util.GetClusterFromMetadata(ctx, k8sClient, machine.ObjectMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Cluster resource: %w", err)
	}
	if cluster == nil {
		logger.Info(fmt.Sprintf("Machine not associated with a cluster using the label %s: <name of cluster>", capiv1.ClusterLabelName))
		return nil, nil
	}

	metalClusterNamespacedName := types.NamespacedName{
		Namespace: metalMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	metalCluster := getMetalStackCluster(ctx, logger, k8sClient, metalClusterNamespacedName)
	if metalCluster == nil {
		return nil, nil
	}

	return &metalStackMachineResources{
		logger: logger,
		client: k8sClient,

		cluster:      cluster,
		machine:      machine,
		metalCluster: metalCluster,
		metalMachine: metalMachine,
	}, nil
}

// isReady checks if all resources are ready
func (r *metalStackMachineResources) isReady() bool {
	if !r.cluster.Status.InfrastructureReady {
		r.logger.Info("Cluster infrastructure isn't ready yet")
		return false
	}

	if r.machine.Spec.Bootstrap.DataSecretName == nil {
		r.logger.Info("Bootstrap secret isn't ready yet")
		return false
	}

	return true
}

func (r *metalStackMachineResources) getBootstrapData(ctx context.Context) ([]byte, error) {
	secretName := r.machine.Spec.Bootstrap.DataSecretName
	if secretName == nil {
		return nil, fmt.Errorf("Owner Machine's Spec.Bootstrap.DataSecretName being nil")
	}

	secret := &core.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: r.metalMachine.Namespace,
		Name:      *secretName,
	}
	if err := r.client.Get(ctx, namespacedName, secret); err != nil {
		return nil, err
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, fmt.Errorf("Key 'value' is missing in bootstrap secret")
	}

	return value, nil
}

// isDeletionTimestampZero checks DeletionTimestamp of MetalStackMachine
func (r *metalStackMachineResources) isDeletionTimestampZero() bool {
	return r.metalMachine.ObjectMeta.DeletionTimestamp.IsZero()
}

// isControlPlane checks if machine is intended to be ControlPlane
func (r *metalStackMachineResources) isControlPlane() bool {
	return util.IsControlPlaneMachine(r.machine)
}

// getTagsForRawMachine returns slice of tags for raw MetalStack machine
func (r *metalStackMachineResources) getTagsForRawMachine() (tags []string) {
	tags = append(
		[]string{r.metalCluster.GetClusterIDTag()},
		r.metalMachine.Spec.Tags...,
	)

	tags = append(tags, fmt.Sprintf("%s=%t", capiv1.MachineControlPlaneLabelName, r.isControlPlane()))

	return
}

// setProviderID sets ID of raw metal stack machine
func (r *metalStackMachineResources) setProviderID(rawMachine *models.V1MachineResponse) {
	r.metalMachine.Spec.SetProviderID(*rawMachine.ID)
	r.metalMachine.Status.Addresses = toNodeAddrs(rawMachine)
}

// getProviderID returns ID of raw metal stack machine
func (r *metalStackMachineResources) getProviderID() *string {
	return r.metalMachine.Spec.ProviderID
}

func toNodeAddrs(machine *models.V1MachineResponse) []core.NodeAddress {
	addrs := []core.NodeAddress{}
	for _, n := range machine.Allocation.Networks {
		t := core.NodeExternalIP
		if *n.Private {
			t = core.NodeInternalIP
		}
		addrs = append(addrs, core.NodeAddress{
			Type:    t,
			Address: n.Ips[0],
		})
	}
	return addrs
}
