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
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	gmck "github.com/golang/mock/gomock"
	infra "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	"github.com/metal-stack/cluster-api-provider-metalstack/controllers/mocks"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterapi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// todo: duplicate
func NewAndReadyScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterapi.AddToScheme(scheme)
	_ = infra.AddToScheme(scheme)
	return scheme
}

var _ = Describe("Test struct MetalStackClusterReconciler", func() {
	// Set up gomock Controller for each test case.
	gmckController := &gmck.Controller{}

	// mock of MetalStackClient
	mClient := &mocks.MockMetalStackClient{}
	BeforeEach(func() {
		gmckController = gmck.NewController(GinkgoT())
		mClient = mocks.NewMockMetalStackClient(gmckController)
	})
	AfterEach(func() {
		gmckController.Finish()
	})

	Context("Test func allocateNetwork", func() {
		When("Partition not set", func() {
			It("should return typed error `notSet`", func() {
				_, err := newTestReconciler(mClient).allocateNetwork(&infra.MetalStackCluster{})
				Expect(err).To(Equal(&notSet{"Partition not set"}))
			})
		})

		When("ProjectID not set", func() {
			It("should return typed error `notSet`", func() {
				s := "test-partition"
				_, err := newTestReconciler(mClient).allocateNetwork(&infra.MetalStackCluster{
					Spec: infra.MetalStackClusterSpec{
						Partition: &s,
					},
				})
				Expect(err).To(Equal(&notSet{"ProjectID not set"}))
			})
		})

		It("should forward the returned error from the lower-level API", func() {
			// Set the returned error.
			theErr := errors.New("this error going to be returned by the tested func")
			mClient.EXPECT().NetworkAllocate(gmck.Any()).Return(nil, theErr)
			r := newTestReconciler(mClient)

			// Run the target func.
			_, err := r.allocateNetwork(newTestCluster())
			Expect(err).To(Equal(theErr))
		})

		It("should return the project ID", func() {
			expectedID := "this ID going to be returned by the tested func"

			// Set the response from `metal-go` API.
			mClient.EXPECT().NetworkAllocate(gmck.Any()).Return(&metalgo.NetworkDetailResponse{
				Network: &models.V1NetworkResponse{
					ID: &expectedID,
				},
			}, nil)
			r := newTestReconciler(mClient)
			id, err := r.allocateNetwork(newTestCluster())
			Expect(err).ToNot(HaveOccurred())
			Expect(id).To(Equal(&expectedID))
		})
	})
})

func newTestCluster() *infra.MetalStackCluster {
	cluster := &infra.MetalStackCluster{}

	cluster.SetName("test-name")

	s := "test-partition"
	cluster.Spec.Partition = &s

	id := "test-project-ID"
	cluster.Spec.ProjectID = &id

	return cluster
}

func newTestReconciler(mClient MetalStackClient) *MetalStackClusterReconciler {
	return &MetalStackClusterReconciler{
		Client:           fake.NewFakeClientWithScheme(NewAndReadyScheme()),
		Log:              zap.New(zap.UseDevMode(true)),
		MetalStackClient: mClient,
	}
}
