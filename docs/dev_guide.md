# Development guide

## Prerequisites
For development you will need to install 2 tools: [kubebuilder](https://book.kubebuilder.io/quick-start.html)(run `make kubebuilder`) and [kustomize](https://kubernetes-sigs.github.io/kustomize/)(run `make kustomize`).

## Updating API
Our API conforms to rules described in [kubebuilder book](https://book.kubebuilder.io/cronjob-tutorial/api-design.html), please read it first before making any changes to that.

After making changes to API resources, run `make crds` to generate correct CRD manifests(stored in `/config/resources/crd/bases/`).

## Testing
Run `make test`.