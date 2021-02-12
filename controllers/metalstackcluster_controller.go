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
	api "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/tag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// MetalStackClusterReconciler reconciles a MetalStackCluster object
type MetalStackClusterReconciler struct {
	Client           client.Client
	Log              logr.Logger
	MetalStackClient MetalStackClient
	Scheme           *runtime.Scheme
}

func NewMetalStackClusterReconciler(metalClient MetalStackClient, mgr manager.Manager) *MetalStackClusterReconciler {
	return &MetalStackClusterReconciler{
		Client:           mgr.GetClient(),
		Log:              ctrl.Log.WithName("controllers").WithName("MetalStackCluster"),
		MetalStackClient: metalClient,
		Scheme:           mgr.GetScheme(),
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *MetalStackClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.MetalStackCluster{}).
		Watches(
			&source.Kind{Type: &capi.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(api.GroupVersion.WithKind("MetalStackCluster")),
			}).
		Complete(r)
}

// Reconcile reconciles MetalStackCluster resource
func (r *MetalStackClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, err error) {
	ctx := context.Background()
	logger := r.Log.WithValues("MetalStackCluster", req.NamespacedName)

	logger.Info("Starting MetalStackCluster reconcilation")

	// Fetch the MetalStackCluster.
	metalCluster := &api.MetalStackCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, metalCluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Persist any changes to MetalStackCluster.
	h, err := patch.NewHelper(metalCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("init patch helper: %w", err)
	}
	defer func() {
		if e := h.Patch(ctx, metalCluster); e != nil {
			if err != nil {
				err = fmt.Errorf("%s: %w", e.Error(), err)
			}
			err = fmt.Errorf("patch: %w", e)
		}
	}()

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, metalCluster.ObjectMeta)
	if err != nil {
		clusterErr := capierrors.InvalidConfigurationClusterError
		metalCluster.Status.FailureReason = &clusterErr
		metalCluster.Status.FailureMessage = pointer.StringPtr("Unable to get OwnerCluster")
		return ctrl.Result{}, fmt.Errorf("get OwnerCluster: %w", err)
	}
	if cluster == nil {
		logger.Info("Waiting for cluster controller to set OwnerRef to MetalStackCluster")
		return ctrl.Result{Requeue: true}, nil
	}

	if util.IsPaused(cluster, metalCluster) {
		logger.Info("reconcilation is paused for this object")
		return ctrl.Result{Requeue: true}, nil
	}

	if !metalCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, logger, cluster, metalCluster)
	}

	return r.reconcile(ctx, logger, metalCluster)
}

