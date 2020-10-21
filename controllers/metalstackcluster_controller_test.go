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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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

var _ = Describe("MetalStackClusterReconciler", func() {
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
	forwardingErr := "should forward the returned error from the lower-level API"
	theErr := errors.New("this error going to be returned by the tested func")
	Describe("allocateNetwork", func() {
		DescribeTable(
			reflect.TypeOf(errSpecNotSet{}).String(),
			func(s string) {
				cluster := newTestCluster()
				unsetPointeeField(&cluster.Spec, s)

				// Run the target func.
				_, err := newTestReconciler(mClient).allocateNetwork(cluster)
				Expect(err).To(Equal(newErrSpecNotSet(s)))
			},
			func() []TableEntry {
				entries := []TableEntry{}
				for _, s := range []string{"Partition", "ProjectID"} {
					entries = append(entries, Entry(toTestDescr(newErrSpecNotSet(s)), s))
				}
				return entries
			}()...,
		)
		It(forwardingErr, func() {
			// Set the returned error.
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
	Describe("controlPlaneIP", func() {
		{
			field := "ProjectID"
			It(toTestDescr(newErrSpecNotSet(field)), func() {
				cluster := newTestCluster()
				unsetPointeeField(&cluster.Spec, field)
				_, err := newTestReconciler(mClient).controlPlaneIP(cluster)
				Expect(err).To(Equal(newErrSpecNotSet(field)))
			})
		}
		It(forwardingErr, func() {
			mClient.EXPECT().MachineFind(gmck.Any()).Return(nil, theErr)
			ip, err := newTestReconciler(mClient).controlPlaneIP(newTestCluster())
			Expect(ip).To(Equal(""))
			Expect(err).To(Equal(theErr))
		})
		It(fmt.Sprintf("should return `%v`", reflect.TypeOf(errMachineNotFound{}).String()), func() {
			mClient.EXPECT().MachineFind(gmck.Any()).Return(nil, nil)
			cluster := newTestCluster()
			ip, err := newTestReconciler(mClient).controlPlaneIP(cluster)
			Expect(ip).To(Equal(""))
			Expect(err).To(Equal(newErrMachineNotFound(*cluster.Spec.ProjectID, cluster.ControlPlaneTags())))
		})
	})
	Describe("createFirewall", func() {
		DescribeTable(
			reflect.TypeOf(errSpecNotSet{}).String(),
			func(s string) {
				cluster := newTestCluster()

				// Unset a field.
				v := reflect.Value{}
				if parsed := strings.Split(s, "."); len(parsed) == 1 {
					v = reflect.ValueOf(&cluster.Spec).Elem().FieldByName(s)
				} else {
					v = reflect.ValueOf(cluster.Spec.Firewall).Elem().FieldByName(parsed[len(parsed)-1])
				}
				v.Set(reflect.Zero(v.Type()))

				// Run the target func.
				err := newTestReconciler(mClient).createFirewall(cluster)
				Expect(err).To(Equal(newErrSpecNotSet(s)))
			},
			func() []TableEntry {
				entries := []TableEntry{}
				for _, s := range []string{
					"Firewall.DefaultNetworkID",
					"Firewall.Image",
					"Firewall.Size",
					"Partition",
					"PrivateNetworkID",
					"ProjectID"} {
					entries = append(entries, Entry(toTestDescr(newErrSpecNotSet(s)), s))
				}
				return entries
			}()...,
		)
		It(forwardingErr, func() {
			// Set the returned error.
			mClient.EXPECT().FirewallCreate(gmck.Any()).Return(nil, theErr)
			r := newTestReconciler(mClient)

			// Run the target func.
			err := r.createFirewall(newTestCluster())
			Expect(err).To(Equal(theErr))
		})
	})
})

// todo: duplicate
func newAndReadyScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterapi.AddToScheme(scheme)
	_ = infra.AddToScheme(scheme)
	return scheme
}
func toTestDescr(e *errSpecNotSet) string {
	return fmt.Sprintf("should contain the message `%v`", e)
}
func newTestCluster() *infra.MetalStackCluster {
	cluster := &infra.MetalStackCluster{}

	// fields to set
	ss := []string{
		"Name",
		"Spec.Firewall.DefaultNetworkID",
		"Spec.Firewall.Image",
		"Spec.Firewall.Size",
		"Spec.Partition",
		"Spec.PrivateNetworkID",
		"Spec.ProjectID",
	}

	// Set corresponding fields.
	func(ss []string) {
		for _, s := range ss {
			parsed := strings.Split(s, ".")
			last := parsed[len(parsed)-1]
			newStr := "test-" + last
			switch len(parsed) {
			case 1: // Name
				reflect.ValueOf(cluster).Elem().FieldByName(last).Set(reflect.ValueOf(newStr))
			case 2: // fields in Spec
				reflect.ValueOf(&cluster.Spec).Elem().FieldByName(last).Set(reflect.ValueOf(&newStr))
			case 3: // fields in Firewall
				if cluster.Spec.Firewall == nil {
					cluster.Spec.Firewall = &infra.Firewall{}
				}
				reflect.ValueOf(cluster.Spec.Firewall).Elem().FieldByName(last).Set(reflect.ValueOf(&newStr))
			default:
				return
			}
		}
	}(ss)
	return cluster
}
func newTestReconciler(mClient MetalStackClient) *MetalStackClusterReconciler {
	return &MetalStackClusterReconciler{
		Client:           fake.NewFakeClientWithScheme(newAndReadyScheme()),
		Log:              zap.New(zap.UseDevMode(true)),
		MetalStackClient: mClient,
	}
}
func unsetPointeeField(i interface{}, name string) {
	v := reflect.ValueOf(i).Elem().FieldByName(name)
	v.Set(reflect.Zero(v.Type()))
}
