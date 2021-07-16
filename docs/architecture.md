# Architecture

![Architecture diagram](images/metal_stack_arch.drawio.svg)

## Components
The `cluster-api-provider-metalstack` controller main goal is to provision  K8s cluster on `metal-stack`. Main requirements:
- Provision ControlPlane node
- Provision Worker nodes
- Allow user to provision Firewall for Cluster
- Allow user to deploy nodes on specific `metal-stack` machines.

### MetalStackCluster Controller
Watches new/updated/deleted `MetalStackCluster` resources. Responsible for:
- network allocation
- MetalStackFirewall resource creation

#### Reconciliation example:
Initial state of new `MetalStackCluster` resource:
```
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalStackCluster
metadata:
  name: metal-stack-cluster
  namespace: metal-stack
spec:
  controlPlaneEndpoint:
    host: "100.255.254.1"
    port: 6443
  firewallSpec:
    image: firewall-ubuntu-2.0
    machineType: v1-small-x86
    providerID: metalstack://2294c949-88f6-5390-8154-fa53d93a3313
  partition: vagrant
  projectID: 00000000-0000-0000-0000-000000000000
  publicNetworkID: internet-vagrant-lab
```

State of resource after reconciliation:
```
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalStackCluster
metadata:
  name: metal-stack-cluster
  namespace: metal-stack
  |----------------------------------------------------------------------------|
  |# ownerReferences refers to the linked Cluster                              |
  |ownerReferences:                                                            |
  |- apiVersion: cluster.x-k8s.io/v1alpha3                                     |
  |  kind: Cluster                                                             |
  |  name: metal-stack-cluster                                                 |
  |  uid: 193ec580-89db-46cd-b6f7-ddc0cd79636d                                 |
  |----------------------------------------------------------------------------|
spec:
  controlPlaneEndpoint:
    host: "100.255.254.1"
    port: 6443
  firewallSpec:
    image: firewall-ubuntu-2.0
    machineType: v1-small-x86
    providerID: metalstack://2294c949-88f6-5390-8154-fa53d93a3313
  partition: vagrant
  projectID: 00000000-0000-0000-0000-000000000000
  publicNetworkID: internet-vagrant-lab
status:
  ready: true
```

### MetalStackMachine Controller
Watches new/updated/deleted `MetalStackMachine` resources. Responsible for:
- creates/updates raw machine instance

#### Reconciliation example
Initial state of new `MetalStackMachine` resource:
```
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalStackMachine
metadata:
  name: metal-stack-machine
  namespace: default
spec:
  image: ubuntu-cloud-init-20.04
  providerID: metalstack://e0ab02d2-27cd-5a5e-8efc-080ba80cf258
  machineType: v1-small-x86
  partition: vagrant
```

State of resource after reconciliation:
```
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalStackMachine
metadata:
  name: metal-stack-machine
  namespace: default
  |----------------------------------------------------------------------------|
  |# ownerReferences refers to the linked Machine                              |
  |ownerReferences:                                                            |
  |- apiVersion: cluster.x-k8s.io/v1alpha3                                     |
  |  kind: Machine                                                             |
  |  name: metal-stack-cluster                                                 |
  |  uid: 193ec580-89db-46cd-b6f7-ddc0cd796332                                 |
  |----------------------------------------------------------------------------|
spec:
  image: ubuntu-cloud-init-20.04
  providerID: metalstack://e0ab02d2-27cd-5a5e-8efc-080ba80cf258
  machineType: v1-small-x86
  partition: vagrant
  |----------------------------------------------------------------------------|
  |# userData comes from 'Machine'                                             |
  |userData:                                                                   |
  |  name: test1-controlplane-0-user-data                                      |
  | namespace: metal3                                                          |
  |----------------------------------------------------------------------------|
status:
  addresses:
  - address: 172.22.0.10
    type: InternalIP
  - address: node-1
    type: Hostname
  - address: node-1
    type: InternalDNS
  ready: true  
```

### MetalStackFirewall Controller
Watches new/updated/deleted `MetalStackFirewall` resources. Responsible for:
- creates/updates firewall instance.

#### Reconciliation example
Initial state of new `MetalStackFirewall` resource:
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
  ready: false
```

State of resource after reconciliation:
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