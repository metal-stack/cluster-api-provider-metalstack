# Development setup

## Prerequisites
First of all, follow steps in [dev guide document](./dev_guide.md) prerequisites. Then clone [mini-lab]() project to your local directory, it will provide infrastructure for testing this provider.

## Setup
For basic setup with Control Plane node only, use `master` branch in `mini-lab`. For setup with worker node, swtich to `3-machines` branch. 

In `mini-lab` project directory run:
```
make
eval $(make dev-env)

make route
# Execute the output of the previous command.
make fwrules
# Execute the output of the previous command.
```

In this project root directory run:
```
make crds
make managerless
```

Then run this commands, instead of `{version}` placeholder insert appropriate version(just check generated directories, usually it should be `{latest_tag}-dirty`):
```
sed -i "s/cluster-api-provider-metalstack-controller-manager-metrics-service/cap-metalstack-controller-manager-metrics-service/g" out/managerless/infrastructure-metalstack/{version}/infrastructure-components.yaml
clusterctl init --config=out/managerless/infrastructure-metalstack/clusterctl-{version}.yaml --infrastructure=metalstack -v3
```

Create resources for running cluster:
```
make cluster
kubectl apply -f ./out/cluster.yaml
```

Run this provider:
```
make manager && ./bin/manager-linux-amd64
```