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

	"github.com/go-logr/logr"
	infra "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterapi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// MetalStackClusterReconciler reconciles a MetalStackCluster object
type MetalStackClusterReconciler struct {
	Client           client.Client
	Log              logr.Logger
	MetalStackClient MetalStackClient
}

func NewMetalStackClusterReconciler(metalClient MetalStackClient, mgr manager.Manager) *MetalStackClusterReconciler {
	return &MetalStackClusterReconciler{
		Client:           mgr.GetClient(),
		Log:              ctrl.Log.WithName("controllers").WithName("MetalStackCluster"),
		MetalStackClient: metalClient,
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *MetalStackClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infra.MetalStackCluster{}).
		Watches(
			&source.Kind{Type: &clusterapi.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(infra.GroupVersion.WithKind("MetalStackCluster")),
			}).
		Complete(r)
}

func (r *MetalStackClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, err error) {
	logger := r.Log.WithValues("MetalStackCluster", req.NamespacedName)
	ctx := context.Background()

	// Fetch the MetalStackCluster.
	metalCluster := &infra.MetalStackCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, metalCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetch MetalStackCluster: %w", err)
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, metalCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get owner cluster: %w", err)
	}
	if cluster == nil {
		logger.Info("OwnerCluster not set: requeueing")
		return requeue, nil
	}
	if util.IsPaused(cluster, metalCluster) {
		logger.Info("MetalStackCluster or its OwnerCluster paused: reconciliation stopped")
		return ctrl.Result{}, nil
	}

	// Persist any change of the MetalStackCluster.
	h, err := patch.NewHelper(metalCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("patch cluster: %w", err)
	}
	defer func() {
		if e := h.Patch(ctx, metalCluster); e != nil {
			if err != nil {
				err = errors.Wrap(e, err.Error())
			}
			err = e
		}
	}()

	// Allocate network.
	if metalCluster.Spec.PrivateNetworkID == nil {
		if err := r.allocateNetwork(metalCluster); err != nil {
			logger.Info(err.Error() + ": requeueing")
			return requeue, nil
		}

	}

	// Allocate IP for API server
	if !metalCluster.Status.ControlPlaneIPAllocated {
		if err := r.allocateControlPlaneIP(metalCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to allocate API-server IP: %w", err)
		}

		logger.Info("", "API-server IP", metalCluster.Spec.ControlPlaneEndpoint.Host)
		metalCluster.Status.ControlPlaneIPAllocated = true
	}

	// Create firewall
	if !metalCluster.Status.FirewallReady {
		err = r.createFirewall(metalCluster)
		if err != nil {
			logger.Info(err.Error() + ": requeueing")
			return requeue, nil
		}
		logger.Info("firewall created")
		metalCluster.Status.FirewallReady = true

		// `FirewallReady` should be updated immediately, so
		if err := r.Client.Status().Update(ctx, metalCluster); err != nil {
			logger.Error(err, "error while updating the status `FirewallReady`")
			return ctrl.Result{}, fmt.Errorf("status update: %w", err)
		}
	}

	metalCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *MetalStackClusterReconciler) allocateNetwork(metalCluster *infra.MetalStackCluster) error {
	resp, err := r.MetalStackClient.NetworkAllocate(&metalgo.NetworkAllocateRequest{
		Description: metalCluster.Name,
		Labels:      map[string]string{tag.ClusterID: metalCluster.Name},
		Name:        metalCluster.Spec.Partition,
		PartitionID: metalCluster.Spec.Partition,
		ProjectID:   metalCluster.Spec.ProjectID,
	})
	if err != nil {
		return err
	}

	metalCluster.Spec.PrivateNetworkID = resp.Network.ID

	return nil
}

