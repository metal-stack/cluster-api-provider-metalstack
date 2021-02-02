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
	"strings"

	"github.com/go-logr/logr"
	metalgo "github.com/metal-stack/metal-go"
	corev1 "k8s.io/api/core/v1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiremote "sigs.k8s.io/cluster-api/controllers/remote"
	capierr "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
)

// MetalStackMachineReconciler reconciles a MetalStackMachine object
type MetalStackMachineReconciler struct {
	Client           client.Client
	Log              logr.Logger
	ClusterTracker   *capiremote.ClusterCacheTracker
	MetalStackClient MetalStackClient
}

// todo: Remove the dependency on manager in this package.
func NewMetalStackMachineReconciler(metalClient MetalStackClient, mgr manager.Manager) (reconciler *MetalStackMachineReconciler, err error) {
	clusterTracker, err := capiremote.NewClusterCacheTracker(
		ctrl.Log.WithName("remote").WithName("ClusterCacheTracker"),
		mgr,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to init ClusterTracker: %w", err)
	}

	return &MetalStackMachineReconciler{
		Client:           mgr.GetClient(),
		Log:              ctrl.Log.WithName("controllers").WithName("MetalStackMachine"),
		ClusterTracker:   clusterTracker,
		MetalStackClient: metalClient,
	}, nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *MetalStackMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.MetalStackMachine{}).
		Watches(
			&source.Kind{Type: &capiv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(api.GroupVersion.WithKind("MetalStackMachine")),
			},
		).
		Complete(r)
}

// Reconcile reconciles MetalStackMachine resource
func (r *MetalStackMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, err error) {
	ctx := context.Background()
	logger := r.Log.WithValues("MetalStackMachine", req.NamespacedName)

	logger.Info("Starting MetalStackMachine reconcilation")

	resources, err := newMetalStackMachineResources(ctx, logger, r.Client, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if resources == nil {
		logger.Info("Resources is nil")
		return ctrl.Result{}, nil
	}

	// Check resources readiness
	if !resources.isReady() {
		return ctrl.Result{}, nil
	}

	if util.IsPaused(resources.cluster, resources.metalMachine) {
		resources.logger.Info("Cluster or MetalStackMachine is paused")
		return ctrl.Result{Requeue: true}, nil
	}

	// Persist any change to MetalStackMachine
	h, err := patch.NewHelper(resources.metalMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if e := h.Patch(ctx, resources.metalMachine); e != nil {
			if err != nil {
				err = fmt.Errorf("%s: %w", e.Error(), err)
			}
			err = fmt.Errorf("patch: %w", e)
		}
	}()

	// Check if need to delete MetalStackMachine
	if !resources.isDeletionTimestampZero() {
		return r.reconcileDelete(ctx, resources)
	}

	return r.reconcile(ctx, resources)
}

// reconcileDelete reconciles MetalStackMachine Delete event
func (r *MetalStackMachineReconciler) reconcileDelete(ctx context.Context, resources *metalStackMachineResources) (ctrl.Result, error) {
	resources.logger.Info("Deleting MetalStackMachine")

	id, err := resources.metalMachine.Spec.ParsedProviderID()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parse provider ID: %w", err)
	}

	if _, err = r.MetalStackClient.MachineDelete(id); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete the MetalStackMachine %s: %w", resources.metalMachine.Name, err)
	}

	controllerutil.RemoveFinalizer(resources.metalMachine, api.MetalStackMachineFinalizer)

	resources.logger.Info("Successfully deleted MetalStackMachine")

	return ctrl.Result{}, nil
}

