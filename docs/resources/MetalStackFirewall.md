# MetalStackFirewall

Resource that provides Firewall configuration.

## Usage example
Firewall spec usually should be defined in `MetalStackCluster` resource `firewallSpec` field. Then `MetalStackCluster` controller will create `MetalStackFirewall` resource when all required data is ready(private network ID and kubeconfig for cluster):
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: MetalStackCluster
metadata:
  name: test1-v8vmn
  namespace: default
spec:
  controlPlaneEndpoint:
    host: 100.255.254.3
    port: 6443
  firewallSpec:
    image: firewall-ubuntu-2.0
    machineType: v1-small-x86
    providerID: metalstack://2294c949-88f6-5390-8154-fa53d93a3313
  partition: vagrant
  projectID: 00000000-0000-0000-0000-000000000000
  publicNetworkID: internet-vagrant-lab
```

After `MetalStackCluster` and `MetalStackFirewall` controllers reconcilation:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: MetalStackFirewall
metadata:
  name: test1-v8vmn
  namespace: default
  labels: 
      cluster.x-k8s.io/cluster-name: test1-v8vmn
spec:
  image: firewall-ubuntu-2.0
  machineType: v1-small-x86
  providerID: metalstack://2294c949-88f6-5390-8154-fa53d93a3313
status:
  ready: true
```

## Fields
Required fields:
- **image**: string -- OS image name.
- **machineType**: string -- machine type(currently specifies only size).

Optional fields:
- **providerID**: string -- ID of Metal Stack machine on which the firewall should be deployed.
- **sshKeys**: string -- public SSH keys for machine.