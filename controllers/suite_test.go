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

	api "github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apimachineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
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
	namespaceName         = "test"
	clusterName           = "test-cluster-name"
	metalStackClusterName = "test-metal-stack-cluster"
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
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

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

func newOwnerRef() *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: capi.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       clusterName,
	}
}

func newCluster(paused bool) *capi.Cluster {
	spec := capi.ClusterSpec{
		Paused: paused,
	}
	status := capi.ClusterStatus{}
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

func newMetalStackCluster(ownerRef *metav1.OwnerReference, privateNetworkID *string, deletion bool) *api.MetalStackCluster {
	spec := api.MetalStackClusterSpec{
		PrivateNetworkID: privateNetworkID,
	}
	status := api.MetalStackClusterStatus{}
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
	if deletion {
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
