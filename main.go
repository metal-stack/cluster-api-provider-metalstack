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
package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	metalgo "github.com/metal-stack/metal-go"

	infra "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	"github.com/metal-stack/cluster-api-provider-metalstack/controllers"
	clusterapi "sigs.k8s.io/cluster-api/api/v1alpha3"
	// +kubebuilder:scaffold:imports
)

var setupLog = ctrl.Log.WithName("setup")

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8081", "The address the metric endpoint binds to.")
	flag.BoolVar(
		&enableLeaderElection,
		"enable-leader-election",
		false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// Create the `metal-API` client.
	metalClient, err := metalgo.NewDriver(os.Getenv("METALCTL_URL"), "", os.Getenv("METALCTL_HMAC"))
	if err != nil {
		setupLog.Error(err, "unable to get `metal-stack/metal-go`client")
		os.Exit(1)
	}
	setupLog.Info("metalstack client connected")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), *newManagerOptions(metricsAddr, enableLeaderElection))
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = controllers.NewMetalStackClusterReconciler(metalClient, mgr).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MetalStackCluster")
		os.Exit(1)
	}
	if err = controllers.NewMetalStackMachineReconciler(metalClient, mgr).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MetalStackMachine")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func newAndReadyScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterapi.AddToScheme(scheme)
	_ = infra.AddToScheme(scheme)
	return scheme
}

func newManagerOptions(metricsAddr string, enableLeaderElection bool) *ctrl.Options {
	// Machine and cluster operations can create enough events to trigger the event recorder spam filter
	// Setting the burst size higher ensures all events will be recorded and submitted to the API
	// broadcaster := record.NewBroadcasterWithCorrelatorOptions(record.CorrelatorOptions{
	// 	BurstSize: 100,
	// })
	return &ctrl.Options{
		Scheme:             newAndReadyScheme(),
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		// EventBroadcaster:   broadcaster,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "cad3ba79.cluster.x-k8s.io", // todo: What is this?
	}
}
