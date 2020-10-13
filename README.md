# Cluster API Provider MetalStack

```
cd /path/to/mini-lab
make
eval $(make dev-env)
cd /path/to/cluster-api-provider-metalstack
make crds
make managerless

sed -i "s/cluster-api-provider-metalstack-controller-manager-metrics-service/cap-metalstack-controller-manager-metrics-service/g" out/managerless/infrastructure-metalstack/v0.3.0/infrastructure-components.yaml
clusterctl init --config=out/managerless/infrastructure-metalstack/clusterctl-v0.3.0.yaml --infrastructure=metalstack -v3
make cluster

k apply -f ./out/cluster.yaml
make manager && ./bin/manager-linux-amd64

# in another terminal
watch metalctl machine ls 
# "Phoned Home" should be observed eventually

# in another terminal
watch kubectl get cluster 
# "Provisioned" should be observed eventually
```