func (r *MetalStackClusterReconciler) allocateControlPlaneIP(metalCluster *infra.MetalStackCluster) error {
	_, err := r.MetalStackClient.IPAllocate(&metalgo.IPAllocateRequest{
		Description: "",
		Name:        metalCluster.Name + "api-server-IP",
		Networkid:   "internet-vagrant-lab",
		Projectid:   metalCluster.Spec.ProjectID,
		IPAddress:   metalCluster.Spec.ControlPlaneEndpoint.Host,
		Type:        "",
		Tags:        []string{},
	})

	return err
}

// todo: Not used.
func (r *MetalStackClusterReconciler) controlPlaneIP(metalCluster *infra.MetalStackCluster) (string, error) {
	tags := metalCluster.ControlPlaneTags()
	mm, err := r.MetalStackClient.MachineFind(&metalgo.MachineFindRequest{
		AllocationProject: &metalCluster.Spec.ProjectID,
		Tags:              tags,
	})
	if err != nil {
		return "", err
	}
	if mm == nil {
		return "", newErrMachineNotFound(metalCluster.Spec.ProjectID, tags)
	}

	// todo: Consider high availabilty case and test it.
	if len(mm.Machines) != 1 {
		return "", &errMachineNotFound{fmt.Sprintf("%v machine(s) found", len(mm.Machines))}
	}
	m := mm.Machines[0]
	if m.Allocation == nil || len(m.Allocation.Networks) == 0 || len(m.Allocation.Networks[0].Ips) == 0 || m.Allocation.Networks[0].Ips[0] == "" {
		return "", &errIPNotAllocated{"IP address not allocated"}
	}
	return m.Allocation.Networks[0].Ips[0], nil
}

// todo: Ask metal-API for an available external network IP (partition id empty -> destinationprefix: 0.0.0.0/0)
func (r *MetalStackClusterReconciler) createFirewall(metalCluster *infra.MetalStackCluster) error {
	if metalCluster.Spec.Firewall.DefaultNetworkID == nil {
		return newErrSpecNotSet("Firewall.DefaultNetworkID")
	}
	if metalCluster.Spec.Firewall.Image == nil {
		return newErrSpecNotSet("Firewall.Image")
	}
	if metalCluster.Spec.Firewall.Size == nil {
		return newErrSpecNotSet("Firewall.Size")
	}
	if metalCluster.Spec.PrivateNetworkID == nil {
		return newErrSpecNotSet("PrivateNetworkID")
	}

	_, err := r.MetalStackClient.FirewallCreate(&metalgo.FirewallCreateRequest{
		MachineCreateRequest: metalgo.MachineCreateRequest{
			Description:   metalCluster.Name + " created by Cluster API provider MetalStack",
			Name:          metalCluster.Name,
			Hostname:      metalCluster.Name + "-firewall",
			Size:          *metalCluster.Spec.Firewall.Size,
			Project:       metalCluster.Spec.ProjectID,
			Partition:     metalCluster.Spec.Partition,
			Image:         *metalCluster.Spec.Firewall.Image,
			SSHPublicKeys: metalCluster.Spec.Firewall.SSHKeys,
			Networks:      toNetworks(*metalCluster.Spec.Firewall.DefaultNetworkID, *metalCluster.Spec.PrivateNetworkID),
			UserData:      "",
			Tags:          []string{},
		},
	})
	return err
}

// errIPNotAllocated error representing that the requested machine does not have an IP yet assigned
type errIPNotAllocated struct {
	s string
}

func (e *errIPNotAllocated) Error() string {
	return e.s
}

// errMachineNotFound error representing that the requested machine was not yet found
type errMachineNotFound struct {
	s string
}

func (e *errMachineNotFound) Error() string {
	return e.s
}
func newErrMachineNotFound(projectID string, tags []string) *errMachineNotFound {
	return &errMachineNotFound{fmt.Sprintf("machine with the project ID %v and the tags %v not found", projectID, tags)}
}

type errSpecNotSet struct {
	msg string
}

func (e *errSpecNotSet) Error() string {
	return e.msg
}

func newErrSpecNotSet(s string) *errSpecNotSet {
	return &errSpecNotSet{
		msg: s + " not set",
	}
}
