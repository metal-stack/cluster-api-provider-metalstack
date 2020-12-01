module github.com/metal-stack/cluster-api-provider-metalstack

go 1.14

require (
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.4.4
	github.com/google/uuid v1.1.2
	github.com/metal-stack/metal-go v0.10.3
	github.com/metal-stack/metal-lib v0.6.3
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.10.0
	k8s.io/api v0.17.12
	k8s.io/apimachinery v0.17.12
	k8s.io/client-go v0.17.12
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20201015054608-420da100c033
	sigs.k8s.io/cluster-api v0.3.10
	sigs.k8s.io/controller-runtime v0.5.11
)
