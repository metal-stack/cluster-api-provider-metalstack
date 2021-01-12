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

import "sigs.k8s.io/cluster-api/controllers/noderefutil"

type Firewall struct {
	// +optional
	DefaultNetworkID *string `json:"defaultNetworkID,omitempty"`

	// +optional
	Image *string `json:"image,omitempty"`

	// ProviderID sepecifies ID of machine on which firewall should be deployed
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// +optional
	Size *string `json:"size,omitempty"`

	// +optional
	SSHKeys []string `json:"sshKeys,omitempty"`
}

func (spec *Firewall) ParsedProviderID() (string, error) {
	unparsed := spec.ProviderID
	if unparsed == nil {
		return "", ProviderIDNotSet
	}
	parsed, err := noderefutil.NewProviderID(*unparsed)
	if err != nil {
		return "", err
	}
	return parsed.ID(), nil
}
