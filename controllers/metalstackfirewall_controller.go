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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	metalgo "github.com/metal-stack/metal-go"
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
	log := r.Log.WithValues("MetalStackFirewall", req.NamespacedName)

	// Fetch the MetalStackFirewall in the Request.
	firewall := &api.MetalStackFirewall{}
	if err := r.Client.Get(ctx, req.NamespacedName, firewall); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(fmt.Errorf("fetch MetalStackFirewall: %w", err))
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
		return r.reconcileDelete(ctx, log, firewall)
	}

	return r.reconcile(ctx, log, firewall)
}

func (r *MetalStackFirewallReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, firewall *api.MetalStackFirewall) (ctrl.Result, error) {
	logger.Info("Deleting MetalStackFirewall")

	id, err := firewall.Spec.ParsedProviderID()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parse provider ID: %w", err)
	}

	if _, err = r.MetalStackClient.MachineDelete(id); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete the MetalStackFirewall %s: %w", firewall.Name, err)
	}

	controllerutil.RemoveFinalizer(firewall, api.MetalStackFirewallFinalizer)

	logger.Info("Successfully deleted MetalStackFirewall")

	return ctrl.Result{}, nil
}

func (r *MetalStackFirewallReconciler) reconcile(ctx context.Context, logger logr.Logger, firewall *api.MetalStackFirewall) (ctrl.Result, error) {
	controllerutil.AddFinalizer(firewall, api.MetalStackFirewallFinalizer)

	// Check if the firewall was deployed successfully
	if pid, err := firewall.Spec.ParsedProviderID(); err == nil {
		resp, err := r.MetalStackClient.FirewallGet(pid)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get firewall with ID %s: %w", pid, err)
		}

		if resp.Firewall.Allocation != nil {
			succeded := *resp.Firewall.Allocation.Succeeded
			firewall.Status.Ready = succeded

			return ctrl.Result{Requeue: !succeded}, nil
		}
	}

	err := r.createRawMachineIfNotExists(ctx, logger, firewall)

	return ctrl.Result{}, err
}

func (r *MetalStackFirewallReconciler) createRawMachineIfNotExists(ctx context.Context, logger logr.Logger, firewall *api.MetalStackFirewall) error {
	machineCreateReq := metalgo.MachineCreateRequest{
		Description:   firewall.Name + " created by Cluster API provider MetalStack",
		Name:          firewall.Name,
		Hostname:      firewall.Name + "-firewall",
		Size:          firewall.Spec.MachineType,
		Project:       firewall.Spec.ProjectID,
		Partition:     firewall.Spec.Partition,
		Image:         firewall.Spec.Image,
		SSHPublicKeys: firewall.Spec.SSHKeys,
		Networks:      toMachineNetworks(firewall.Spec.PublicNetworkID, *firewall.Spec.PrivateNetworkID),
		UserData:      "",
		Tags:          []string{},
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
		return err
	}

	firewall.Spec.SetProviderID(*resp.Firewall.ID)
	return nil
}
