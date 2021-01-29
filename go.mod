module github.com/metal-stack/cluster-api-provider-metalstack

go 1.15

require (
	github.com/containerd/fifo v0.0.0-20201026212402-0724c46b320c // indirect
	github.com/docker/docker v0.7.3-0.20190506211059-b20a14b54661
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.4.4
	github.com/google/uuid v1.1.5 // indirect
	github.com/metal-stack/metal-go v0.11.5
	github.com/metal-stack/metal-lib v0.6.8
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.17.9
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v0.17.9
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19
	sigs.k8s.io/cluster-api v0.3.12
	sigs.k8s.io/controller-runtime v0.5.14
)
