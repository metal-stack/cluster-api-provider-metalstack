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
	"time"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/pkg/errors"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
)

const (
	clusterIDTag = "cluster-api-provider-metalstack:cluster-id"
)

// MetalStackClusterReconciler reconciles a MetalStackCluster object
type MetalStackClusterReconciler struct {
	client.Client

	Log       logr.Logger
	MStClient *metalgo.Driver
	Recorder  record.EventRecorder
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metalstackclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *MetalStackClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, err error) {
	ctx := context.Background()
	logger := r.Log.WithValues("MetalStackCluster", req.NamespacedName)

	mstCluster := &v1alpha3.MetalStackCluster{}
	if err := r.Get(ctx, req.NamespacedName, mstCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger = logger.WithName(mstCluster.APIVersion)

	cluster, err := util.GetOwnerCluster(ctx, r.Client, mstCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	if cluster == nil {
		logger.Info("OwnerCluster is not set yet. The reconciliation request will be requeued.")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 2 * time.Second,
		}, nil
	}

	if util.IsPaused(cluster, mstCluster) {
		logger.Info("MetalStackCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	h, err := patch.NewHelper(mstCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to initialize patch.Helper")
	}
	defer func() {
		if e := h.Patch(context.TODO(), mstCluster); e != nil {
			if err != nil {
				err = errors.Wrap(e, err.Error())
			}
			err = e
		}
	}()

	if mstCluster.Spec.PrivateNetworkID == nil {
		networkID, err := r.allocateNetwork(mstCluster)
		if err != nil {
			logger.Error(err, "no response to network allocation")
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Second}, err
		}
		mstCluster.Spec.PrivateNetworkID = networkID
	}

	if !mstCluster.Status.FirewallReady {
		err = r.createFirewall(mstCluster)
		if err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: 2 * time.Second}, err
		}
		logger.Info("A firewall was created.")
		mstCluster.Status.FirewallReady = true
	}

	mstCluster.Status.Ready = true

	ip, err := r.getControlPlaneIP(mstCluster)
	if err != nil {
		switch err.(type) {
		case *MachineNotFound, *MachineNoIP: // todo: Do we really need these two types?
			logger.Info(err.Error() + " Control plane machine not found. Requeueing...")
			return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
		default:
			logger.Info(err.Error() + " error getting a control plane ip")
			return ctrl.Result{}, err
		}
	}
	mstCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: ip,
		Port: 6443,
	}

	return ctrl.Result{}, nil
}

func (r *MetalStackClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha3.MetalStackCluster{}).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(v1alpha3.GroupVersion.WithKind("MetalStackCluster")),
			},
		).
		Complete(r)
}

func (r *MetalStackClusterReconciler) allocateNetwork(mstCluster *v1alpha3.MetalStackCluster) (*string, error) {
	log.Println(mstCluster)
	resp, err := r.MStClient.NetworkAllocate(&metalgo.NetworkAllocateRequest{
		ProjectID:   *mstCluster.Spec.ProjectID,
		PartitionID: *mstCluster.Spec.Partition,
		Name:        *mstCluster.Spec.Partition,
		Description: mstCluster.Name,
		Labels:      map[string]string{tag.ClusterID: mstCluster.Name},
	})
	if err != nil {
		return nil, err
	}
	return resp.Network.ID, nil
}

func (r *MetalStackClusterReconciler) createFirewall(mstCluster *v1alpha3.MetalStackCluster) error {
	req := &metalgo.FirewallCreateRequest{
		MachineCreateRequest: metalgo.MachineCreateRequest{
			Description:   mstCluster.Name + " created by Cluster API provider MetalStack",
			Name:          mstCluster.Name,
			Hostname:      mstCluster.Name + "-firewall",
			Size:          "v1-small-x86",
			Project:       *mstCluster.Spec.ProjectID,
			Partition:     *mstCluster.Spec.Partition,
			Image:         "firewall-ubuntu-2.0",
			SSHPublicKeys: []string{},
			Networks:      toMStNetworks("internet-vagrant-lab", *mstCluster.Spec.PrivateNetworkID),
			UserData:      "",
			Tags:          []string{""},
		},
	}

	_, err := r.MStClient.FirewallCreate(req)
	return err
}

func (r *MetalStackClusterReconciler) getControlPlaneIP(mstCluster *v1alpha3.MetalStackCluster) (string, error) {
	if mstCluster == nil {
		return "", fmt.Errorf("cannot get IP of machine in nil cluster")
	}
	tags := []string{
		fmt.Sprintf("%s:%s", clusterIDTag, mstCluster.Name),
		v1alpha3.MasterTag,
	}

	mm, err := r.MStClient.MachineFind(&metalgo.MachineFindRequest{AllocationProject: mstCluster.Spec.ProjectID, Tags: tags})

	if err != nil {
		return "", fmt.Errorf("Error retrieving machines: %v", err)
	}

	if mm == nil {
		return "", &MachineNotFound{fmt.Sprintf("A machine with the project ID %s and tags %v doesn't exist.", *mstCluster.Spec.ProjectID, tags)}
	}

	if len(mm.Machines) != 1 {
		return "", &MachineNotFound{fmt.Sprintf("%v machine(s) found", len(mm.Machines))}
	}
	m := mm.Machines[0]

	if m.Allocation == nil || len(m.Allocation.Networks) == 0 || len(m.Allocation.Networks[0].Ips) == 0 || m.Allocation.Networks[0].Ips[0] == "" {
		return "", &MachineNoIP{"The machine doesn't have an IP address."}
	}
	return m.Allocation.Networks[0].Ips[0], nil
}

func (r *MetalStackClusterReconciler) newHelper(c *v1alpha3.MetalStackCluster) (*patch.Helper, error) {
	return patch.NewHelper(c, r.Client)
}

// MachineNotFound error representing that the requested machine was not yet found
type MachineNotFound struct {
	s string
}

func (e *MachineNotFound) Error() string {
	return e.s
}

// MachineNoIP error representing that the requested machine does not have an IP yet assigned
type MachineNoIP struct {
	s string
}

func (e *MachineNoIP) Error() string {
	return e.s
}

func toMStNetworks(ss ...string) (networks []metalgo.MachineAllocationNetwork) {
	for _, s := range ss {
		networks = append(networks, metalgo.MachineAllocationNetwork{
			NetworkID:   s,
			Autoacquire: true,
		})
	}
	return
}
