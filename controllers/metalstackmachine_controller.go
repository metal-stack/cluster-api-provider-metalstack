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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/pkg/errors"

	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	cluster "sigs.k8s.io/cluster-api/api/v1alpha3"
	clustererr "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mst "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
)

const (
	// MStMachineFinalizer is the finalizer for the MetalStackMachine.
	MStMachineFinalizer = "metalstackmachine.infrastructure.cluster.x-k8s.io"
)

// MetalStackMachineReconciler reconciles a MetalStackMachine object
type MetalStackMachineReconciler struct {
	client.Client
	Log       logr.Logger
	MStClient *metalgo.Driver
	Recorder  record.EventRecorder
	Scheme    *runtime.Scheme
	// MetalStackClient *metalstack.MetalStackClient

	cluster    *cluster.Cluster
	machine    *cluster.Machine
	mstCluster *mst.MetalStackCluster
	mstMachine *mst.MetalStackMachine
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *MetalStackMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, err error) {
	logger := r.Log.WithName("MetalStackMachine").WithValues("namespace", req.Namespace, "name", req.Name)

	// Fetch the MetalStackMachine.
	ctx := context.Background()
	if err := r.Get(ctx, req.NamespacedName, r.mstMachine); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info(err.Error())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	r.machine, err = util.GetOwnerMachine(ctx, r.Client, r.mstMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if r.machine == nil {
		logger.Info("no OwnerReference of the MetalStackMachine has the Kind Machine")
		return ctrl.Result{}, nil
	}
	logger = logger.WithName("Machine").WithValues("name", r.machine.Name)

	// Fetch the Cluster.
	r.cluster, err = util.GetClusterFromMetadata(ctx, r.Client, r.machine.ObjectMeta)
	if err != nil {
		logger.Info(err.Error())
		// todo: Return err or nil?
		return ctrl.Result{}, nil
	}
	logger = logger.WithName("Cluster").WithValues("name", r.cluster.Name)

	// Return if the retrieved objects are paused.
	if util.IsPaused(r.cluster, r.mstMachine) {
		logger.Info("the Cluster is paused or the MetalStackMachine has the `paused` annotation")
		return ctrl.Result{}, nil
	}

	// Fetch the MetalStackCluster.
	k := client.ObjectKey{
		Namespace: r.mstMachine.Namespace,
		Name:      r.cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, k, r.mstCluster); err != nil {
		logger.Info(err.Error())
		return ctrl.Result{}, nil
	}
	logger = logger.WithName("MetalStackCluster").WithValues("name", r.mstCluster.Name)

	// Persist any change of the MetalStackMachine.
	h, err := patch.NewHelper(r.mstMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if e := h.Patch(ctx, r.mstCluster); e != nil {
			if err != nil {
				err = errors.Wrap(e, err.Error())
			}
			err = e
		}
	}()

	if !r.mstMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.deleteMStMachine(ctx)
	}

	if r.mstMachine.Status.Failed() {
		logger.Info("the Status of the MetalStackMachine showing failure")
		return ctrl.Result{}, nil
	}

	r.addMStMachineFinalizer()

	// todo: Better the logic. The reconciliation of the MetalStackCluster isn't done yet.
	if !r.cluster.Status.InfrastructureReady {
		logger.Info("the network not ready")
		return ctrl.Result{}, nil
	}

	if !r.machine.Status.BootstrapReady {
		logger.Info("the bootstrap provider not ready")
		return ctrl.Result{}, nil
	}

	rawMachine := &models.V1MachineResponse{}
	ID, err := r.mstMachine.Spec.ParsedProviderID()
	if err != nil {
		switch err.(type) {
		case nil:
			logger.Info("failed to parse ProviderID of the MetalStackMachine")
			return ctrl.Result{}, nil
		case *mst.ErrorProviderIDNotSet:
			logger.Info("ProviderID ot the MetalStackMachine not set")
			req, err := r.newRequestToCreateMachine()
			if err != nil {
				logger.Info("failed to create a new machine-creation-request")
			}
			resp, err := r.MStClient.MachineCreate(req)
			if err != nil {
				logger.Info("failed to create the MetalStackMachine")
				r.mstMachine.Status.SetFailure(err.Error(), clustererr.CreateMachineError)
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: 3 * time.Second,
				}, err
			}
			rawMachine = resp.Machine
		}
	}
	resp, err := r.MStClient.MachineGet(ID)
	if err != nil {
		logger.Error(err, "failed to get the MetalStackMachine with the ID %v", ID)
		return ctrl.Result{}, err
	}
	rawMachine = resp.Machine

	r.setMStMachineStatus(rawMachine)

	return ctrl.Result{}, nil
}

