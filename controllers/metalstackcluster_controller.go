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

	metalgo "github.com/metal-stack/metal-go"
	"github.com/pkg/errors"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
)

const (
	clusterIDTag = "cluster-api-provider-metalstack:cluster-id"
)

// MetalStackClusterReconciler reconciles a MetalStackCluster object
type MetalStackClusterReconciler struct {
	client.Client

	Log       logr.Logger
	MStClient *metalgo.Driver
	Recorder  record.EventRecorder
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *MetalStackClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, err error) {
	ctx := context.Background()
	logger := r.Log.WithValues("MetalStackCluster", req.NamespacedName)

	mstCluster := &v1alpha3.MetalStackCluster{}
	if err := r.Get(ctx, req.NamespacedName, mstCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger = logger.WithName(mstCluster.APIVersion)

	cluster, err := util.GetOwnerCluster(ctx, r.Client, mstCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	if cluster == nil {
		logger.Info("OwnerCluster is not set yet. The reconciliation request will be requeued.")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 2 * time.Second,
		}, nil
	}

	if util.IsPaused(cluster, mstCluster) {
		logger.Info("MetalStackCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	h, err := patch.NewHelper(mstCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to initialize patch.Helper")
	}
	defer func(h *patch.Helper) {
		if e := h.Patch(context.TODO(), mstCluster); err == nil && e != nil {
			err = e
		}
	}(h)

	mstCluster.Status.Ready = true

	// todo: clear up the error handling
	address, err := r.getIP(mstCluster)
	_, isNoMachine := err.(*MachineNotFound)
	_, isNoIP := err.(*MachineNoIP)
	switch {
	case err != nil && isNoMachine:
		logger.Info(err.Error() + "Control plane machine not found. Requeueing...")
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	case err != nil && isNoIP:
		logger.Info(err.Error(), "Control plane machine not found. Requeueing...")
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	case err != nil:
		logger.Info(err.Error(), "error getting a control plane ip")
		return ctrl.Result{}, err
	case err == nil:
		mstCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
			Host: address,
			Port: 6443,
		}
	}

	return ctrl.Result{}, nil
}

func (r *MetalStackClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha3.MetalStackCluster{}).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(v1alpha3.GroupVersion.WithKind("MetalStackCluster")),
			},
		).
		Complete(r)
}

func (r *MetalStackClusterReconciler) getIP(mstCluster *v1alpha3.MetalStackCluster) (string, error) {
	if mstCluster == nil {
		return "", fmt.Errorf("cannot get IP of machine in nil cluster")
	}
	tags := []string{
		fmt.Sprintf("%s:%s", clusterIDTag, mstCluster.Name),
		v1alpha3.MasterTag,
	}

	mm, err := r.MStClient.MachineFind(&metalgo.MachineFindRequest{AllocationProject: mstCluster.Spec.ProjectID, Tags: tags})

	if err != nil {
		return "", fmt.Errorf("Error retrieving machines: %v", err)
	}

	if mm == nil {
		return "", &MachineNotFound{fmt.Sprintf("A machine with the project ID %s and tags %v doesn't exist.", *mstCluster.Spec.ProjectID, tags)}
	}

	if len(mm.Machines) != 1 {
		return "", &MachineNotFound{fmt.Sprintf("%v machine(s) found", len(mm.Machines))}
	}
	m := mm.Machines[0]

	if m.Allocation == nil || len(m.Allocation.Networks) == 0 || len(m.Allocation.Networks[0].Ips) == 0 || m.Allocation.Networks[0].Ips[0] == "" {
		return "", &MachineNoIP{"The machine doesn't have an IP address."}
	}
	return m.Allocation.Networks[0].Ips[0], nil
}

func (r *MetalStackClusterReconciler) newHelper(c *v1alpha3.MetalStackCluster) (*patch.Helper, error) {
	return patch.NewHelper(c, r.Client)
}

// MachineNotFound error representing that the requested machine was not yet found
type MachineNotFound struct {
	s string
}

func (e *MachineNotFound) Error() string {
	return e.s
}

// MachineNoIP error representing that the requested machine does not have an IP yet assigned
type MachineNoIP struct {
	s string
}

func (e *MachineNoIP) Error() string {
	return e.s
}
