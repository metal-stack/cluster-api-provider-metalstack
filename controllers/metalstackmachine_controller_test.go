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
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/metal-stack/cluster-api-provider-metalstack/controllers/mocks"
	metalgo "github.com/metal-stack/metal-go"
	metalmodels "github.com/metal-stack/metal-go/api/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconcile MetalStackMachine", func() {

	type MetalStackMachineTestCase struct {
		Objects  []runtime.Object
		Requeue  bool
		Error    bool
		MockFunc func()
	}

	ctrl := gomock.NewController(GinkgoT())
	metalClient := mocks.NewMockMetalStackClient(ctrl)
	metalStackMachineTestFunc := func(tc MetalStackMachineTestCase) {
		r := newTestMetalMachineReconciler(metalClient, tc.Objects)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      metalStackMachineName,
				Namespace: namespaceName,
			},
		}

		if tc.MockFunc != nil {
			tc.MockFunc()
		}

		res, err := r.Reconcile(req)
		if tc.Error {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(res.Requeue).To(Equal(tc.Requeue))
	}

	DescribeTable("Create Machine", metalStackMachineTestFunc,
		Entry("Should be no error when metal-stack machine not found", MetalStackMachineTestCase{}),
		Entry("Should fail if unable to get Owner Machine", MetalStackMachineTestCase{
			Objects: []runtime.Object{newMetalStackMachine(newMachineOwnerRef(), nil, false)},
			Error:   true,
		}),
		Entry("Should fail if unable to get cluster", MetalStackMachineTestCase{
			Objects: []runtime.Object{newMachine(), newMetalStackMachine(newMachineOwnerRef(), nil, false)},
			Error:   true,
		}),
		Entry("Should return if resources not ready", MetalStackMachineTestCase{
			Objects: []runtime.Object{
				newCluster(false, false),
				newMetalStackCluster(newClusterOwnerRef(), pointer.StringPtr("privateNetworkID"), false, false),
				newMachine(),
				newMetalStackMachine(newMachineOwnerRef(), nil, false)},
		}),
		Entry("Should requeue if Control Plane IP not allocated", MetalStackMachineTestCase{
			Objects: []runtime.Object{
				newCluster(false, true),
				newMetalStackCluster(newClusterOwnerRef(), pointer.StringPtr("privateNetworkID"), false, false),
				newMachine(),
				newMetalStackMachine(newMachineOwnerRef(), nil, false)},
			Requeue: true,
		}),
		Entry("Should fail if MachineCreate failed", MetalStackMachineTestCase{
			Objects: []runtime.Object{
				newCluster(false, true),
				newMetalStackCluster(newClusterOwnerRef(), pointer.StringPtr("privateNetworkID"), true, false),
				newMachine(),
				newMetalStackMachine(newMachineOwnerRef(), nil, false)},
			Error: true,
			MockFunc: func() {
				metalClient.EXPECT().MachineCreate(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
		}),
		Entry("Should requeue if Node not ready", MetalStackMachineTestCase{
			Objects: []runtime.Object{
				newCluster(false, true),
				newMetalStackCluster(newClusterOwnerRef(), pointer.StringPtr("privateNetworkID"), true, false),
				newMachine(),
				newMetalStackMachine(newMachineOwnerRef(), pointer.StringPtr(nodeID), false)},
			Requeue: true,
			MockFunc: func() {
				metalClient.EXPECT().MachineGet(gomock.Any()).Return(
					&metalgo.MachineGetResponse{
						Machine: &metalmodels.V1MachineResponse{
							Allocation: &metalmodels.V1MachineAllocation{},
						},
					}, nil)
			},
		}),
		Entry("Should succeed", MetalStackMachineTestCase{
			Objects: []runtime.Object{
				newNode(),
				newCluster(false, true),
				newMetalStackCluster(newClusterOwnerRef(), pointer.StringPtr("privateNetworkID"), true, false),
				newMachine(),
				newMetalStackMachine(newMachineOwnerRef(), pointer.StringPtr(nodeID), false)},
			MockFunc: func() {
				metalClient.EXPECT().MachineGet(gomock.Any()).Return(
					&metalgo.MachineGetResponse{
						Machine: &metalmodels.V1MachineResponse{
							Allocation: &metalmodels.V1MachineAllocation{},
						},
					}, nil)
			},
		}),
	)

	DescribeTable("Delete Machine", metalStackMachineTestFunc,
		Entry("Should fail if ProviderID not set", MetalStackMachineTestCase{
			Objects: []runtime.Object{
				newCluster(false, true),
				newMetalStackCluster(newClusterOwnerRef(), pointer.StringPtr("privateNetworkID"), false, false),
				newMachine(),
				newMetalStackMachine(newMachineOwnerRef(), nil, true)},
			Error: true,
		}),
		Entry("Should fail if MachineFind failed", MetalStackMachineTestCase{
			Objects: []runtime.Object{
				newCluster(false, true),
				newMetalStackCluster(newClusterOwnerRef(), pointer.StringPtr("privateNetworkID"), false, false),
				newMachine(),
				newMetalStackMachine(newMachineOwnerRef(), pointer.StringPtr(nodeID), true)},
			Error: true,
			MockFunc: func() {
				metalClient.EXPECT().MachineFind(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
		}),
		Entry("Should fail if MachineDelete failed", MetalStackMachineTestCase{
			Objects: []runtime.Object{
				newCluster(false, true),
				newMetalStackCluster(newClusterOwnerRef(), pointer.StringPtr("privateNetworkID"), false, false),
				newMachine(),
				newMetalStackMachine(newMachineOwnerRef(), pointer.StringPtr(nodeID), true)},
			Error: true,
			MockFunc: func() {
				metalClient.EXPECT().MachineFind(gomock.Any()).Return(
					&metalgo.MachineListResponse{
						Machines: []*metalmodels.V1MachineResponse{nil},
					}, nil)
				metalClient.EXPECT().MachineDelete(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
		}),
		Entry("Should succeed", MetalStackMachineTestCase{
			Objects: []runtime.Object{
				newCluster(false, true),
				newMetalStackCluster(newClusterOwnerRef(), pointer.StringPtr("privateNetworkID"), false, false),
				newMachine(),
				newMetalStackMachine(newMachineOwnerRef(), pointer.StringPtr(nodeID), true)},
			MockFunc: func() {
				metalClient.EXPECT().MachineFind(gomock.Any()).Return(
					&metalgo.MachineListResponse{}, nil)
			},
		}),
	)
})
