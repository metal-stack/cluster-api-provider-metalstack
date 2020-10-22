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
	"reflect"
	"strings"

	gmck "github.com/golang/mock/gomock"
	"github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	infra "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	"github.com/metal-stack/cluster-api-provider-metalstack/controllers/mocks"
	metalgo "github.com/metal-stack/metal-go"
	. "github.com/onsi/ginkgo"

	// . "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe(typeOf(MetalStackClusterReconciler{}), func() {
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
	Describe("deleteMachine", func() {
		It("should succeed", func() {
			mClient.EXPECT().MachineDelete(gmck.Any()).Return(nil, nil)
			r := newTestMachineReconciler(mClient)
			result, err := r.deleteMachine(context.TODO(), r.Log, newTestResource())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})
	// Describe("getRawMachineOrCreate", func() {
	// 	It("should", func() {
	// 		mClient.EXPECT().MachineCreate(gmck.Any()).Return(
	// 			&metalgo.MachineCreateResponse{
					
	// 			}
	// 		)
	// 	})
	// })
})

func newTestMachine() *infra.MetalStackMachine {
	machine := new(infra.MetalStackMachine)

	// Set corresponding fields.
	for _, s := range []string{
		"Spec.ProviderID",
	} {
		parsed := strings.Split(s, ".")
		last := parsed[len(parsed)-1]
		newStr := "test-" + last
		switch len(parsed) {
		case 1: // Name
			reflect.ValueOf(machine).Elem().FieldByName(last).Set(reflect.ValueOf(newStr))
		case 2: // fields in Spec
			if last == "ProviderID" {
				newStr = "metalstack://" + newStr
			}
			reflect.ValueOf(&machine.Spec).Elem().FieldByName(last).Set(reflect.ValueOf(&newStr))
		default:
			return new(infra.MetalStackMachine)
		}
	}
	return machine
}
func newTestMachineReconciler(mClient MetalStackClient) *MetalStackMachineReconciler {
	return &MetalStackMachineReconciler{
		Client:           fake.NewFakeClientWithScheme(newAndReadyScheme()),
		Log:              zap.New(zap.UseDevMode(true)),
		MetalStackClient: mClient,
	}
}
func newTestResource() *resource {
	return &resource{
		metalCluster: newTestCluster(),
		metalMachine: newTestMachine(),
	}
}