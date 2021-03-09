# Development setup

## Prerequisites
For development you will need to install 2 tools: [kubebuilder](https://book.kubebuilder.io/quick-start.html)(run `make kubebuilder`) and [kustomize](https://kubernetes-sigs.github.io/kustomize/)(run `make kustomize`).

For running and testing locally this provider you will need to install and run [mini-lab](https://github.com/metal-stack/mini-lab). Follow instructions in README to run it. It will provide Kubernetes cluster where Cluster API components will be installed and virtual environment.

## Updating API
Our API conforms to rules described in [kubebuilder book](https://book.kubebuilder.io/cronjob-tutorial/api-design.html), please read it first before making any changes to that.

After making changes to API resources, run `make generate` to update generated code in `/api` directory. Then run `make crds` to update CRD manifests stored in `/config/resources/crd/bases/`.

## Testing
Run `make test`.