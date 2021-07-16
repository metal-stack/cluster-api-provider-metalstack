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
	"path/filepath"
	"testing"

	api "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha4"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apimachineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	capiremote "sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	nodeID         = "metalstack://test"
	clusterName    = "test-cluster-name"
	machineName    = "test-machine-name"
	namespaceName  = "test"
	dataSecretName = "test-data-secret"

	metalStackClusterName  = "test-metal-stack-cluster"
	metalStackMachineName  = "test-metal-stack-machine"
	metalStackFirewallName = " test-metal-stack-firewall"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func init() {
	klog.InitFlags(nil)
	klog.SetOutput(GinkgoWriter)
	logf.SetLogger(klogr.New())
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	Expect(api.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(capi.AddToScheme(scheme.Scheme)).To(Succeed())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func setupScheme() *apimachineryruntime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = capi.AddToScheme(scheme)
	_ = api.AddToScheme(scheme)

	return scheme
}

func newTestMetalClusterReconciler(metalClient MetalStackClient, objects []runtime.Object) *MetalStackClusterReconciler {
	return &MetalStackClusterReconciler{
		Client:           fake.NewFakeClientWithScheme(setupScheme(), objects...),
		Log:              zap.New(zap.UseDevMode(true)),
		MetalStackClient: metalClient,
	}
}

func newTestMetalMachineReconciler(metalClient MetalStackClient, objects []runtime.Object) *MetalStackMachineReconciler {
	log := zap.New(zap.UseDevMode(true))
	scheme := setupScheme()
	client := fake.NewFakeClientWithScheme(scheme, objects...)

	return &MetalStackMachineReconciler{
		Client: client,
		Log:    log,
		ClusterTracker: capiremote.NewTestClusterCacheTracker(
			zap.New(zap.UseDevMode(true)),
			client,
			scheme,
			types.NamespacedName{
				Namespace: namespaceName,
				Name:      clusterName,
			},
		),
		MetalStackClient: metalClient,
	}
}

func newTestMetalFirewallReconciler(metalClient MetalStackClient, objects []runtime.Object) *MetalStackFirewallReconciler {
	return &MetalStackFirewallReconciler{
		Client:           fake.NewFakeClientWithScheme(setupScheme(), objects...),
		Log:              zap.New(zap.UseDevMode(true)),
		MetalStackClient: metalClient,
	}
}

func newClusterOwnerRef() *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: capi.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       clusterName,
	}
}

func newMachineOwnerRef() *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: capi.GroupVersion.String(),
		Kind:       "Machine",
		Name:       machineName,
	}
}

func newCluster(paused, infraReady bool) *capi.Cluster {
	spec := capi.ClusterSpec{
		Paused: paused,
		InfrastructureRef: &corev1.ObjectReference{
			Name: metalStackClusterName,
		},
	}
	status := capi.ClusterStatus{
		InfrastructureReady: infraReady,
	}
	typeMeta := metav1.TypeMeta{
		Kind:       "Cluster",
		APIVersion: capi.GroupVersion.String(),
	}
	objMeta := metav1.ObjectMeta{
		Name:      clusterName,
		Namespace: namespaceName,
	}

	return &capi.Cluster{
		TypeMeta:   typeMeta,
		ObjectMeta: objMeta,
		Spec:       spec,
		Status:     status,
	}
}

func newMetalStackCluster(
	ownerRef *metav1.OwnerReference,
	privateNetworkID *string,
	controlPlaneIPAllocated bool,
	deleted bool,
) *api.MetalStackCluster {
	spec := api.MetalStackClusterSpec{
		PrivateNetworkID: privateNetworkID,
	}
	status := api.MetalStackClusterStatus{
		ControlPlaneIPAllocated: controlPlaneIPAllocated,
	}
	typeMeta := metav1.TypeMeta{
		Kind:       "MetalStackCluster",
		APIVersion: api.GroupVersion.String(),
	}

	ownerRefs := []metav1.OwnerReference{}
	if ownerRef != nil {
		ownerRefs = append(ownerRefs, *ownerRef)
	}

	objMeta := metav1.ObjectMeta{
		Name:            metalStackClusterName,
		Namespace:       namespaceName,
		OwnerReferences: ownerRefs,
	}
	if deleted {
		now := metav1.Now()
		objMeta.DeletionTimestamp = &now
	}

	return &api.MetalStackCluster{
		TypeMeta:   typeMeta,
		ObjectMeta: objMeta,
		Spec:       spec,
		Status:     status,
	}
}

