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
	"time"

	metalgo "github.com/metal-stack/metal-go"
	ctrl "sigs.k8s.io/controller-runtime"
)

var requeue = ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}

func toNetworks(ss ...string) (networks []metalgo.MachineAllocationNetwork) {
	for _, s := range ss {
		networks = append(networks, metalgo.MachineAllocationNetwork{
			NetworkID:   s,
			Autoacquire: true,
		})
	}
	return
}
