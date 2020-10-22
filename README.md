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
make route
sudo ip r a 100.255.254.0/24 nexthop via 192.168.121.44 dev virbr1 nexthop via 192.168.121.12 dev virbr1\n
make fwrules
sudo iptables -I LIBVIRT_FWO -s 100.255.254.0/24 -i virbr1 -j ACCEPT;\nsudo iptables -I LIBVIRT_FWI -d 100.255.254.0/24 -o virbr1 -j ACCEPT;\nsudo iptables -t nat -A LIBVIRT_PRT -s 100.255.254.0/24 ! -d 100.255.254.0/24 -j MASQUERADE\n
ssh metal@100.255.254.1
