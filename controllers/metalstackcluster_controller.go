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
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/pkg/errors"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterapi "sigs.k8s.io/cluster-api/api/v1alpha3"

	// clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
)

const (
	clusterIDTag = "cluster-api-provider-metalstack:cluster-id"
)

// MetalStackClusterReconciler reconciles a MetalStackCluster object
type MetalStackClusterReconciler struct {
	client.Client
	Log         logr.Logger
	MetalClient *metalgo.Driver
	Recorder    record.EventRecorder
	Scheme      *runtime.Scheme
}

func NewMetalStackClusterReconciler(metalClient *metalgo.Driver, mgr manager.Manager) *MetalStackClusterReconciler {
	return &MetalStackClusterReconciler{
		Client:      mgr.GetClient(),
		Log:         ctrl.Log.WithName("controllers").WithName("MetalStackMachine"),
		MetalClient: metalClient,
		Recorder:    mgr.GetEventRecorderFor("metalstackmachine-controller"),
		Scheme:      mgr.GetScheme(),
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *MetalStackClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, err error) {
	ctx := context.Background()
	logger := r.Log.WithValues("MetalStackCluster", req.NamespacedName)

	metalCluster := &v1alpha3.MetalStackCluster{}
	if err := r.Get(ctx, req.NamespacedName, metalCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger = logger.WithName(metalCluster.APIVersion)

	cluster, err := util.GetOwnerCluster(ctx, r.Client, metalCluster.ObjectMeta)
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

	if util.IsPaused(cluster, metalCluster) {
		logger.Info("MetalStackCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	h, err := patch.NewHelper(metalCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if e := h.Patch(ctx, metalCluster); e != nil {
			if err != nil {
				err = errors.Wrap(e, err.Error())
			}
			err = e
		}
	}()

	if metalCluster.Spec.PrivateNetworkID == nil {
		networkID, err := r.allocateNetwork(metalCluster)
		if err != nil {
			logger.Error(err, "no response to network allocation")
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Second}, err
		}
		metalCluster.Spec.PrivateNetworkID = networkID
	}

	if !metalCluster.Status.FirewallReady {
		err = r.createFirewall(metalCluster)
		if err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Second}, err
		}
		logger.Info("A firewall was created.")
		metalCluster.Status.FirewallReady = true
	}

	metalCluster.Status.Ready = true

	ip, err := r.getControlPlaneIP(metalCluster)
	if err != nil {
		switch err.(type) {
		case *MachineNotFound, *IPNotAllocated: // todo: Do we really need these two types? Check the logs.
			logger.Info(" Control plane machine not found. Requeueing...", "error", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
		default:
			logger.Error(err, " error getting a control plane ip")
			return ctrl.Result{}, err
		}
	}
	metalCluster.Spec.ControlPlaneEndpoint = clusterapi.APIEndpoint{
		Host: ip,
		Port: 6443,
	}

	return ctrl.Result{}, nil
}

func (r *MetalStackClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha3.MetalStackCluster{}).
		Watches(
			&source.Kind{Type: &clusterapi.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(v1alpha3.GroupVersion.WithKind("MetalStackCluster")),
			},
		).
		Complete(r)
}

func (r *MetalStackClusterReconciler) allocateNetwork(metalCluster *v1alpha3.MetalStackCluster) (*string, error) {
	resp, err := r.MetalClient.NetworkAllocate(&metalgo.NetworkAllocateRequest{
		ProjectID:   *metalCluster.Spec.ProjectID,
		PartitionID: *metalCluster.Spec.Partition,
		Name:        *metalCluster.Spec.Partition,
		Description: metalCluster.Name,
		Labels:      map[string]string{tag.ClusterID: metalCluster.Name},
	})
	if err != nil {
		return nil, err
	}
	return resp.Network.ID, nil
}

// todo: Ask metal-API to find out the external network IP (partition id empty -> destinationprefix: 0.0.0.0/0)
func (r *MetalStackClusterReconciler) createFirewall(metalCluster *v1alpha3.MetalStackCluster) error {
	req := &metalgo.FirewallCreateRequest{
		MachineCreateRequest: metalgo.MachineCreateRequest{
			Description:   metalCluster.Name + " created by Cluster API provider MetalStack",
			Name:          metalCluster.Name,
			Hostname:      metalCluster.Name + "-firewall",
			Size:          "v1-small-x86",
			Project:       *metalCluster.Spec.ProjectID,
			Partition:     *metalCluster.Spec.Partition,
			Image:         "firewall-ubuntu-2.0",
			SSHPublicKeys: []string{},
			Networks:      toNetworks(*metalCluster.Spec.Firewall.DefaultNetworkID, *metalCluster.Spec.PrivateNetworkID),
			UserData:      "",
			Tags:          []string{},
		},
	}

	_, err := r.MetalClient.FirewallCreate(req)
	return err
}

// todo: Implement?
// The IP is internal at the moment, which can be replaced by explicitly allocated IP at the creation of the control plane.
func (r *MetalStackClusterReconciler) getControlPlaneIP(metalCluster *v1alpha3.MetalStackCluster) (string, error) {
	if metalCluster == nil {
		return "", errors.New("pointer to MetalStackCluster not allowed to be nil")
	}

	tags := []string{
		clusterIDTag + ":" + metalCluster.Name,
		clusterapi.MachineControlPlaneLabelName + ":true",
	}
	mm, err := r.MetalClient.MachineFind(&metalgo.MachineFindRequest{
		AllocationProject: metalCluster.Spec.ProjectID,
		Tags:              tags,
	})
	if err != nil {
		return "", err
	}
	if mm == nil {
		return "", &MachineNotFound{fmt.Sprintf("machine with the project ID %v and the tags %v not found", *metalCluster.Spec.ProjectID, tags)}
	}

	if len(mm.Machines) != 1 {
		return "", &MachineNotFound{fmt.Sprintf("%v machine(s) found", len(mm.Machines))}
	}
	m := mm.Machines[0]

	if m.Allocation == nil || len(m.Allocation.Networks) == 0 || len(m.Allocation.Networks[0].Ips) == 0 || m.Allocation.Networks[0].Ips[0] == "" {
		return "", &IPNotAllocated{"IP address not allocate"}
	}
	return m.Allocation.Networks[0].Ips[0], nil
}

// MachineNotFound error representing that the requested machine was not yet found
type MachineNotFound struct {
	s string
}

func (e *MachineNotFound) Error() string {
	return e.s
}

// IPNotAllocated error representing that the requested machine does not have an IP yet assigned
type IPNotAllocated struct {
	s string
}

func (e *IPNotAllocated) Error() string {
	return e.s
}

func toNetworks(ss ...string) (networks []metalgo.MachineAllocationNetwork) {
	for _, s := range ss {
		networks = append(networks, metalgo.MachineAllocationNetwork{
			NetworkID:   s,
			Autoacquire: true,
		})
	}
	return
}
