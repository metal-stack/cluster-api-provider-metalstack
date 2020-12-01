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

	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

	infra "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
)

const (
	// MachineFinalizer is the finalizer for MetalStackMachine.
	MachineFinalizer = "metalstackmachine.infrastructure.cluster.x-k8s.io"
)

// MetalStackMachineReconciler reconciles a MetalStackMachine object
type MetalStackMachineReconciler struct {
	client.Client
	Log logr.Logger
	MetalStackClient
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

func (r *MetalStackMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, err error) {
	logger := r.Log.WithValues("MetalStackMachine", req.NamespacedName)
	ctx := context.Background()

	// Fetch the MetalStackMachine.
	metalMachine := &infra.MetalStackMachine{}
	if err := r.Get(ctx, req.NamespacedName, metalMachine); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, metalMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		logger.Info("no OwnerReference of the MetalStackMachine has the Kind Machine")
		return ctrl.Result{}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Return if the retrieved objects are paused.
	if util.IsPaused(cluster, metalMachine) {
		logger.Info("the Cluster is paused or the MetalStackMachine has the `paused` annotation")
		return ctrl.Result{}, nil
	}

	// Fetch the MetalStackCluster.
	k := client.ObjectKey{
		Namespace: metalMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	metalCluster := &infra.MetalStackCluster{}
	if err := r.Get(ctx, k, metalCluster); err != nil {
		logger.Info(err.Error())
		return ctrl.Result{}, nil
	}

	rsrc := newResource(cluster, machine, metalCluster, metalMachine)

	// Persist any change of the MetalStackMachine.
	h, err := patch.NewHelper(rsrc.metalMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if e := h.Patch(ctx, rsrc.metalMachine); e != nil {
			if err != nil {
				err = errors.Wrap(e, err.Error())
			}
			err = e
		}
	}()

	if !rsrc.metalMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.deleteMachine(ctx, logger, rsrc)
	}

	// todo: Check if the failure still holds after some time.
	// todo: Check the logic of failure. It should be Idempotent.
	if rsrc.metalMachine.Status.Failed() {
		logger.Info("Status of the MetalStackMachine showing failure")
		return ctrl.Result{}, nil
	}

	controllerutil.AddFinalizer(rsrc.metalMachine, MachineFinalizer)

	if !rsrc.cluster.Status.InfrastructureReady {
		logger.Info("firewall not ready")
		return ctrl.Result{Requeue: true}, nil
	}

	if rsrc.machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("bootstrap not ready yet")
		return ctrl.Result{}, nil
	}

	raw, err := r.getRawMachineOrCreate(logger, rsrc)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.setMachineStatus(rsrc, raw)

	return ctrl.Result{}, nil
}

func (r *MetalStackMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infra.MetalStackMachine{}).
		Watches(
			&source.Kind{Type: &cluster.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infra.GroupVersion.WithKind("MetalStackMachine")),
			},
		).
		Complete(r)
}

func (r *MetalStackMachineReconciler) bootstrapData(rsrc *resource) ([]byte, error) {
	secretName := rsrc.machine.Spec.Bootstrap.DataSecretName
	if secretName == nil {
		return nil, errors.New("owner Machine's Spec.Bootstrap.DataSecretName being nil")
	}

	secret := &core.Secret{}
	if err := r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: rsrc.metalMachine.Namespace,
			Name:      *secretName,
		},
		secret,
	); err != nil {
		return nil, err
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("key `value` of the map `Data` of the bootstrap data missing")
	}

	return value, nil
}
func (r *MetalStackMachineReconciler) deleteMachine(ctx context.Context, logger logr.Logger, rsrc *resource) (ctrl.Result, error) {
	logger.Info("the MetalStackMachine being deleted")

	id, err := rsrc.metalMachine.Spec.ParsedProviderID()
	if err != nil {
		return ctrl.Result{}, err
	}

	if _, err = r.MetalStackClient.MachineDelete(id); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete the MetalStackMachine %v: %v", rsrc.metalMachine.Name, err)
	}

	controllerutil.RemoveFinalizer(rsrc.metalMachine, MachineFinalizer)

	return ctrl.Result{}, nil
}

func (r *MetalStackMachineReconciler) getRawMachineOrCreate(logger logr.Logger, rsrc *resource) (*models.V1MachineResponse, error) {
	id, err := rsrc.metalMachine.Spec.ParsedProviderID()
	if err != nil {
		if err == infra.ProviderIDNotSet {
			logger.Info(err.Error())
			req, err := r.newRequestToCreateMachine(rsrc)
			if err != nil {
				return nil, err
			}
			resp, err := r.MetalStackClient.MachineCreate(req)
			if err != nil {
				// todo: When to unset?
				rsrc.metalMachine.Status.SetFailure(err.Error(), clustererr.CreateMachineError)
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
func (r *MetalStackMachineReconciler) newRequestToCreateMachine(rsrc *resource) (*metalgo.MachineCreateRequest, error) {
	name := rsrc.metalMachine.Name
	networks := toNetworks(*rsrc.metalCluster.Spec.Firewall.DefaultNetworkID, *rsrc.metalCluster.Spec.PrivateNetworkID)
	userData, err := r.bootstrapData(rsrc)
	// todo: Remove this
	log.Println("userData: ", string(userData))
	if err != nil {
		return nil, err
	}
	// todo: Remove this
	log.Printf("creating a machine with IP: %v", rsrc.metalCluster.Spec.ControlPlaneEndpoint.Host)
	return &metalgo.MachineCreateRequest{
		Hostname:  name,
		Image:     rsrc.metalMachine.Spec.Image,
		IPs:       []string{rsrc.metalCluster.Spec.ControlPlaneEndpoint.Host},
		Name:      name,
		Networks:  networks,
		Partition: *rsrc.metalCluster.Spec.Partition,
		Project:   *rsrc.metalCluster.Spec.ProjectID,
		Size:      rsrc.metalMachine.Spec.MachineType,
		Tags:      rsrc.machineCreationTags(),
		UserData:  string(userData),
	}, nil
}
func (r *MetalStackMachineReconciler) setMachineStatus(rsrc *resource, rawMachine *models.V1MachineResponse) {
	// todo: Shoul each status be updated separately?
	rsrc.metalMachine.Spec.SetProviderID(*rawMachine.ID)
	rsrc.metalMachine.Status.Liveliness = rawMachine.Liveliness
	rsrc.metalMachine.Status.Addresses = toNodeAddrs(rawMachine)
}
func toNodeAddrs(machine *models.V1MachineResponse) []core.NodeAddress {
	addrs := []core.NodeAddress{}
	for _, n := range machine.Allocation.Networks {
		t := core.NodeExternalIP
		if *n.Private {
			t = core.NodeInternalIP
		}
		addrs = append(addrs, core.NodeAddress{
			Type:    t,
			Address: n.Ips[0],
		})
	}
	return addrs
}
