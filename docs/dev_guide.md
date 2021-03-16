# Development guide

## Prerequisites
First of all, follow steps in [dev setup document](./dev_guide.md) prerequisites. 

## Setup
For basic setup with Control Plane node only, use `master` branch in `mini-lab`. For setup with worker node, swtich to `3-machines` branch. Then start `mini-lab` by executing `make`. After setup is complete, wait until all machines are in `Waiting` status(you can check that by running `metalctl machine ls` command).

After mini-lab is setup you need to set kubeconfig and apply routes and firewall rules so you can access firewall from the host:
```
eval $(make dev-env)
make route
# Execute the output of the previous command.
make fwrules
# Execute the output of the previous command.
```

## Full setup guide
First, you need to generate manifests with Cluster and Machine resources. In this project root directory run:
```
make crds
make managerless
```

After manifests are generated you need to setup Cluster API manager and apply CRD that provider requires. You will need to substitute current version in command bellow. Usually it's `{latest_tag}-dirty`.
```
clusterctl init --config=out/managerless/infrastructure-metalstack/clusterctl-{version}.yaml --infrastructure=metalstack -v3
```

After Cluster API manager is setup, you can run provider:
```
make manager && ./bin/manager-linux-amd64
```

To create Kubernetes Control Plane, run:
```
make cluster
kubectl apply -f ./out/cluster.yaml
```

First it will create Control Plane node, then Firewall(after kubeconfig for created cluster is available as a `Secret` in Cluster API management node).

If you are on `3-machines` branch of `mini-lab`, you can also start worker node. First, you will need to install CNI on you cluster. First you need to get kubeconfig and save it to some file, like `clusterctl get kubeconfig {cluster-name} > k.conf`. You can check `cluster-name` in `out/cluster.yaml` file. Then install some CNI, for example `Flannel` manifests:
```
kubectl apply --kubeconfig=k.conf -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
```

You will need to wait until Control Plane `Node` is ready. You can check that by running:
```
kubectl get nodes --kubeconfig=k.conf
```

After network is setup you can start worker node. For that, go to `out/cluster.yaml` file, find `MachineDeployment` manifest and set `replicas` to 1. After that apply updated manifests:
```
kubectl apply -f ./out/cluster.yaml
```

After some time you should see that worker node is started.

## Testing
To run controller test, execute `make test`. To run E2E test, `3-machines` branch of `mini-lab` need to be started, after it's ready run `make e2e` command.