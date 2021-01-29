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
	"reflect"
	"strings"

	gmck "github.com/golang/mock/gomock"
	infra "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	"github.com/metal-stack/cluster-api-provider-metalstack/controllers/mocks"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	apimachineryruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterapi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("MetalStackClusterReconciler", func() {
	theErr := errors.New("this error going to be returned by the tested func")
	forwardingErr := "should forward the returned error from the lower-level API"

	mClient := new(mocks.MockMetalStackClient)
	gmckController := new(gmck.Controller)

	BeforeEach(func() {
		gmckController = gmck.NewController(GinkgoT())
		mClient = mocks.NewMockMetalStackClient(gmckController)
	})

	AfterEach(func() {
		gmckController.Finish()
	})

	Describe("allocateNetwork", func() {
		It(forwardingErr, func() {
			// Set the returned error.
			mClient.EXPECT().NetworkAllocate(gmck.Any()).Return(nil, theErr)
			r := newTestClusterReconciler(mClient)

			// Run the target func.
			err := r.allocateNetwork(newTestCluster())
			Expect(err).To(Equal(theErr))
		})
		It("should return the project ID", func() {
			expectedID := "this ID going to be returned by the tested func"

			// Set the response from `metal-go` API.
			mClient.EXPECT().NetworkAllocate(gmck.Any()).Return(
				&metalgo.NetworkDetailResponse{
					Network: &models.V1NetworkResponse{
						ID: &expectedID,
					},
				},
				nil,
			)
			r := newTestClusterReconciler(mClient)
			testCluster := newTestCluster()
			err := r.allocateNetwork(testCluster)
			Expect(err).ToNot(HaveOccurred())
			Expect(testCluster.Spec.PrivateNetworkID).To(Equal(&expectedID))
		})
	})

	Describe("createFirewall", func() {
		DescribeTable(
			"Firewall spec not full",
			func(s string) {
				cluster := newTestCluster()

				// Unset a field.
				if parsed := strings.Split(s, "."); len(parsed) == 2 {
					v := reflect.ValueOf(&cluster.Spec.Firewall).Elem().FieldByName(strings.Split(s, ".")[1])
					v.Set(reflect.Zero(v.Type()))
				}

				// Run the target func.
				err := newTestClusterReconciler(mClient).createFirewall(zap.New(zap.UseDevMode(true)), cluster)
				Expect(err.Error()).To(Equal(s))
			},
			Entry("Firewall.DefaultNetworkID not set", "Firewall.DefaultNetworkID"),
			Entry("Firewall.Image not set", "Firewall.Image"),
			Entry("Firewall.Size not set", "Firewall.Size"),
		)
		It(forwardingErr, func() {
			// Set the returned error.
			mClient.EXPECT().FirewallCreate(gmck.Any()).Return(nil, theErr)
			err := newTestClusterReconciler(mClient).createFirewall(zap.New(zap.UseDevMode(true)), newTestCluster())
			Expect(err).To(Equal(theErr))
		})
	})
})

// todo: Remove the duplicated logic.
func newAndReadyScheme() *apimachineryruntime.Scheme {
	scheme := apimachineryruntime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterapi.AddToScheme(scheme)
	_ = infra.AddToScheme(scheme)
	return scheme
}

func newTestCluster() *infra.MetalStackCluster {
	cluster := new(infra.MetalStackCluster)

	// Set corresponding fields.
	for _, s := range []string{
		"Name",
		"Spec.Firewall.DefaultNetworkID",
		"Spec.Firewall.Image",
		"Spec.Firewall.Size",
		"Spec.Partition",
		"Spec.PrivateNetworkID",
		"Spec.ProjectID",
	} {
		parsed := strings.Split(s, ".")
		last := parsed[len(parsed)-1]
		newStr := "test-" + last
		switch len(parsed) {
		case 1: // Name
			reflect.ValueOf(cluster).Elem().FieldByName(last).Set(reflect.ValueOf(newStr))
		case 2: // fields in Spec
			if last == "PrivateNetworkID" {
				reflect.ValueOf(&cluster.Spec).Elem().FieldByName(last).Set(reflect.ValueOf(&newStr))
			} else {
				reflect.ValueOf(&cluster.Spec).Elem().FieldByName(last).Set(reflect.ValueOf(newStr))
			}
		case 3: // fields in Firewall
			reflect.ValueOf(&cluster.Spec.Firewall).Elem().FieldByName(last).Set(reflect.ValueOf(&newStr))
		default:
			return new(infra.MetalStackCluster)
		}
	}
	return cluster
}

func newTestClusterReconciler(mClient MetalStackClient) *MetalStackClusterReconciler {
	return &MetalStackClusterReconciler{
		Client:           fake.NewFakeClientWithScheme(newAndReadyScheme()),
		Log:              zap.New(zap.UseDevMode(true)),
		MetalStackClient: mClient,
	}
}
