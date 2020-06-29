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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrastructurev1alpha3 "github.com/metal-stack/cluster-api-provider-metal/api/v1alpha3"
	metal "github.com/metal-stack/cluster-api-provider-metal/pkg/cloud/metal"
	"github.com/metal-stack/cluster-api-provider-metal/pkg/cloud/metal/scope"
)

// MetalClusterReconciler reconciles a MetalCluster object
type MetalClusterReconciler struct {
	client.Client
	Log         logr.Logger
	Recorder    record.EventRecorder
	Scheme      *runtime.Scheme
	MetalClient *metal.MetalClient
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *MetalClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	logger := r.Log.WithValues("metalcluster", req.NamespacedName)

	// your logic here
	metalcluster := &infrastructurev1alpha3.MetalCluster{}
	if err := r.Get(ctx, req.NamespacedName, metalcluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger = logger.WithName(metalcluster.APIVersion)

	// Fetch the Machine.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, metalcluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	if cluster == nil {
		logger.Info("OwnerCluster is not set yet. Requeuing...")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 2 * time.Second,
		}, nil
	}

	if util.IsPaused(cluster, metalcluster) {
		logger.Info("MetalCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Logger:       logger,
		Client:       r.Client,
		Cluster:      cluster,
		MetalCluster: metalcluster,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}
	// Always close the scope when exiting this function so we can persist any MetalCluster changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	clusterScope.MetalCluster.Status.Ready = true

	address, err := r.getIP(clusterScope.MetalCluster)
	_, isNoMachine := err.(*MachineNotFound)
	_, isNoIP := err.(*MachineNoIP)
	switch {
	case err != nil && isNoMachine:
		logger.Info("Control plane machine not found. Requeueing...")
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	case err != nil && isNoIP:
		logger.Info("Control plane machine not found. Requeueing...")
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	case err != nil:
		logger.Error(err, "error getting a control plane ip")
		return ctrl.Result{}, err
	case err == nil:
		clusterScope.MetalCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
			Host: address,
			Port: 6443,
		}
	}

	return ctrl.Result{}, nil
}

func (r *MetalClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha3.MetalCluster{}).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(infrastructurev1alpha3.GroupVersion.WithKind("MetalCluster")),
			},
		).
		Complete(r)
}

func (r *MetalClusterReconciler) getIP(cluster *infrastructurev1alpha3.MetalCluster) (string, error) {
	if cluster == nil {
		return "", fmt.Errorf("cannot get IP of machine in nil cluster")
	}
	tags := []string{
		metal.GenerateClusterTag(string(cluster.Name)),
		infrastructurev1alpha3.MasterTag,
	}
	mgr, err := r.MetalClient.GetMachineByTags(cluster.Spec.ProjectID, tags)
	if err != nil {
		return "", fmt.Errorf("error retrieving machine: %v", err)
	}
	if mgr == nil {
		return "", &MachineNotFound{err: fmt.Sprintf("machine does not exist")}
	}
	if len(mgr.Machines) != 1 {
		return "", &MachineNotFound{err: fmt.Sprintf("more or less than one machine found")}
	}
	machine := mgr.Machines[0]

	if machine.Allocation == nil || len(machine.Allocation.Networks) == 0 || len(machine.Allocation.Networks[0].Ips) == 0 || machine.Allocation.Networks[0].Ips[0] == "" {
		return "", &MachineNoIP{err: "machine does not yet have an IP address"}
	}
	return machine.Allocation.Networks[0].Ips[0], nil
}

// MachineNotFound error representing that the requested machine was not yet found
type MachineNotFound struct {
	err string
}

func (e *MachineNotFound) Error() string {
	return e.err
}

// MachineNoIP error representing that the requested machine does not have an IP yet assigned
type MachineNoIP struct {
	err string
}

func (e *MachineNoIP) Error() string {
	return e.err
}
