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

	"github.com/golang/mock/gomock"
	metalgo "github.com/metal-stack/metal-go"
	metalmodels "github.com/metal-stack/metal-go/api/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/metal-stack/cluster-api-provider-metalstack/controllers/mocks"
)

var _ = Describe("Reconcile MetalStackFirewall", func() {

	type MetalStackFirewallTestCase struct {
		Objects  []runtime.Object
		Requeue  bool
		Error    bool
		MockFunc func()
	}

	ctrl := gomock.NewController(GinkgoT())
	metalClient := mocks.NewMockMetalStackClient(ctrl)
	metalStackMachineTestFunc := func(tc MetalStackFirewallTestCase) {
		r := newTestMetalFirewallReconciler(metalClient, tc.Objects)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      metalStackFirewallName,
				Namespace: namespaceName,
			},
		}

		if tc.MockFunc != nil {
			tc.MockFunc()
		}

		res, err := r.Reconcile(context.TODO(), req)
		if tc.Error {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(res.Requeue).To(Equal(tc.Requeue))
	}

	DescribeTable("Create Firewall", metalStackMachineTestFunc,
		Entry("Should be no error when metal-stack firewall not found", MetalStackFirewallTestCase{}),
		Entry("Should requeue if MetalStackCluster not available", MetalStackFirewallTestCase{
			Objects: []runtime.Object{newMetalStackFirewall(nil, false)},
			Requeue: true,
		}),
		Entry("Should fail if FirewallCreate failed", MetalStackFirewallTestCase{
			Objects: []runtime.Object{
				newSecret(fmt.Sprintf(kubeconfigSecretNameTemplate, metalStackClusterName)),
				newMetalStackCluster(nil, pointer.StringPtr("privateNetworkID"), false, false),
				newMetalStackFirewall(nil, false),
			},
			Error: true,
			MockFunc: func() {
				metalClient.EXPECT().FirewallCreate(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
		}),
		Entry("Should requeue if allocation not succeeded", MetalStackFirewallTestCase{
			Objects: []runtime.Object{
				newSecret(fmt.Sprintf(kubeconfigSecretNameTemplate, metalStackClusterName)),
				newMetalStackCluster(nil, pointer.StringPtr("privateNetworkID"), false, false),
				newMetalStackFirewall(pointer.StringPtr(nodeID), false),
			},
			Requeue: true,
			MockFunc: func() {
				metalClient.EXPECT().MachineGet(gomock.Any()).Return(
					&metalgo.MachineGetResponse{
						Machine: &metalmodels.V1MachineResponse{
							Allocation: &metalmodels.V1MachineAllocation{},
						},
					}, nil)
				metalClient.EXPECT().FirewallGet(gomock.Any()).Return(
					&metalgo.FirewallGetResponse{
						Firewall: &metalmodels.V1FirewallResponse{
							Allocation: &metalmodels.V1MachineAllocation{
								Succeeded: pointer.BoolPtr(false),
							},
						},
					}, nil)
			},
		}),
		Entry("Should succeed", MetalStackFirewallTestCase{
			Objects: []runtime.Object{
				newSecret(fmt.Sprintf(kubeconfigSecretNameTemplate, metalStackClusterName)),
				newMetalStackCluster(nil, pointer.StringPtr("privateNetworkID"), false, false),
				newMetalStackFirewall(pointer.StringPtr(nodeID), false),
			},
			MockFunc: func() {
				metalClient.EXPECT().MachineGet(gomock.Any()).Return(
					&metalgo.MachineGetResponse{
						Machine: &metalmodels.V1MachineResponse{
							Allocation: &metalmodels.V1MachineAllocation{},
						},
					}, nil)
				metalClient.EXPECT().FirewallGet(gomock.Any()).Return(
					&metalgo.FirewallGetResponse{
						Firewall: &metalmodels.V1FirewallResponse{
							Allocation: &metalmodels.V1MachineAllocation{
								Succeeded: pointer.BoolPtr(true),
							},
						},
					}, nil)
			},
		}),
	)

	DescribeTable("Delete Firewall", metalStackMachineTestFunc,
		Entry("Should fail if ProviderID not set", MetalStackFirewallTestCase{
			Objects: []runtime.Object{
				newSecret(fmt.Sprintf(kubeconfigSecretNameTemplate, metalStackClusterName)),
				newMetalStackCluster(nil, pointer.StringPtr("privateNetworkID"), false, false),
				newMetalStackFirewall(nil, true),
			},
			Error: true,
		}),
		Entry("Should fail if FirewallFind failed", MetalStackFirewallTestCase{
			Objects: []runtime.Object{
				newSecret(fmt.Sprintf(kubeconfigSecretNameTemplate, metalStackClusterName)),
				newMetalStackCluster(nil, pointer.StringPtr("privateNetworkID"), false, false),
				newMetalStackFirewall(pointer.StringPtr(nodeID), true),
			},
			Error: true,
			MockFunc: func() {
				metalClient.EXPECT().FirewallFind(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
		}),
		Entry("Should fail if MachineDelete failed", MetalStackFirewallTestCase{
			Objects: []runtime.Object{
				newSecret(fmt.Sprintf(kubeconfigSecretNameTemplate, metalStackClusterName)),
				newMetalStackCluster(nil, pointer.StringPtr("privateNetworkID"), false, false),
				newMetalStackFirewall(pointer.StringPtr(nodeID), true),
			},
			Error: true,
			MockFunc: func() {
				metalClient.EXPECT().FirewallFind(gomock.Any()).Return(
					&metalgo.FirewallListResponse{
						Firewalls: []*metalmodels.V1FirewallResponse{nil},
					}, nil)
				metalClient.EXPECT().MachineDelete(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
		}),
		Entry("Should succeed", MetalStackFirewallTestCase{
			Objects: []runtime.Object{
				newSecret(fmt.Sprintf(kubeconfigSecretNameTemplate, metalStackClusterName)),
				newMetalStackCluster(nil, pointer.StringPtr("privateNetworkID"), false, false),
				newMetalStackFirewall(pointer.StringPtr(nodeID), true),
			},
			MockFunc: func() {
				metalClient.EXPECT().FirewallFind(gomock.Any()).Return(
					&metalgo.FirewallListResponse{
						Firewalls: []*metalmodels.V1FirewallResponse{nil},
					}, nil)
				metalClient.EXPECT().MachineDelete(gomock.Any()).Return(nil, nil)
			},
		}),
	)

})
