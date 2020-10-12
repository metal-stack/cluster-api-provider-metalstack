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

	metal "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
)

const (
	// MachineFinalizer is the finalizer for MetalStackMachine.
	MachineFinalizer = "metalstackmachine.infrastructure.cluster.x-k8s.io"
)

// todo: Add constructor.
// MetalStackMachineReconciler reconciles a MetalStackMachine object
type MetalStackMachineReconciler struct {
	client.Client
	Log         logr.Logger
	MetalClient *metalgo.Driver
	Recorder    record.EventRecorder
	Scheme      *runtime.Scheme
}

type resource struct {
	cluster      *cluster.Cluster
	machine      *cluster.Machine
	metalCluster *metal.MetalStackCluster
	metalMachine *metal.MetalStackMachine
}

func newResource(
	cl *cluster.Cluster,
	m *cluster.Machine,
	metalCl *metal.MetalStackCluster,
	metalM *metal.MetalStackMachine,
) *resource {
	return &resource{
		cluster:      cl,
		machine:      m,
		metalCluster: metalCl,
		metalMachine: metalM,
	}
}

func (rsrc *resource) toMetalTags() []string {
	tags := append([]string{
		"cluster-api-provider-metalstack:machine-uid:" + uuid.New().String(),
		clusterIDTag + rsrc.metalCluster.Name,
	}, rsrc.metalMachine.Spec.Tags...)
	if util.IsControlPlaneMachine(rsrc.machine) {
		tags = append(tags, cluster.MachineControlPlaneLabelName+":true")
	}
	tags = append(tags, cluster.MachineControlPlaneLabelName+":false")

	return tags
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *MetalStackMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, err error) {
	logger := r.Log.WithName("MetalStackMachine").WithValues("namespace", req.Namespace, "name", req.Name)
	ctx := context.Background()

	// Fetch the MetalStackMachine.
	metalMachine := &metal.MetalStackMachine{}
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
	logger = logger.WithName("Machine").WithValues("name", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info(err.Error())
		// todo: Return err or nil?
		return ctrl.Result{}, nil
	}
	logger = logger.WithName("Cluster").WithValues("name", cluster.Name)

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
	metalCluster := &metal.MetalStackCluster{}
	if err := r.Get(ctx, k, metalCluster); err != nil {
		logger.Info(err.Error())
		return ctrl.Result{}, nil
	}
	logger = logger.WithName("MetalStackCluster").WithValues("name", metalCluster.Name)

	rsrc := newResource(cluster, machine, metalCluster, metalMachine)

	// todo: See if the updates of API resources can be made explicitly.
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
		return r.deleteMachine(ctx, rsrc)
	}

	// todo: Check if the failure still holds after some time.
	// todo: Check the logic of failure. It should be Idempotent.
	if rsrc.metalMachine.Status.Failed() {
		logger.Info("the Status of the MetalStackMachine showing failure")
		return ctrl.Result{}, nil
	}

	r.addMachineFinalizer(rsrc)

	// todo: Better the logic. The reconciliation of the MetalStackCluster isn't done yet.
	if !rsrc.cluster.Status.InfrastructureReady {
		logger.Info("the network not ready")
		return ctrl.Result{}, nil
	}

	if !rsrc.machine.Status.BootstrapReady {
		logger.Info("the bootstrap provider not ready")
		return ctrl.Result{}, nil
	}

	rawMachine := &models.V1MachineResponse{}
	ID, err := rsrc.metalMachine.Spec.ParsedProviderID()
	if err != nil {
		switch err.(type) {
		case nil:
			logger.Info("failed to parse ProviderID of the MetalStackMachine")
			return ctrl.Result{}, nil
		case *metal.ErrorProviderIDNotSet:
			logger.Info("ProviderID ot the MetalStackMachine not set")
			req, err := r.newRequestToCreateMachine(rsrc)
			if err != nil {
				logger.Info("failed to create a new machine-creation-request")
			}
			resp, err := r.MetalClient.MachineCreate(req)
			if err != nil {
				logger.Info("failed to create the MetalStackMachine")
				rsrc.metalMachine.Status.SetFailure(err.Error(), clustererr.CreateMachineError)
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: 30 * time.Second,
				}, err
			}
			rawMachine = resp.Machine
			ID = *rawMachine.ID
		default:
			logger.Info("unknown errors")
			return ctrl.Result{}, err
		}
	}
	resp, err := r.MetalClient.MachineGet(ID)
	if err != nil {
		logger.Error(err, "failed to get the MetalStackMachine with the ID %v", ID)
		return ctrl.Result{}, err
	}
	rawMachine = resp.Machine

	r.setMachineStatus(rsrc, rawMachine)

	return ctrl.Result{}, nil
}

