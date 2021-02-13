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
	"log"

	"github.com/go-logr/logr"
	api "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	metalgo "github.com/metal-stack/metal-go"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// MetalStackFirewallReconciler reconciles a MetalStackFirewall object
type MetalStackFirewallReconciler struct {
	Client           client.Client
	Log              logr.Logger
	MetalStackClient MetalStackClient
	Scheme           *runtime.Scheme
}

func NewMetalStackFirewallReconciler(metalClient MetalStackClient, mgr manager.Manager) *MetalStackFirewallReconciler {
	return &MetalStackFirewallReconciler{
		Client:           mgr.GetClient(),
		Log:              ctrl.Log.WithName("controllers").WithName("MetalStackCluster"),
		MetalStackClient: metalClient,
		Scheme:           mgr.GetScheme(),
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackfirewalls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackfirewalls/status,verbs=get;update;patch

func (r *MetalStackFirewallReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.MetalStackFirewall{}).
		Complete(r)
}

func (r *MetalStackFirewallReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log.WithValues("MetalStackFirewall", req.NamespacedName)

	// Fetch the MetalStackFirewall in the Request.
	firewall := &api.MetalStackFirewall{}
	if err := r.Client.Get(ctx, req.NamespacedName, firewall); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	metalClusterNamespacedName := types.NamespacedName{
		Namespace: firewall.Namespace,
		Name:      firewall.Labels[capi.ClusterLabelName],
	}
	metalCluster := getMetalStackCluster(ctx, logger, r.Client, metalClusterNamespacedName)
	if metalCluster == nil {
		return ctrl.Result{Requeue: true}, nil
	}

	// Persist any change to MetalStackFirewall
	h, err := patch.NewHelper(firewall, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if e := h.Patch(ctx, firewall); e != nil {
			if err != nil {
				err = fmt.Errorf("%s: %w", e.Error(), err)
			}
			err = fmt.Errorf("patch: %w", e)
		}
	}()

	if !firewall.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, logger, firewall, metalCluster)
	}

	return r.reconcile(ctx, logger, firewall, metalCluster)
}

func (r *MetalStackFirewallReconciler) reconcileDelete(
	ctx context.Context,
	logger logr.Logger,
	firewall *api.MetalStackFirewall,
	metalCluster *api.MetalStackCluster,
) (ctrl.Result, error) {
	logger.Info("Deleting MetalStackFirewall")

	id, err := firewall.Spec.ParsedProviderID()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parse provider ID: %w", err)
	}

	resp, err := r.MetalStackClient.FirewallFind(&metalgo.FirewallFindRequest{
		MachineFindRequest: metalgo.MachineFindRequest{
			ID:                &id,
			AllocationProject: &metalCluster.Spec.ProjectID,
			Tags:              []string{metalCluster.GetClusterIDTag()},
		},
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error finding firewalls: %w", err)
	}

	if len(resp.Firewalls) == 1 {
		if _, err = r.MetalStackClient.MachineDelete(id); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete the MetalStackFirewall %s: %w", firewall.Name, err)
		}
	}

	controllerutil.RemoveFinalizer(firewall, api.MetalStackFirewallFinalizer)

	logger.Info("Successfully deleted MetalStackFirewall")

	return ctrl.Result{}, nil
}

func (r *MetalStackFirewallReconciler) reconcile(
	ctx context.Context,
	logger logr.Logger,
	firewall *api.MetalStackFirewall,
	metalCluster *api.MetalStackCluster,
) (ctrl.Result, error) {
	controllerutil.AddFinalizer(firewall, api.MetalStackFirewallFinalizer)

	// Check if the firewall was deployed successfully
	if pid, err := firewall.Spec.ParsedProviderID(); err == nil {
		resp, _ := r.MetalStackClient.MachineGet(pid)
		if resp.Machine.Allocation != nil {
			resp2, err := r.MetalStackClient.FirewallGet(pid)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get firewall with ID %s: %w", pid, err)
			}

			succeded := *resp2.Firewall.Allocation.Succeeded
			firewall.Status.Ready = succeded

			return ctrl.Result{Requeue: !succeded}, nil
		}
	}

	created, err := r.createRawMachineIfNotExists(ctx, logger, firewall, metalCluster)

	return ctrl.Result{Requeue: !created}, err
}

func (r *MetalStackFirewallReconciler) createRawMachineIfNotExists(
	ctx context.Context,
	logger logr.Logger,
	firewall *api.MetalStackFirewall,
	metalCluster *api.MetalStackCluster,
) (bool, error) {
	kubeconfig, err := getKubeconfig(ctx, r.Client, metalCluster)
	if err != nil {
		return false, fmt.Errorf("Failed to get kubeconfig: %w", err)
	}
	if kubeconfig == nil {
		return false, nil
	}

	userData, err := generateFirewallIgnitionConfig(kubeconfig)
	if err != nil {
		return false, fmt.Errorf("Failed to generate firewall ignition config: %w", err)
	}

	log.Println(firewall.Spec.Image)

	log.Println(userData)
	machineCreateReq := metalgo.MachineCreateRequest{
		Description:   firewall.Name + " created by Cluster API provider MetalStack",
		Name:          firewall.Name,
		Hostname:      firewall.Name + "-firewall",
		Size:          firewall.Spec.MachineType,
		Project:       metalCluster.Spec.ProjectID,
		Partition:     metalCluster.Spec.Partition,
		Image:         firewall.Spec.Image,
		SSHPublicKeys: firewall.Spec.SSHKeys,
		Networks:      toMachineNetworks(metalCluster.Spec.PublicNetworkID, *metalCluster.Spec.PrivateNetworkID),
		UserData:      userData,
		Tags:          []string{metalCluster.GetClusterIDTag()},
	}

	// If ProviderID is provided set it in request
	if pid, err := firewall.Spec.ParsedProviderID(); err == nil {
		logger.Info(fmt.Sprintf("Deploy Firewall on machine: %s", pid))
		machineCreateReq.UUID = pid
	}

	resp, err := r.MetalStackClient.FirewallCreate(&metalgo.FirewallCreateRequest{
		MachineCreateRequest: machineCreateReq,
	})
	if err != nil {
		return false, err
	}

	firewall.Spec.SetProviderID(*resp.Firewall.ID)
	return true, nil
}
