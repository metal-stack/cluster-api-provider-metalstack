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
	// Set up gomock Controller for each test case.
	gmckController := new(gmck.Controller)

	// mock of MetalStackClient
	mClient := new(mocks.MockMetalStackClient)
	BeforeEach(func() {
		gmckController = gmck.NewController(GinkgoT())
		mClient = mocks.NewMockMetalStackClient(gmckController)
	})
	AfterEach(func() {
		gmckController.Finish()
	})
	forwardingErr := "should forward the returned error from the lower-level API"
	theErr := errors.New("this error going to be returned by the tested func")
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
	Describe("controlPlaneIP", func() {
		It(forwardingErr, func() {
			mClient.EXPECT().MachineFind(gmck.Any()).Return(nil, theErr)
			ip, err := newTestClusterReconciler(mClient).controlPlaneIP(newTestCluster())
			Expect(ip).To(Equal(""))
			Expect(err).To(Equal(theErr))
		})
		It(fmt.Sprintf("should return `%v`", typeOf(errMachineNotFound{})), func() {
			mClient.EXPECT().MachineFind(gmck.Any()).Return(nil, nil)
			cluster := newTestCluster()
			ip, err := newTestClusterReconciler(mClient).controlPlaneIP(cluster)
			Expect(ip).To(Equal(""))
			Expect(err).To(Equal(newErrMachineNotFound(cluster.Spec.ProjectID, cluster.ControlPlaneTags())))
		})
		DescribeTable(
			typeOf(errIPNotAllocated{}),
			func(resp *models.V1MachineResponse) {
				mClient.EXPECT().MachineFind(gmck.Any()).Return(
					&metalgo.MachineListResponse{
						Machines: []*models.V1MachineResponse{
							resp,
						},
					},
					nil,
				)
				_, err := newTestClusterReconciler(mClient).controlPlaneIP(newTestCluster())
				Expect(typeOf(err)).To(Equal(typeOf(&errIPNotAllocated{})))
			},
			toEntriesForErrIPNotAllocated()...,
		)
		It("should return control-plane-IP", func() {
			expectedIP := "127.0.0.1"
			mClient.EXPECT().MachineFind(gmck.Any()).Return(
				&metalgo.MachineListResponse{
					Machines: []*models.V1MachineResponse{
						{
							Allocation: &models.V1MachineAllocation{
								Networks: []*models.V1MachineNetwork{
									{
										Ips: []string{expectedIP},
									},
								},
							},
						},
					},
				},
				nil,
			)
			ip, err := newTestClusterReconciler(mClient).controlPlaneIP(newTestCluster())
			Expect(err).ToNot(HaveOccurred())
			Expect(ip).To(Equal(expectedIP))
		})
	})
	Describe("createFirewall", func() {
		DescribeTable(
			typeOf(errSpecNotSet{}),
			func(s string) {
				cluster := newTestCluster()

				// Unset a field.
				if parsed := strings.Split(s, "."); len(parsed) == 2 {
					v := reflect.ValueOf(&cluster.Spec.Firewall).Elem().FieldByName(strings.Split(s, ".")[1])
					v.Set(reflect.Zero(v.Type()))
				}

				// Run the target func.
				err := newTestClusterReconciler(mClient).createFirewall(zap.New(zap.UseDevMode(true)), cluster)
				Expect(err).To(Equal(newErrSpecNotSet(s)))
			},
			toEntriesForErrSpecNotSet(
				"Firewall.DefaultNetworkID",
				"Firewall.Image",
				"Firewall.Size",
			)...,
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
func toEntriesForErrSpecNotSet(ss ...string) []TableEntry {
	entries := []TableEntry{}
	for _, s := range ss {
		entries = append(entries, Entry(toTestDescr(newErrSpecNotSet(s)), s))
	}
	return entries
}
func toEntriesForErrIPNotAllocated() []TableEntry {
	resps := []*models.V1MachineResponse{
		{},
		{
			Allocation: &models.V1MachineAllocation{
				Networks: []*models.V1MachineNetwork{},
			},
		},
		{
			Allocation: &models.V1MachineAllocation{
				Networks: []*models.V1MachineNetwork{
					{
						Ips: []string{},
					},
				},
			},
		},
		{
			Allocation: &models.V1MachineAllocation{
				Networks: []*models.V1MachineNetwork{
					{
						Ips: []string{""},
					},
				},
			},
		},
	}
	entries := []TableEntry{}
	for _, resp := range resps {
		entries = append(entries, Entry("should return "+typeOf(errIPNotAllocated{}), resp))
	}
	return entries
}
func toTestDescr(e *errSpecNotSet) string {
	return fmt.Sprintf("should contain the message `%v`", e)
}
func typeOf(i interface{}) string {
	return reflect.TypeOf(i).String()
}
func unsetPointeeField(i interface{}, name string) {
	v := reflect.ValueOf(i).Elem().FieldByName(name)
	v.Set(reflect.Zero(v.Type()))
}
