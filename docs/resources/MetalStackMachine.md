# MetalStackMachine

Resource that provides configuration for running machine on Metal Stack.

## Usage example

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalStackMachine
metadata:
  name: test1-hglxe-master-0
  namespace: default
spec:
  image: ubuntu-cloud-init-20.04
  machineType: v1-small-x86
```

After reconcilation:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalStackMachine
metadata:
  name: test1-hglxe-master-0
  namespace: default
spec:
  image: ubuntu-cloud-init-20.04
  machineType: v1-small-x86
  providerID: metalstack://e0ab02d2-27cd-5a5e-8efc-080ba80cf258
status:
  addresses:
    - address: 172.22.0.10
      type: InternalIP
  allocated: true
  ready: true
```

## Fields
Required fields:
- **Image**: string - OS image.
-	**MachineType**: string - machine type(currently specifies only size).

Optional fields:
- **ProviderID**: *string - ID of Metal Stack machine.
- **SSHKeys**: []string - public SSH keys for machine.
-	**Tags**: []string - set of tags to add to Metal Stack machine.