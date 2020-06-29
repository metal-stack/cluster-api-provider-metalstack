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

package metal

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	infrastructurev1alpha3 "github.com/metal-stack/cluster-api-provider-metal/api/v1alpha3"
	"github.com/metal-stack/cluster-api-provider-metal/pkg/cloud/metal/scope"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

const (
	apiTokenVarName = "METAL_API_TOKEN"
	apiKeyVarName   = "METAL_API_KEY"
	apiURLVarName   = "METAL_API_URL"
	clientName      = "CAPP-v1alpha3"
)

type MetalClient struct {
	*metalgo.Driver
}

// NewClient creates a new Client for the given Metal credentials
func NewClient(metalAPIURL, metalAPIToken, metalAPIKey string) *MetalClient {
	token := strings.TrimSpace(metalAPIKey)

	if token != "" {
		driver, err := metalgo.NewDriver(metalAPIURL, metalAPIToken, metalAPIKey)
		if err != nil {
			return nil
		}
		return &MetalClient{driver}
	}

	return nil
}

func GetClient() (*MetalClient, error) {
	url := os.Getenv(apiURLVarName)
	token := os.Getenv(apiTokenVarName)
	key := os.Getenv(apiKeyVarName)
	if token == "" {
		return nil, fmt.Errorf("env var %s is required", apiTokenVarName)
	}
	if url == "" {
		return nil, fmt.Errorf("env var %s is required", apiURLVarName)
	}
	return NewClient(url, token, key), nil
}

func (c *MetalClient) GetDevice(deviceID string) (*metalgo.MachineGetResponse, error) {
	dev, err := c.MachineGet(deviceID)
	return dev, err
}

func (c *MetalClient) NewDevice(hostname, project string, machineScope *scope.MachineScope, extraTags []string) (*metalgo.MachineCreateResponse, error) {
	userDataRaw, err := machineScope.GetRawBootstrapData()
	if err != nil {
		return nil, errors.Wrap(err, "impossible to retrieve bootstrap data from secret")
	}
	userData := string(userDataRaw)
	tags := append(machineScope.MetalMachine.Spec.Tags, extraTags...)
	if machineScope.IsControlPlane() {
		// control plane machines should get the API key injected
		tmpl, err := template.New("control-plane-user-data").Parse(userData)
		if err != nil {
			return nil, fmt.Errorf("error parsing control-plane userdata template: %v", err)
		}
		stringWriter := &strings.Builder{}
		apiKeyStruct := map[string]interface{}{
			"apiKey": os.Getenv(apiKeyVarName),
		}
		if err := tmpl.Execute(stringWriter, apiKeyStruct); err != nil {
			return nil, fmt.Errorf("error executing control-plane userdata template: %v", err)
		}
		userData = stringWriter.String()
		tags = append(tags, infrastructurev1alpha3.MasterTag)
	} else {
		tags = append(tags, infrastructurev1alpha3.WorkerTag)
	}
	serverCreateOpts := &metalgo.MachineCreateRequest{
		Hostname: hostname,

		Project:   project,
		Partition: machineScope.MetalMachine.Spec.Partition,
		Image:     machineScope.MetalMachine.Spec.Image,
		Tags:      tags,
		UserData:  userData,
	}

	dev, err := c.MachineCreate(serverCreateOpts)
	return dev, err
}

func (c *MetalClient) GetDeviceAddresses(machine *metalgo.MachineGetResponse) ([]corev1.NodeAddress, error) {
	addrs := make([]corev1.NodeAddress, 0)
	for _, network := range machine.Machine.Allocation.Networks {
		addrType := corev1.NodeInternalIP
		if !*network.Private {
			addrType = corev1.NodeExternalIP
		}
		a := corev1.NodeAddress{
			Type:    addrType,
			Address: network.Ips[0],
		}
		addrs = append(addrs, a)
	}
	return addrs, nil
}

func (c *MetalClient) GetDeviceByTags(project string, tags []string) (*metalgo.MachineListResponse, error) {
	mfr := metalgo.MachineFindRequest{AllocationProject: &project, Tags: tags}
	machines, err := c.MachineFind(&mfr)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving devices: %v", err)
	}

	return machines, nil
}
