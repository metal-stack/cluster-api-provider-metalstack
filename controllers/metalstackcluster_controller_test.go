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
	metalgo "github.com/metal-stack/metal-go"
)

type MockClient struct{}

func (cl *MockClient) FirewallCreate(fcr *metalgo.FirewallCreateRequest) (*metalgo.FirewallCreateResponse, error) {

}

func (cl *MockClient) MachineCreate(mcr *metalgo.MachineCreateRequest) (*metalgo.MachineCreateResponse, error) {

}

func (cl *MockClient) MachineDelete(machineID string) (*metalgo.MachineDeleteResponse, error) {

}

func (cl *MockClient) MachineFind(mfr *metalgo.MachineFindRequest) (*metalgo.MachineListResponse, error) {

}

func (cl *MockClient) MachineGet(id string) (*metalgo.MachineGetResponse, error) {

}

func (cl *MockClient) NetworkAllocate(ncr *metalgo.NetworkAllocateRequest) (*metalgo.NetworkDetailResponse, error) {

}
