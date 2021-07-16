# MetalStackCluster

Resource that provides Metal Stack specific Cluster configuration.

## Usage example

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: MetalStackCluster
metadata:
  name: metal-stack-cluster
  namespace: metal-stack
spec:
  controlPlaneEndpoint:
    host: "100.255.254.1"
    port: 6443
  firewall:
    defaultNetworkID: metal-stack-network
    image: firewall-ubuntu-2.0
    size: v1-small-x86
  partition: vagrant
  projectID: 00000000-0000-0000-0000-000000000000
```

After reconcilation:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: MetalStackCluster
metadata:
  name: metal-stack-cluster
  namespace: metal-stack
spec:
  controlPlaneEndpoint:
    host: "100.255.254.1"
    port: 6443
  firewall:
    defaultNetworkID: metal-stack-network
    image: firewall-ubuntu-2.0
    size: v1-small-x86
  partition: vagrant
  projectID: 00000000-0000-0000-0000-000000000000
  privateNetworkID: "vagrant-virtual-network"
status:
  ready: true
  firewallReady: true
```

## Fields
Required fields:
- **ControlPlaneEndpoint**: [v1alpha3.APIEndpoint]() - represents the endpoint used to communicate with the control plane.
- **ProjectID**: string - ID of project in which K8s cluster should be deployed. Project can span multiple partitions, but K8s cluster can be deployed only on single partition.
- **Partition**: string - ID of [partition](https://docs.metal-stack.io/stable/overview/architecture/#Partitions) in which K8s should be deployed.
- **Firewall**: [Firewall]() - each K8s cluster in Metal Stack should have dedicated firewall, so it's required that user provides firewall config. 

Optional fields:
- **PrivateNetworkID**: *string - ID of the network which connects nodes. If not specifyed, it's allocated by MetalStackCluster controller.