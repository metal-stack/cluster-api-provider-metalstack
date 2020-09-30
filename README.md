# Cluster API Provider Metal-Stack

```
cd /path/to/mini-lab
make
eval `make dev-env`
cd /path/to/cluster-api-provider-metalstack
make managerless
sed -i "s/cluster-api-provider-metalstack-controller-manager-metrics-service/capmst-controller-manager-metrics-service/g" out/managerless/infrastructure-metalstack/v0.3.0/infrastructure-components.yaml
clusterctl init
clusterctl init --config=out/managerless/infrastructure-metalstack/clusterctl-v0.3.0.yaml --infrastructure=metalstack
make cluster
metalctl network allocate --partition vagrant --project 00000000-0000-0000-0000-000000000000 --name vagrant

# Copy the id (network ID).
# Replace the .spec.privateNetworkID of the MetalStackCluster in ./out/cluster.yaml with the freshly got network ID.

# Add the tag "kubernetes.io/role:master" to .spec.tags of the MetalStackMachine. The result reads: 
# tags: ["kubernetes.io/role:node"]

k apply -f ./out/cluster.yaml
make manager
./bin/manager-linux-amd64

# in another terminal
watch metalctl machine ls 
# "Phoned Home" should be observed eventually

# in another terminal
watch kubectl get cluster 
# "Provisioned" should be observed
```
