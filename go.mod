module github.com/metal-stack/cluster-api-provider-metalstack

go 1.14

require (
	github.com/containerd/fifo v0.0.0-20200410184934-f15a3290365b // indirect
	github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/google/uuid v1.1.2
	github.com/metal-stack/metal-go v0.7.7
	github.com/metal-stack/metal-lib v0.6.2
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/onsi/ginkgo v1.13.0
	github.com/onsi/gomega v1.10.1
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.18.5
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v0.18.5
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19
	sigs.k8s.io/cluster-api v0.3.6
	sigs.k8s.io/controller-runtime v0.6.0
)
