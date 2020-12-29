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
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	cluster "sigs.k8s.io/cluster-api/api/v1alpha3"
	clustererr "sigs.k8s.io/cluster-api/errors"
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

const (
	// MetalStackMachineFinalizer is the finalizer for MetalStackMachine.
	MetalStackMachineFinalizer = "metalstackmachine.infrastructure.cluster.x-k8s.io"
)

// MetalStackMachineReconciler reconciles a MetalStackMachine object
type MetalStackMachineReconciler struct {
	Client           client.Client
	Log              logr.Logger
	MetalStackClient MetalStackClient
}

// todo: Remove the dependency on manager in this package.
func NewMetalStackMachineReconciler(metalClient MetalStackClient, mgr manager.Manager) *MetalStackMachineReconciler {
	return &MetalStackMachineReconciler{
		Client:           mgr.GetClient(),
		Log:              ctrl.Log.WithName("controllers").WithName("MetalStackMachine"),
		MetalStackClient: metalClient,
	}
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *MetalStackMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.MetalStackMachine{}).
		Watches(
			&source.Kind{Type: &cluster.Machine{}},
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

	resources, err := fetchMetalStackMachineResources(ctx, logger, r.Client, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check resources readiness
	if !resources.isReady() {
		return requeueInstantly, nil
	}

	// Persist any change to MetalStackMachine
	h, err := patch.NewHelper(resources.metalMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if e := h.Patch(ctx, resources.metalMachine); e != nil {
			if err != nil {
				err = errors.Wrap(e, err.Error())
			}
			err = e
		}
	}()

	// Check if need to delete MetalStackMachine
	if !resources.isDeletionTimestampZero() {
		return r.reconcileDelete(ctx, resources)
	}

	controllerutil.AddFinalizer(resources.metalMachine, MetalStackMachineFinalizer)

	return r.reconcile(ctx, resources)
}

// reconcileDelete reconciles MetalStackMachine Delete event
func (r *MetalStackMachineReconciler) reconcileDelete(ctx context.Context, resources *metalStackMachineResources) (ctrl.Result, error) {
	resources.logger.Info("Trying to delete MetalStackMachine")

	id, err := resources.metalMachine.Spec.ParsedProviderID()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parse provider ID: %w", err)
	}

	if _, err = r.MetalStackClient.MachineDelete(id); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete the MetalStackMachine %s: %w", resources.metalMachine.Name, err)
	}

	controllerutil.RemoveFinalizer(resources.metalMachine, MetalStackMachineFinalizer)

	resources.logger.Info("Successfully deleted MetalStackMachine")

	return ctrl.Result{}, nil
}

// reconcile reconciles MetalStackMachine Create/Update events
func (r *MetalStackMachineReconciler) reconcile(ctx context.Context, resources *metalStackMachineResources) (ctrl.Result, error) {
	rawMachine, err := r.getRawMachineOrCreate(ctx, resources)
	if err != nil {
		return ctrl.Result{}, err
	}
	resources.updateMetalMachineStatus(rawMachine)

	return ctrl.Result{}, nil
}

func (r *MetalStackMachineReconciler) getRawMachineOrCreate(ctx context.Context, resources *metalStackMachineResources) (*models.V1MachineResponse, error) {
	id, err := resources.metalMachine.Spec.ParsedProviderID()
	if err != nil {
		if err == api.ProviderIDNotSet {
			req, err := r.newRequestToCreateMachine(ctx, resources)
			if err != nil {
				return nil, fmt.Errorf("new createMachine request: %w", err)
			}

			resp, err := r.MetalStackClient.MachineCreate(req)
			if err != nil {
				// todo: When to unset?
				resources.metalMachine.Status.SetFailure(err.Error(), clustererr.CreateMachineError)
				return nil, err
			}

			return resp.Machine, nil
		}

		return nil, err
	}

	resp, err := r.MetalStackClient.MachineGet(id)
	if err != nil {
		return nil, err
	}

	return resp.Machine, nil
}

func (r *MetalStackMachineReconciler) newRequestToCreateMachine(ctx context.Context, resources *metalStackMachineResources) (*metalgo.MachineCreateRequest, error) {
	name := resources.metalMachine.Name
	networks := toNetworks(*resources.metalCluster.Spec.Firewall.DefaultNetworkID, *resources.metalCluster.Spec.PrivateNetworkID)
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

	if resources.isControlPlane() {
		resources.logger.Info("Creating ControlPlane node")
		config.IPs = []string{resources.metalCluster.Spec.ControlPlaneEndpoint.Host}
	} else {
		resources.logger.Info("Creating worker node")
	}

	return config, nil
}
