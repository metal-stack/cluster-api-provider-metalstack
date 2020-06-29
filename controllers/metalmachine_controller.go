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
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	metalgo "github.com/metal-stack/metal-go"
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

	metal "github.com/metal-stack/cluster-api-provider-metal/pkg/cloud/metal"
	"github.com/metal-stack/cluster-api-provider-metal/pkg/cloud/metal/scope"

	infrastructurev1alpha3 "github.com/metal-stack/cluster-api-provider-metal/api/v1alpha3"
)

const (
	providerName = "metal"
)

// MetalMachineReconciler reconciles a MetalMachine object
type MetalMachineReconciler struct {
	client.Client
	Log         logr.Logger
	Recorder    record.EventRecorder
	Scheme      *runtime.Scheme
	MetalClient *metal.MetalClient
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *MetalMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	logger := r.Log.WithValues("metalmachine", req.NamespacedName)

	// your logic here
	metalmachine := &infrastructurev1alpha3.MetalMachine{}
	if err := r.Get(ctx, req.NamespacedName, metalmachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger = logger.WithName(metalmachine.APIVersion)

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, metalmachine.ObjectMeta)
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
		logger.Info("MetalMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	metalcluster := &infrastructurev1alpha3.MetalCluster{}
	metalclusterNamespacedName := client.ObjectKey{
		Namespace: metalmachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, metalclusterNamespacedName, metalcluster); err != nil {
		logger.Info("MetalCluster is not available yet")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("metalcluster", metalcluster.Name)

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:       r.Client,
		Logger:       logger,
		Cluster:      cluster,
		MetalCluster: metalcluster,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Logger:       logger,
		Client:       r.Client,
		Cluster:      cluster,
		Machine:      machine,
		MetalCluster: metalcluster,
		MetalMachine: metalmachine,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any MetalMachine changes.
	defer func() {
		if err := machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !metalmachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope, clusterScope, logger)
	}

	return r.reconcile(ctx, machineScope, clusterScope, logger)
}

func (r *MetalMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha3.MetalMachine{}).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrastructurev1alpha3.GroupVersion.WithKind("MetalMachine")),
			},
		).
		Complete(r)
}

func (r *MetalMachineReconciler) reconcile(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Reconciling MetalMachine")
	metalmachine := machineScope.MetalMachine
	// If the MetalMachine is in an error state, return early.
	if metalmachine.Status.ErrorReason != nil || metalmachine.Status.ErrorMessage != nil {
		machineScope.Info("Error state detected, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// If the MetalMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(metalmachine, infrastructurev1alpha3.MachineFinalizer)

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
		machine *metalgo.MachineGetResponse
		err     error
	)
	// if we have no provider ID, then we are creating
	if providerID != "" {
		machine, err = r.MetalClient.GetMachine(providerID)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	if machine == nil {
		// generate a unique UID that will survive pivot, i.e. is not tied to the cluster itself
		mUID := uuid.New().String()
		tags := []string{
			metal.GenerateMachineTag(mUID),
			metal.GenerateClusterTag(clusterScope.Name()),
		}

		name := machineScope.Name()
		//name, clusterScope.MetalCluster.Spec.ProjectID, machineScope, tags
		mcr := metalgo.MachineCreateRequest{
			Name:     name,
			Hostname: name,
			Project:  clusterScope.MetalCluster.Spec.ProjectID,
			Tags:     tags,
		}
		machine, err := r.MetalClient.MachineCreate(&mcr)
		if err != nil {
			errs := fmt.Errorf("failed to create machine %s %s: %v", *machine.Machine.ID, name, err)
			machineScope.SetErrorReason(capierrors.CreateMachineError)
			machineScope.SetErrorMessage(errs)
			return ctrl.Result{}, errs
		}
	}

	// we do not need to set this as metal://<id> because SetProviderID() does the formatting for us
	machineScope.SetProviderID(*machine.Machine.ID)
	machineScope.SetInstanceStatus(infrastructurev1alpha3.MetalResourceStatus(*machine.Machine.Liveliness))

	addrs, err := r.MetalClient.GetMachineAddresses(machine)
	if err != nil {
		machineScope.SetErrorMessage(errors.New("failed to getting machine addresses"))
		return ctrl.Result{}, err
	}
	machineScope.SetAddresses(addrs)

	// Proceed to reconcile the MetalMachine state.
	var result = ctrl.Result{}

	switch infrastructurev1alpha3.MetalResourceStatus(*machine.Machine.Liveliness) {
	case infrastructurev1alpha3.MetalResourceStatusNew, infrastructurev1alpha3.MetalResourceStatusQueued, infrastructurev1alpha3.MetalResourceStatusProvisioning:
		machineScope.Info("Machine instance is pending", "instance-id", machineScope.GetInstanceID())
		result = ctrl.Result{RequeueAfter: 10 * time.Second}
	case infrastructurev1alpha3.MetalResourceStatusRunning:
		machineScope.Info("Machine instance is active", "instance-id", machineScope.GetInstanceID())
		machineScope.SetReady()
		result = ctrl.Result{}
	default:
		machineScope.SetErrorReason(capierrors.UpdateMachineError)
		machineScope.SetErrorMessage(errors.Errorf("Instance status %q is unexpected", *machine.Machine.Liveliness))
		result = ctrl.Result{}
	}

	return result, nil
}

func (r *MetalMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Deleting machine")
	metalmachine := machineScope.MetalMachine
	providerID := machineScope.GetInstanceID()
	if providerID == "" {
		logger.Info("no provider ID provided, nothing to delete")
		controllerutil.RemoveFinalizer(metalmachine, infrastructurev1alpha3.MachineFinalizer)
		return ctrl.Result{}, nil
	}

	machine, err := r.MetalClient.GetMachine(providerID)
	if err != nil {
		// FIXME check not found
		// if err  {
		// 	//		if err.(*packngo.ErrorResponse).Response != nil && err.(*packngo.ErrorResponse).Response.StatusCode == http.StatusNotFound {
		// 	// When the server does not exist we do not have anything left to do.
		// 	// Probably somebody manually deleted the server from the UI or via API.
		// 	logger.Info("Server not found, nothing left to do")
		// 	controllerutil.RemoveFinalizer(metalmachine, infrastructurev1alpha3.MachineFinalizer)
		// 	return ctrl.Result{}, nil
		// }
		return ctrl.Result{}, fmt.Errorf("error retrieving machine status %s: %v", metalmachine.Name, err)
	}

	// We should never get there but this is a safetly check
	if machine == nil {
		controllerutil.RemoveFinalizer(metalmachine, infrastructurev1alpha3.MachineFinalizer)
		return ctrl.Result{}, fmt.Errorf("machine does not exist: %s", metalmachine.Name)
	}

	_, err = r.MetalClient.MachineDelete(*machine.Machine.ID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete the machine: %v", err)
	}

	controllerutil.RemoveFinalizer(metalmachine, infrastructurev1alpha3.MachineFinalizer)
	return ctrl.Result{}, nil
}