func (r *MetalStackClusterReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, cluster *capi.Cluster, metalCluster *api.MetalStackCluster) (ctrl.Result, error) {
	logger.Info("Deleting MetalStackCluster")

	// Check if there's still active machines in Cluster
	machineCount, err := r.countMachines(ctx, cluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to count machines: %w", err)
	}
	if machineCount > 0 {
		// Delete machines
		logger.Info("Deleting Cluster's Machines")
		if err := r.deleteMachines(ctx, cluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete machines: %w", err)
		}
	}

	// Delete firewall
	logger.Info("Deleting Firewall")
	if err := r.deleteFirewall(ctx, metalCluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete Firewall: %w", err)
	}

	// Delete network
	logger.Info("Deleting Cluster network")
	resp, err := r.MetalStackClient.NetworkFind(&metalgo.NetworkFindRequest{
		ID:        metalCluster.Spec.PrivateNetworkID,
		ProjectID: &metalCluster.Spec.ProjectID,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list networks: %w", err)
	}

	if len(resp.Networks) == 1 {
		if _, err := r.MetalStackClient.NetworkFree(*metalCluster.Spec.PrivateNetworkID); err != nil {
			return ctrl.Result{
				Requeue: true,
			}, nil
		}
	}

	controllerutil.RemoveFinalizer(metalCluster, api.MetalStackClusterFinalizer)
	logger.Info("Successfully deleted MetalStackCluster")

	return ctrl.Result{}, nil
}

func (r *MetalStackClusterReconciler) reconcile(ctx context.Context, logger logr.Logger, metalCluster *api.MetalStackCluster) (ctrl.Result, error) {
	controllerutil.AddFinalizer(metalCluster, api.MetalStackClusterFinalizer)

	// Allocate network.
	if metalCluster.Spec.PrivateNetworkID == nil {
		if err := r.allocateNetwork(metalCluster); err != nil {
			logger.Info(err.Error() + ": requeueing")
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Allocate IP for API server
	if !metalCluster.Status.ControlPlaneIPAllocated {
		if err := r.allocateControlPlaneIP(logger, metalCluster); err != nil {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	firewall := &api.MetalStackFirewall{}
	if err := r.Client.Get(ctx, metalCluster.GetFirewallNamespacedName(), firewall); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to fetch firewall: %w", err)
		}

		if err := r.createFirewall(ctx, metalCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create firewall: %w", err)
		}

		logger.Info("Cluster firewall is created")
	}

	metalCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *MetalStackClusterReconciler) allocateNetwork(metalCluster *api.MetalStackCluster) error {
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

func (r *MetalStackClusterReconciler) allocateControlPlaneIP(logger logr.Logger, metalCluster *api.MetalStackCluster) error {
	resp, err := r.MetalStackClient.IPAllocate(&metalgo.IPAllocateRequest{
		Description: "",
		Name:        metalCluster.Name + "-api-server-IP",
		Networkid:   metalCluster.Spec.PublicNetworkID,
		Projectid:   metalCluster.Spec.ProjectID,
		IPAddress:   metalCluster.Spec.ControlPlaneEndpoint.Host,
		Type:        "",
		Tags:        []string{},
	})
	if err != nil {
		logger.Info(fmt.Sprintf("Failed to allocate Control Plane IP %s", err))
		return err
	}

	metalCluster.Spec.ControlPlaneEndpoint.Host = *resp.IP.Ipaddress

	metalCluster.Status.ControlPlaneIPAllocated = true
	logger.Info(fmt.Sprintf("Control Plane IP %s allocated", *resp.IP.Ipaddress))

	return nil
}

func (r *MetalStackClusterReconciler) countMachines(ctx context.Context, cluster *capi.Cluster) (count int, err error) {
	machines := capi.MachineList{}
	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			capi.ClusterLabelName: cluster.Name,
		}),
	}

	if r.Client.List(ctx, &machines, listOptions...) != nil {
		return 0, fmt.Errorf("failed to list machines: %w", err)
	}

	return len(machines.Items), nil
}

func (r *MetalStackClusterReconciler) deleteMachines(ctx context.Context, cluster *capi.Cluster) error {
	machine := capi.Machine{}
	deleteOptions := []client.DeleteAllOfOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			capi.ClusterLabelName: cluster.Name,
		}),
	}

	return r.Client.DeleteAllOf(ctx, &machine, deleteOptions...)
}

func (r *MetalStackClusterReconciler) createFirewall(ctx context.Context, metalCluster *api.MetalStackCluster) error {
	firewall := &api.MetalStackFirewall{}
	firewall.Name = metalCluster.Name
	firewall.Namespace = metalCluster.Namespace
	firewall.Spec = metalCluster.Spec.FirewallSpec
	firewall.Labels = map[string]string{
		capi.ClusterLabelName: metalCluster.Name,
	}

	return r.Client.Create(ctx, firewall)
}

func (r *MetalStackClusterReconciler) deleteFirewall(ctx context.Context, metalCluster *api.MetalStackCluster) error {
	firewall := &api.MetalStackFirewall{}
	deleteOptions := []client.DeleteAllOfOption{
		client.InNamespace(metalCluster.Namespace),
		client.MatchingLabels(map[string]string{
			capi.ClusterLabelName: metalCluster.Name,
		}),
	}

	return r.Client.DeleteAllOf(ctx, firewall, deleteOptions...)
}
