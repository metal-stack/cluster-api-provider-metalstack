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

// MetalStackClient is the interface of the client for the interaction with `metal-API`
// On the next line, there's no space between `//` and `go`. It must be `//go:generate`.
//go:generate mockgen -destination=mocks/mock_metalstackclient.go -package=mocks . MetalStackClient
type MetalStackClient interface {
	FirewallCreate(fcr *metalgo.FirewallCreateRequest) (*metalgo.FirewallCreateResponse, error)
	IPAllocate(iar *metalgo.IPAllocateRequest) (*metalgo.IPDetailResponse, error)
	IPList() (*metalgo.IPListResponse, error)
	IPFree(id string) (*metalgo.IPDetailResponse, error)
	MachineCreate(mcr *metalgo.MachineCreateRequest) (*metalgo.MachineCreateResponse, error)
	MachineDelete(machineID string) (*metalgo.MachineDeleteResponse, error)
	MachineFind(mfr *metalgo.MachineFindRequest) (*metalgo.MachineListResponse, error)
	MachineGet(id string) (*metalgo.MachineGetResponse, error)
	NetworkAllocate(ncr *metalgo.NetworkAllocateRequest) (*metalgo.NetworkDetailResponse, error)
	NetworkFree(id string) (*metalgo.NetworkDetailResponse, error)
}
