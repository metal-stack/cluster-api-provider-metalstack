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

package v1alpha4

// MetalStackResourceStatus describes the status of a MetalStack resource.
type MetalStackResourceStatus string

var (
	// MetalStackResourceStatusNew represents a MetalStack resource requested.
	// The MetalStack infrastructure uses a queue to avoid any abuse. So a resource
	// does not get created straight away but it can wait for a bit in a queue.
	MetalStackResourceStatusNew = MetalStackResourceStatus("new")
	// MetalStackResourceStatusQueued represents a machine waiting for his turn to be provisioned.
	// Time in queue depends on how many creation requests you already issued, or
	// from how many resources waiting to be deleted we have for you.
	MetalStackResourceStatusQueued = MetalStackResourceStatus("queued")
	// MetalStackResourceStatusProvisioning represents a resource that got dequeued
	// and it is actively processed by a worker.
	MetalStackResourceStatusProvisioning = MetalStackResourceStatus("provisioning")
	// MetalStackResourceStatusRunning represents a MetalStack resource already provisioned and in a active state.
	MetalStackResourceStatusRunning = MetalStackResourceStatus("active")
	// MetalStackResourceStatusErrored represents a MetalStack resource in a errored state.
	MetalStackResourceStatusErrored = MetalStackResourceStatus("errored")
	// MetalStackResourceStatusOff represents a MetalStack resource in off state.
	MetalStackResourceStatusOff = MetalStackResourceStatus("off")
)

// MetalStackMachineTemplateResource describes the data needed to create am MetalStackMachine from a template
type MetalStackMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec MetalStackMachineSpec `json:"spec"`
}
