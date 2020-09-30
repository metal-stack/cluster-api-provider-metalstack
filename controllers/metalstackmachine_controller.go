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
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metalstack "github.com/metal-stack/cluster-api-provider-metalstack/pkg/cloud/metalstack"
	"github.com/metal-stack/cluster-api-provider-metalstack/pkg/cloud/metalstack/scope"

	infrastructurev1alpha3 "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
)

// MetalStackMachineReconciler reconciles a MetalStackMachine object
type MetalStackMachineReconciler struct {
	client.Client
	Log              logr.Logger
	Recorder         record.EventRecorder
	Scheme           *runtime.Scheme
	MetalStackClient *metalstack.MetalStackClient
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *MetalStackMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	logger := r.Log.WithValues("metalstackmachine", req.NamespacedName)

	// your logic here
	metalstackmachine := &infrastructurev1alpha3.MetalStackMachine{}
	if err := r.Get(ctx, req.NamespacedName, metalstackmachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger = logger.WithName(metalstackmachine.APIVersion)

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, metalstackmachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		logger.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	if util.IsPaused(cluster, machine) {
		logger.Info("MetalStackMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	metalstackcluster := &infrastructurev1alpha3.MetalStackCluster{}
	metalstackclusterNamespacedName := client.ObjectKey{
		Namespace: metalstackmachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, metalstackclusterNamespacedName, metalstackcluster); err != nil {
		logger.Info("MetalStackCluster is not available yet")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("metalstackcluster", metalstackcluster.Name)

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:            r.Client,
		Logger:            logger,
		Cluster:           cluster,
		MetalStackCluster: metalstackcluster,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Logger:            logger,
		Client:            r.Client,
		Cluster:           cluster,
		Machine:           machine,
		MetalStackCluster: metalstackcluster,
		MetalStackMachine: metalstackmachine,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any MetalStackMachine changes.
	defer func() {
		if err := machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !metalstackmachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope, clusterScope, logger)
	}

	return r.reconcile(ctx, machineScope, clusterScope, logger)
}

func (r *MetalStackMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha3.MetalStackMachine{}).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrastructurev1alpha3.GroupVersion.WithKind("MetalStackMachine")),
			},
		).
		Complete(r)
}

