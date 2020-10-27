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
	"github.com/google/uuid"
	infra "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	"github.com/metal-stack/metal-lib/pkg/tag"
	cluster "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
)

type resource struct {
	cluster      *cluster.Cluster
	machine      *cluster.Machine
	metalCluster *infra.MetalStackCluster
	metalMachine *infra.MetalStackMachine
}

func newResource(
	cl *cluster.Cluster,
	m *cluster.Machine,
	metalCl *infra.MetalStackCluster,
	metalM *infra.MetalStackMachine,
) *resource {
	return &resource{
		cluster:      cl,
		machine:      m,
		metalCluster: metalCl,
		metalMachine: metalM,
	}
}
func (rsrc *resource) machineCreationTags() []string {
	tags := append([]string{
		"cluster-api-provider-metalstack:machine-uid=" + uuid.New().String(),
		tag.ClusterID + "=" + rsrc.metalCluster.Name,
	}, rsrc.metalMachine.Spec.Tags...)
	if util.IsControlPlaneMachine(rsrc.machine) {
		tags = append(tags, cluster.MachineControlPlaneLabelName+"=true")
	} else {
		tags = append(tags, cluster.MachineControlPlaneLabelName+"=false")
	}

	return tags
}
