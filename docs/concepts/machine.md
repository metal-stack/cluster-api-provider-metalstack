# MetalMachine

is the name of the resource that identifies a [Machine](metal-go) on Metal.

This is an example of it:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalMachine
metadata:
  name: "qa-master-0"
spec:
  OS: "ubuntu_18_04"
  facility:
  - "dfw2"
  billingCycle: hourly
  machineType: "t2.small"
  sshKeys:
  - "your-sshkey-name"
  tags: []
```

It is a [Kubernetes Custom Resource Definition (CRD)](crd-docs) as everything
else in the cluster-api land.

The reported fields in the example are the most common one but you can see the
full list of supported parameters as part of the OpenAPI definition available
[here](config/resources/crd/bases/infrastructure.cluster.x-k8s.io_metalmachines.yaml)
searching for `kind: MetalMachine`.

## Reserved instances

Metal provides the possibility to [reserve
hardware](metal-docs-reserved-hardware) in order to have to power you need
always available.

> Reserved hardware gives you the ability to reserve specific servers for a
> committed period of time. Unlike hourly on-demand, once you reserve hardware,
> you will have access to that specific hardware for the duration of the
> reservation.

You can specify the reservation ID using the field `hardwareReservationID`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: MetalMachine
metadata:
  name: "qa-master-0"
spec:
  image: "ubuntu-19.10"
  partition:
  - "dfw2"
  machineType: "c1-xlarge-x86"
  sshKeys:
  - "your-sshkey-name"
  tags: []
```

### pros and cons

Hardware reservation is a great feature, this chapter is about the feature
described above and nothing more.
It covers a very simple use case, you have a set of machines that you created
statically in the YAML and you like to have them using a reservation ID.

It does not work in combination of MetalMachineTemplate and MachineDeployment
where the pool of MetalMachine is dynamically managed by the cluster-api
controllers. You can track progress on this scenario subscribing to the issue
["Add support for reservation IDs with MachineDeployment #136"](github-issue-resid-dynamic) on GitHub.

[metal-go]: https://github.com/metal-stack/metal-go
[crd-docs]: https://github.com/metal-stack/cluster-api-provider-metal/blob/master/config/resources/crd/bases/infrastructure.cluster.x-k8s.io_metalmachines.yaml
[openapi-types]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[metal-docs-reserved-hardware]: https://www.metal.com/developers/docs/getting-started/deployment-options/reserved-hardware/
[github-issue-resid-dynamic]: https://github.com/metal-stack/cluster-api-provider-metal/issues/136