func (r *MetalStackMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metal.MetalStackMachine{}).
		Watches(
			&source.Kind{Type: &cluster.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(metal.GroupVersion.WithKind("MetalStackMachine")),
			},
		).
		Complete(r)
}

func (r *MetalStackMachineReconciler) addMachineFinalizer(rsrc *resource) {
	controllerutil.AddFinalizer(rsrc.metalMachine, MachineFinalizer)
}

// todo: duplicate
func (r *MetalStackMachineReconciler) allocatePrivateNetwork(rsrc *resource) (*string, error) {
	resp, err := r.MetalClient.NetworkAllocate(&metalgo.NetworkAllocateRequest{
		ProjectID:   *rsrc.metalCluster.Spec.ProjectID,
		PartitionID: *rsrc.metalCluster.Spec.Partition,
		Name:        *rsrc.metalCluster.Spec.Partition,
		Description: rsrc.metalCluster.Name,
		Labels:      map[string]string{tag.ClusterID: rsrc.metalCluster.Name},
	})
	if err != nil {
		return nil, err
	}
	return resp.Network.ID, nil
}

func (r *MetalStackMachineReconciler) deleteMachine(ctx context.Context, rsrc *resource) (ctrl.Result, error) {
	r.Log.Info("the MetalStackMachine being deleted")

	id, err := rsrc.metalMachine.Spec.ParsedProviderID()
	if err != nil {
		r.Log.Error(err, "failed to parse the ProviderID of the MetalStackMachine")
		return ctrl.Result{}, err
	}

	if _, err = r.MetalClient.MachineDelete(id); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete the MetalStackMachine %v: %v", rsrc.metalMachine.Name, err)
	}

	r.removeMachineFinalizer(rsrc)

	return ctrl.Result{}, nil
}

// todo: Return err?
func (r *MetalStackMachineReconciler) newRequestToCreateMachine(rsrc *resource) (*metalgo.MachineCreateRequest, error) {
	name := rsrc.metalMachine.Name

	// todo: We should have had the private network ID in Cluster's spec.
	privateNID := rsrc.metalCluster.Spec.PrivateNetworkID
	if privateNID == nil {
		nID, err := r.allocatePrivateNetwork(rsrc)
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
	networks := toMetalNetworks(append(rsrc.metalCluster.Spec.AdditionalNetworks, *privateNID)...)
	if i := rsrc.metalMachine.Spec.Image; strings.Contains(i, "firewall") {
		networks = append(networks, metalgo.MachineAllocationNetwork{
			NetworkID:   "internet-vagrant-lab",
			Autoacquire: true,
		})
	}

	// todo: Add the logic of UserData.

	return &metalgo.MachineCreateRequest{
		Hostname:  name,
		Image:     rsrc.metalMachine.Spec.Image,
		Name:      name,
		Networks:  networks,
		Partition: *rsrc.metalCluster.Spec.Partition,
		Project:   *rsrc.metalCluster.Spec.ProjectID,
		Size:      rsrc.metalMachine.Spec.MachineType,
		Tags:      rsrc.toMetalTags(),
		UserData:  "",
	}, nil
}

func (r *MetalStackMachineReconciler) removeMachineFinalizer(rsrc *resource) {
	controllerutil.RemoveFinalizer(rsrc.metalMachine, MachineFinalizer)
}

func (r *MetalStackMachineReconciler) setMachineStatus(rsrc *resource, rawMachine *models.V1MachineResponse) {
	// todo: Shift to machine creation.
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
