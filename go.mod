module github.com/metal-stack/cluster-api-provider-metalstack

go 1.14

require (
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.4.4
	github.com/google/uuid v1.1.2
	github.com/metal-stack/metal-go v0.7.7
	github.com/metal-stack/metal-lib v0.6.2
	github.com/onsi/ginkgo v1.13.0
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.18.5
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v0.18.5
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19
	sigs.k8s.io/cluster-api v0.3.6
	sigs.k8s.io/controller-runtime v0.6.0
)
