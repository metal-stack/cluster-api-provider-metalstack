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
	api "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha4"
	metalgo "github.com/metal-stack/metal-go"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubeconfigSecretNameTemplate = "%s-kubeconfig" // cluster_name-kubeconfig
)

func toMachineNetworks(networks ...string) (machineNetworks []metalgo.MachineAllocationNetwork) {
	for _, network := range networks {
		machineNetworks = append(machineNetworks, metalgo.MachineAllocationNetwork{
			NetworkID:   network,
			Autoacquire: true,
		})
	}
	return
}

func getMetalStackCluster(ctx context.Context, logger logr.Logger, k8sClient client.Client, namespacedName types.NamespacedName) *api.MetalStackCluster {
	metalCluster := &api.MetalStackCluster{}
	if err := k8sClient.Get(ctx, namespacedName, metalCluster); err != nil {
		return nil
	}
	if metalCluster == nil {
		logger.Info(fmt.Sprintf("MetalStackCluster %s is not ready yet", namespacedName.Name))
		return nil
	}
	if metalCluster.Spec.PrivateNetworkID == nil {
		logger.Info("Private network isn't allocated yet")
		return nil
	}

	return metalCluster
}

func getKubeconfig(
	ctx context.Context,
	k8sClient client.Client,
	metalCluster *api.MetalStackCluster,
) (kubeconfig []byte, err error) {
	secret := &core.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: metalCluster.Namespace,
		Name:      fmt.Sprintf(kubeconfigSecretNameTemplate, metalCluster.Name),
	}
	if err := k8sClient.Get(ctx, namespacedName, secret); err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	kubeconfig, ok := secret.Data["value"]
	if !ok {
		return nil, fmt.Errorf("No key 'value' in kubeconfig secret")
	}

	return
}