func newMachine() *capi.Machine {
	spec := capi.MachineSpec{
		Bootstrap: capi.Bootstrap{
			DataSecretName: pointer.StringPtr(dataSecretName),
		},
	}
	status := capi.MachineStatus{}
	typeMeta := metav1.TypeMeta{
		Kind:       "Machine",
		APIVersion: capi.GroupVersion.String(),
	}
	objMeta := metav1.ObjectMeta{
		Name:      machineName,
		Namespace: namespaceName,
		Labels:    map[string]string{capi.ClusterLabelName: clusterName},
	}

	return &capi.Machine{
		TypeMeta:   typeMeta,
		ObjectMeta: objMeta,
		Spec:       spec,
		Status:     status,
	}
}

func newMetalStackMachine(ownerRef *metav1.OwnerReference, providerID *string, deleted bool) *api.MetalStackMachine {
	spec := api.MetalStackMachineSpec{
		ProviderID: providerID,
	}
	status := api.MetalStackMachineStatus{}
	typeMeta := metav1.TypeMeta{
		Kind:       "MetalStackMachine",
		APIVersion: api.GroupVersion.String(),
	}

	ownerRefs := []metav1.OwnerReference{}
	if ownerRef != nil {
		ownerRefs = append(ownerRefs, *ownerRef)
	}

	objMeta := metav1.ObjectMeta{
		Name:            metalStackMachineName,
		Namespace:       namespaceName,
		OwnerReferences: ownerRefs,
	}
	if deleted {
		now := metav1.Now()
		objMeta.DeletionTimestamp = &now
	}

	return &api.MetalStackMachine{
		TypeMeta:   typeMeta,
		ObjectMeta: objMeta,
		Spec:       spec,
		Status:     status,
	}
}

func newMetalStackFirewall(providerID *string, deleted bool) *api.MetalStackFirewall {
	spec := api.MetalStackFirewallSpec{
		ProviderID: providerID,
	}
	status := api.MetalStackFirewallStatus{}
	typeMeta := metav1.TypeMeta{
		Kind:       "MetalStackFirewall",
		APIVersion: api.GroupVersion.String(),
	}
	objMeta := metav1.ObjectMeta{
		Name:      metalStackFirewallName,
		Namespace: namespaceName,
		Labels:    map[string]string{capi.ClusterLabelName: metalStackClusterName},
	}
	if deleted {
		now := metav1.Now()
		objMeta.DeletionTimestamp = &now
	}

	return &api.MetalStackFirewall{
		TypeMeta:   typeMeta,
		ObjectMeta: objMeta,
		Spec:       spec,
		Status:     status,
	}
}

func newSecret(name string) *corev1.Secret {
	typeMeta := metav1.TypeMeta{
		Kind:       "Secret",
		APIVersion: corev1.SchemeGroupVersion.Version,
	}
	objMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespaceName,
	}

	return &corev1.Secret{
		TypeMeta:   typeMeta,
		ObjectMeta: objMeta,
		Data:       map[string][]byte{"value": []byte("")},
	}
}

func newNode() *corev1.Node {
	status := corev1.NodeStatus{
		NodeInfo: corev1.NodeSystemInfo{
			SystemUUID: nodeID,
		},
	}
	typeMeta := metav1.TypeMeta{
		Kind:       "Node",
		APIVersion: corev1.SchemeGroupVersion.Version,
	}
	objMeta := metav1.ObjectMeta{
		Name:      nodeID,
		Namespace: namespaceName,
	}

	return &corev1.Node{
		Status:     status,
		TypeMeta:   typeMeta,
		ObjectMeta: objMeta,
	}
}
