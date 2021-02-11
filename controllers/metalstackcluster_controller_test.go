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

var _ = Describe("Reconcile MetalStackCluster", func() {

	type MetalStackClusterTestCase struct {
		Objects  []runtime.Object
		Requeue  bool
		Error    bool
		MockFunc func()
	}

	ctrl := gomock.NewController(GinkgoT())
	metalClient := mocks.NewMockMetalStackClient(ctrl)
	metalStackClusterTestFunc := func(tc MetalStackClusterTestCase) {
		r := newTestMetalClusterReconciler(metalClient, tc.Objects)
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      metalStackClusterName,
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

	DescribeTable("Create Cluster", metalStackClusterTestFunc,
		Entry("Should be no error when metal-stack cluster not found", MetalStackClusterTestCase{}),
		Entry("Should requeue if Owner Cluster not set", MetalStackClusterTestCase{
			Objects: []runtime.Object{newMetalStackCluster(nil, nil, false)},
			Requeue: true,
		}),
		Entry("Should fail if unable to get Owner Cluster", MetalStackClusterTestCase{
			Objects: []runtime.Object{newMetalStackCluster(newOwnerRef(), nil, false)},
			Error:   true,
		}),
		Entry("Should requeue if paused", MetalStackClusterTestCase{
			Objects: []runtime.Object{
				newCluster(true),
				newMetalStackCluster(newOwnerRef(), nil, false),
			},
			Requeue: true,
		}),
		Entry("Should requeue if network allocation failed", MetalStackClusterTestCase{
			Objects: []runtime.Object{
				newCluster(false),
				newMetalStackCluster(newOwnerRef(), nil, false),
			},
			Requeue: true,
			MockFunc: func() {
				metalClient.EXPECT().NetworkAllocate(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
		}),
		Entry("Should requeue if Control Plane IP allocation failed", MetalStackClusterTestCase{
			Objects: []runtime.Object{
				newCluster(false),
				newMetalStackCluster(newOwnerRef(), pointer.StringPtr("privateNetworkID"), false),
			},
			Requeue: true,
			MockFunc: func() {
				metalClient.EXPECT().IPAllocate(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
		}),
		Entry("Should succeed", MetalStackClusterTestCase{
			Objects: []runtime.Object{
				newCluster(false),
				newMetalStackCluster(newOwnerRef(), pointer.StringPtr("privateNetworkID"), false),
			},
			MockFunc: func() {
				metalClient.EXPECT().IPAllocate(gomock.Any()).Return(nil, nil)
			},
		}),
	)

	DescribeTable("Delete Cluster", metalStackClusterTestFunc,
		Entry("Should requeue if not all IPs are freed", MetalStackClusterTestCase{
			Objects: []runtime.Object{
				newCluster(false),
				newMetalStackCluster(newOwnerRef(), pointer.StringPtr("privateNetworkID"), true),
			},
			Requeue: true,
			MockFunc: func() {
				metalClient.EXPECT().IPList().Return(&metalgo.IPListResponse{IPs: []*metalmodels.V1IPResponse{nil}}, nil)
			},
		}),
		Entry("Should fail if NetworkFree returned error", MetalStackClusterTestCase{
			Objects: []runtime.Object{
				newCluster(false),
				newMetalStackCluster(newOwnerRef(), pointer.StringPtr("privateNetworkID"), true),
			},
			Error: true,
			MockFunc: func() {
				metalClient.EXPECT().IPList().Return(&metalgo.IPListResponse{IPs: nil}, nil)
				metalClient.EXPECT().NetworkFree(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
		}),
		Entry("Should succeed", MetalStackClusterTestCase{
			Objects: []runtime.Object{
				newCluster(false),
				newMetalStackCluster(newOwnerRef(), pointer.StringPtr("privateNetworkID"), true),
			},
			MockFunc: func() {
				metalClient.EXPECT().IPList().Return(&metalgo.IPListResponse{IPs: nil}, nil)
				metalClient.EXPECT().NetworkFree(gomock.Any()).Return(nil, nil)
			},
		}),
	)
})
