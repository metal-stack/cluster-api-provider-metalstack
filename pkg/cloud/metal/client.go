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
	"github.com/metal-stack/metal-go/api/models"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var clientLog = ctrl.Log.WithName("client")

const (
	apiTokenVarName = "METAL_API_TOKEN"
	apiKeyVarName   = "METAL_API_KEY"
	apiURLVarName   = "METAL_API_URL"
	clientName      = "CAPP-v1alpha3"
)

type MetalClient struct {
	*metalgo.Driver
}

// newClient creates a new Client for the given Metal credentials
func newClient(metalAPIURL, metalAPIToken, metalAPIKey string) (*MetalClient, error) {
	driver, err := metalgo.NewDriver(metalAPIURL, metalAPIToken, metalAPIKey)
	return &MetalClient{driver}, err
}

func GetClient() (*MetalClient, error) {
	url := strings.TrimSpace(os.Getenv(apiURLVarName))
	token := strings.TrimSpace(os.Getenv(apiTokenVarName))
	key := strings.TrimSpace(os.Getenv(apiKeyVarName))
	if token == "" && key == "" {
		return nil, fmt.Errorf("env var %s or %s is required", apiTokenVarName, apiKeyVarName)
	}
	if url == "" {
		return nil, fmt.Errorf("env var %s is required", apiURLVarName)
	}
	return newClient(url, token, key)
}

func (c *MetalClient) GetMachine(machineID string) (*metalgo.MachineGetResponse, error) {
	return c.MachineGet(machineID)
}

func (c *MetalClient) NewMachine(hostname, project string, machineScope *scope.MachineScope, extraTags []string) (*metalgo.MachineCreateResponse, error) {
	privateNetwork := machineScope.MetalCluster.Spec.PrivateNetworkID
	if privateNetwork == "" {
		return nil, fmt.Errorf("no privatenetwork allocated yet")
	}
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
	networks := []metalgo.MachineAllocationNetwork{
		{NetworkID: privateNetwork, Autoacquire: true},
	}
	for _, additionalNetwork := range machineScope.MetalCluster.Spec.AdditionalNetworks {
		anw := metalgo.MachineAllocationNetwork{
			NetworkID:   additionalNetwork,
			Autoacquire: true,
		}
		networks = append(networks, anw)
	}

	machineCreateRequest := &metalgo.MachineCreateRequest{
		Name:     hostname,
		Hostname: hostname,

		Project:   project,
		Partition: machineScope.MetalMachine.Spec.Partition,
		Image:     machineScope.MetalMachine.Spec.Image,
		Size:      machineScope.MetalMachine.Spec.MachineType,
		Networks:  networks,
		Tags:      tags,
		UserData:  userData,
	}

	return c.MachineCreate(machineCreateRequest)
}

func (c *MetalClient) GetMachineAddresses(machine *models.V1MachineResponse) ([]corev1.NodeAddress, error) {
	addrs := make([]corev1.NodeAddress, 0)
	for _, network := range machine.Allocation.Networks {
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

func (c *MetalClient) GetMachineByTags(project string, tags []string) (*metalgo.MachineListResponse, error) {
	mfr := metalgo.MachineFindRequest{AllocationProject: &project, Tags: tags}
	if c == nil {
		return nil, fmt.Errorf("metal-go client is nil")
	}
	machines, err := c.MachineFind(&mfr)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving machines: %v", err)
	}

	return machines, nil
}

func (c *MetalClient) AllocatePrivateNetwork(name, project, partition string) (string, error) {
	ncr := metalgo.NetworkAllocateRequest{
		Name:        name,
		PartitionID: partition,
		ProjectID:   project,
	}
	nw, err := c.NetworkAllocate(&ncr)
	if err != nil {
		return "", err
	}
	if nw == nil {
		return "", fmt.Errorf("network allocation returned nil network")
	}
	return *nw.Network.ID, nil
}

func (c *MetalClient) FreePrivateNetwork(privateNetworkID string) error {
	// FIXME implement and use
	return fmt.Errorf("not implemented yet")
}