func (r *MetalStackMachineReconciler) reconcile(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Reconciling MetalStackMachine")
	metalstackmachine := machineScope.MetalStackMachine
	// If the MetalStackMachine is in an error state, return early.
	// if metalstackmachine.Status.ErrorReason != nil || metalstackmachine.Status.ErrorMessage != nil {
	// 	machineScope.Info("Error state detected, skipping reconciliation")
	// 	return ctrl.Result{}, nil
	// }

	// If the MetalStackMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(metalstackmachine, infrastructurev1alpha3.MachineFinalizer)

	if !machineScope.Cluster.Status.InfrastructureReady {
		machineScope.Info("Cluster infrastructure is not ready yet")
		return ctrl.Result{}, nil
	}

	// Make sure bootstrap data secret is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		machineScope.Info("Bootstrap data secret is not yet available")
		return ctrl.Result{}, nil
	}

	providerID := machineScope.GetInstanceID()
	var (
		machine *models.V1MachineResponse
		err     error
	)
	// if we have no provider ID, then we are creating
	if providerID != "" {
		mr, err := r.MetalStackClient.GetMachine(providerID)
		if err != nil {
			return ctrl.Result{}, err
		}
		machine = mr.Machine
	}

	privateNetwork := clusterScope.MetalStackCluster.Spec.PrivateNetworkID
	if privateNetwork == "" {
		nwID, err := r.MetalStackClient.AllocatePrivateNetwork(
			clusterScope.Cluster.ClusterName,
			*clusterScope.MetalStackCluster.Spec.ProjectID,
			*machineScope.MetalStackCluster.Spec.Partition)
		if err != nil {
			return ctrl.Result{}, err
		}
		machineScope.SetPrivateNetworkID(nwID)
		privateNetwork = nwID
	}

	if machine == nil || machine.ID == nil {
		// generate a unique UID that will survive pivot, i.e. is not tied to the cluster itself
		mUID := uuid.New().String()
		tags := []string{
			metalstack.GenerateMachineTag(mUID),
			metalstack.GenerateClusterTag(clusterScope.Name()),
		}

		tags = append(tags, machineScope.MetalStackMachine.Spec.Tags...)

		name := machineScope.Name()

		networks := []metalgo.MachineAllocationNetwork{
			{NetworkID: privateNetwork, Autoacquire: true},
		}
		for _, additionalNetwork := range machineScope.MetalStackCluster.Spec.AdditionalNetworks {
			anw := metalgo.MachineAllocationNetwork{
				NetworkID:   additionalNetwork,
				Autoacquire: true,
			}
			networks = append(networks, anw)
		}
		if s := machineScope.MetalStackMachine.Spec.Image; strings.Contains(s, "firewall") {
			networks = append(networks, metalgo.MachineAllocationNetwork{
				NetworkID:   "internet-vagrant-lab",
				Autoacquire: true,
			})
		}

		userDataRaw, err := machineScope.GetRawBootstrapData()
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "impossible to retrieve bootstrap data from secret")
		}
		userData := string(userDataRaw)

		mcr := metalgo.MachineCreateRequest{
			Name:      name,
			Hostname:  name,
			Project:   *clusterScope.MetalStackCluster.Spec.ProjectID,
			Partition: *machineScope.MetalStackCluster.Spec.Partition,
			Image:     machineScope.MetalStackMachine.Spec.Image,
			Networks:  networks,
			Size:      machineScope.MetalStackMachine.Spec.MachineType,
			Tags:      tags,
			UserData:  userData,
		}
		response, err := r.MetalStackClient.MachineCreate(&mcr)
		if err != nil {
			machineScope.SetErrorReason(capierrors.CreateMachineError)
			machineScope.SetErrorMessage(err)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 3 * time.Second,
			}, fmt.Errorf("failed to create a machine: %v", err)
		}
		machine = response.Machine
	}

	// we do not need to set this as metalstack://<id> because SetProviderID() does the formatting for us
	machineScope.SetProviderID(*machine.ID)
	machineScope.SetInstanceStatus(infrastructurev1alpha3.MetalStackResourceStatus(*machine.Liveliness))

	addrs, err := r.MetalStackClient.GetMachineAddresses(machine)
	if err != nil {
		machineScope.SetErrorMessage(errors.New("failed to getting machine addresses"))
		return ctrl.Result{}, err
	}
	machineScope.SetAddresses(addrs)

	// Proceed to reconcile the MetalStackMachine state.
	var result ctrl.Result

	// FIXME match Liveleness with MetalStackResourceStatus
	switch infrastructurev1alpha3.MetalStackResourceStatus(*machine.Liveliness) {
	case infrastructurev1alpha3.MetalStackResourceStatusNew, infrastructurev1alpha3.MetalStackResourceStatusQueued, infrastructurev1alpha3.MetalStackResourceStatusProvisioning:
		machineScope.Info("Machine instance is pending", "instance-id", machineScope.GetInstanceID())
		result = ctrl.Result{RequeueAfter: 10 * time.Second}
	case infrastructurev1alpha3.MetalStackResourceStatusRunning:
		machineScope.Info("Machine instance is active", "instance-id", machineScope.GetInstanceID())
		machineScope.SetReady()
		result = ctrl.Result{}
	default:
		machineScope.SetErrorReason(capierrors.UpdateMachineError)
		machineScope.SetErrorMessage(errors.Errorf("Instance status %q is unexpected", *machine.Liveliness))
		result = ctrl.Result{}
	}

	return result, nil
}

func (r *MetalStackMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Deleting machine")
	metalstackmachine := machineScope.MetalStackMachine
	providerID := machineScope.GetInstanceID()
	if providerID == "" {
		logger.Info("no provider ID provided, nothing to delete")
		controllerutil.RemoveFinalizer(metalstackmachine, infrastructurev1alpha3.MachineFinalizer)
		return ctrl.Result{}, nil
	}

	machine, err := r.MetalStackClient.GetMachine(providerID)
	if err != nil {
		// FIXME check not found
		// if err  {
		// 	//		if err.(*packngo.ErrorResponse).Response != nil && err.(*packngo.ErrorResponse).Response.StatusCode == http.StatusNotFound {
		// 	// When the server does not exist we do not have anything left to do.
		// 	// Probably somebody manually deleted the server from the UI or via API.
		// 	logger.Info("Server not found, nothing left to do")
		// 	controllerutil.RemoveFinalizer(metalstackmachine, infrastructurev1alpha3.MachineFinalizer)
		// 	return ctrl.Result{}, nil
		// }
		return ctrl.Result{}, fmt.Errorf("error retrieving machine status %s: %v", metalstackmachine.Name, err)
	}

	// We should never get there but this is a safetly check
	if machine == nil {
		controllerutil.RemoveFinalizer(metalstackmachine, infrastructurev1alpha3.MachineFinalizer)
		return ctrl.Result{}, fmt.Errorf("machine does not exist: %s", metalstackmachine.Name)
	}

	_, err = r.MetalStackClient.MachineDelete(*machine.Machine.ID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete the machine: %v", err)
	}

	controllerutil.RemoveFinalizer(metalstackmachine, infrastructurev1alpha3.MachineFinalizer)
	return ctrl.Result{}, nil
}
