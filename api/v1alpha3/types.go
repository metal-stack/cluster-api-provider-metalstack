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

package v1alpha3

// MetalResourceStatus describes the status of a Metal resource.
type MetalResourceStatus string

var (
	// MetalResourceStatusNew represents a Metal resource requested.
	// The Metal infrastructure uses a queue to avoid any abuse. So a resource
	// does not get created straight away but it can wait for a bit in a queue.
	MetalResourceStatusNew = MetalResourceStatus("new")
	// MetalResourceStatusQueued represents a machine waiting for his turn to be provisioned.
	// Time in queue depends on how many creation requests you already issued, or
	// from how many resources waiting to be deleted we have for you.
	MetalResourceStatusQueued = MetalResourceStatus("queued")
	// MetalResourceStatusProvisioning represents a resource that got dequeued
	// and it is actively processed by a worker.
	MetalResourceStatusProvisioning = MetalResourceStatus("provisioning")
	// MetalResourceStatusRunning represents a Metal resource already provisioned and in a active state.
	MetalResourceStatusRunning = MetalResourceStatus("active")
	// MetalResourceStatusErrored represents a Metal resource in a errored state.
	MetalResourceStatusErrored = MetalResourceStatus("errored")
	// MetalResourceStatusOff represents a Metal resource in off state.
	MetalResourceStatusOff = MetalResourceStatus("off")
)

// Tags defines a slice of tags.
type Tags []string

// MetalMachineTemplateResource describes the data needed to create am MetalMachine from a template
type MetalMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec MetalMachineSpec `json:"spec"`
}