// reconcile reconciles MetalStackMachine Create/Update events
func (r *MetalStackMachineReconciler) reconcile(ctx context.Context, resources *metalStackMachineResources) (ctrl.Result, error) {
	controllerutil.AddFinalizer(resources.metalMachine, api.MetalStackMachineFinalizer)

	if err := r.createRawMachineIfNotExists(ctx, resources); err != nil {
		return ctrl.Result{}, err
	}

	ok, err := r.setNodeProviderID(ctx, resources)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ok {
		resources.logger.Info("Node not ready yet")
		return ctrl.Result{Requeue: true}, nil
	}

	resources.metalMachine.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *MetalStackMachineReconciler) createRawMachineIfNotExists(ctx context.Context, resources *metalStackMachineResources) error {
	// Check if machine already allocated
	if pid, err := resources.metalMachine.Spec.ParsedProviderID(); err == nil {
		resp, err := r.MetalStackClient.MachineGet(pid)
		if err != nil {
			return fmt.Errorf("Failed to get machine with ID %s: %w", pid, err)
		}

		if resp.Machine.Allocation != nil {
			return nil
		}
	}

	// Allocate new machine
	req, err := r.newRequestToCreateMachine(ctx, resources)
	if err != nil {
		return fmt.Errorf("new createMachine request: %w", err)
	}

	resp, err := r.MetalStackClient.MachineCreate(req)
	if err != nil {
		// todo: When to unset?
		resources.metalMachine.Status.SetFailure(err.Error(), capierr.CreateMachineError)
		return err
	}

	resources.setProviderID(resp.Machine)
	return nil
}

func (r *MetalStackMachineReconciler) newRequestToCreateMachine(ctx context.Context, resources *metalStackMachineResources) (*metalgo.MachineCreateRequest, error) {
	name := resources.metalMachine.Name
	networks := toMachineNetworks(resources.metalCluster.Spec.PublicNetworkID, *resources.metalCluster.Spec.PrivateNetworkID)
	userData, err := resources.getBootstrapData(ctx)
	// todo: Remove this
	log.Println("userData: ", string(userData))
	if err != nil {
		return nil, fmt.Errorf("get bootstrap data: %w", err)
	}

	config := &metalgo.MachineCreateRequest{
		Hostname:  name,
		Image:     resources.metalMachine.Spec.Image,
		Name:      name,
		Networks:  networks,
		Partition: resources.metalCluster.Spec.Partition,
		Project:   resources.metalCluster.Spec.ProjectID,
		Size:      resources.metalMachine.Spec.MachineType,
		Tags:      resources.getTagsForRawMachine(),
		UserData:  string(userData),
	}

	// If ProviderID is provided set it in request
	if pid, err := resources.metalMachine.Spec.ParsedProviderID(); err == nil {
		resources.logger.Info(fmt.Sprintf("Deploy Node on machine: %s", pid))
		config.UUID = pid
	}

	if resources.isControlPlane() {
		resources.logger.Info("Creating ControlPlane node")
		config.IPs = []string{resources.metalCluster.Spec.ControlPlaneEndpoint.Host}
	} else {
		resources.logger.Info("Creating worker node")
	}

	return config, nil
}

func (r *MetalStackMachineReconciler) setNodeProviderID(ctx context.Context, resources *metalStackMachineResources) (ok bool, err error) {
	providerID := resources.getProviderID()
	if providerID == nil {
		return false, fmt.Errorf("providerID is nil")
	}

	remoteClient, err := r.ClusterTracker.GetClient(ctx, util.ObjectKey(resources.cluster))
	if err != nil {
		return false, nil
	}

	node, err := getNode(ctx, remoteClient, *providerID)
	if err != nil {
		return false, fmt.Errorf("get node: %w", err)
	}
	if node == nil {
		resources.logger.Info(fmt.Sprintf("Didn't found node with providerID: %s", *providerID))
		return false, nil
	}

	// Persist change to Node
	h, err := patch.NewHelper(node, remoteClient)
	if err != nil {
		return false, err
	}

	node.Spec.ProviderID = *providerID
	resources.logger.Info(fmt.Sprintf("Set node's providerID: %s", *providerID))

	if err = h.Patch(ctx, node); err != nil {
		return false, fmt.Errorf("Failed to update the target node: %w", err)
	}

	return true, nil
}

func getNode(ctx context.Context, remoteClient client.Client, providerID string) (node *corev1.Node, err error) {
	nodeList := corev1.NodeList{}
	for {
		if err := remoteClient.List(ctx, &nodeList, client.Continue(nodeList.Continue)); err != nil {
			return nil, err
		}

		for _, node := range nodeList.Items {
			if strings.Contains(providerID, node.Status.NodeInfo.SystemUUID) {
				return &node, nil
			}
		}

		if nodeList.Continue == "" {
			break
		}
	}

	return nil, nil
}