func (r *MetalStackMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mst.MetalStackMachine{}).
		Watches(
			&source.Kind{Type: &cluster.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(mst.GroupVersion.WithKind("MetalStackMachine")),
			},
		).
		Complete(r)
}

func (r *MetalStackMachineReconciler) addMStMachineFinalizer() {
	controllerutil.AddFinalizer(r.mstMachine, MStMachineFinalizer)
}

// todo: duplicate
func (r *MetalStackMachineReconciler) allocatePrivateNetwork() (*string, error) {
	resp, err := r.MStClient.NetworkAllocate(&metalgo.NetworkAllocateRequest{
		ProjectID:   *r.mstCluster.Spec.ProjectID,
		PartitionID: *r.mstCluster.Spec.Partition,
		Name:        *r.mstCluster.Spec.Partition,
		Description: r.mstCluster.Name,
		Labels:      map[string]string{tag.ClusterID: r.mstCluster.Name},
	})
	if err != nil {
		return nil, err
	}
	return resp.Network.ID, nil
}

func (r *MetalStackMachineReconciler) deleteMStMachine(ctx context.Context) (ctrl.Result, error) {
	r.Log.Info("the MetalStackMachine being deleted")

	defer r.removeMStMachineFinalizer()

	ID, err := r.mstMachine.Spec.ParsedProviderID()
	if err != nil {
		r.Log.Error(err, "failed to parse the ProviderID of the MetalStackMachine")
		return ctrl.Result{}, err
	}

	if _, err = r.MStClient.MachineDelete(ID); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete the MetalStackMachine %v: %v", r.mstMachine.Name, err)
	}
	return ctrl.Result{}, nil
}

func (r *MetalStackMachineReconciler) newRequestToCreateMachine() (*metalgo.MachineCreateRequest, error) {
	name := r.mstMachine.Name

	privateNID := r.mstCluster.Spec.PrivateNetworkID
	if privateNID == nil {
		nID, err := r.allocatePrivateNetwork()
		if err != nil {
			r.Log.Info("failed to allocate a private network")
			return nil, err
		}
		if nID == nil {
			s := "nil as private network ID"
			r.Log.Info(s)
			return nil, errors.New(s)
		}
		privateNID = nID
	}
	networks := toMStNetworks(append(r.mstCluster.Spec.AdditionalNetworks, *privateNID)...)
	if i := r.mstMachine.Spec.Image; strings.Contains(i, "firewall") {
		networks = append(networks, metalgo.MachineAllocationNetwork{
			NetworkID:   "internet-vagrant-lab",
			Autoacquire: true,
		})
	}

	tags := append([]string{
		"cluster-api-provider-metalstack:machine-uid:" + uuid.New().String(),
		"cluster-api-provider-metalstack:cluster-id:" + r.mstCluster.Name,
	}, r.mstMachine.Spec.Tags...)

	// todo: Add the logic of UserData

	return &metalgo.MachineCreateRequest{
		Hostname:  name,
		Image:     r.mstMachine.Spec.Image,
		Name:      name,
		Networks:  networks,
		Partition: *r.mstCluster.Spec.Partition,
		Project:   *r.mstCluster.Spec.ProjectID,
		Size:      r.mstMachine.Spec.MachineType,
		Tags:      tags,
		UserData:  "",
	}, nil
}

func (r *MetalStackMachineReconciler) removeMStMachineFinalizer() {
	controllerutil.RemoveFinalizer(r.mstMachine, MStMachineFinalizer)
}

func (r *MetalStackMachineReconciler) setMStMachineStatus(rawMachine *models.V1MachineResponse) {
	r.mstMachine.Spec.SetProviderID(*rawMachine.ID)
	r.mstMachine.Status.Liveliness = rawMachine.Liveliness
	r.mstMachine.Status.Addresses = toNodeAddrs(rawMachine)
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
