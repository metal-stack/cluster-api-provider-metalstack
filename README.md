# Cluster API Provider MetalStack

```
cd /path/to/mini-lab
make
eval $(make dev-env)
make route
# Execute the output of the previous command.
make fwrules
# Execute the output of the previous command.
cd /path/to/cluster-api-provider-metalstack
make crds
make kustomize
make managerless

sed -i "s/cluster-api-provider-metalstack-controller-manager-metrics-service/cap-metalstack-controller-manager-metrics-service/g" out/managerless/infrastructure-metalstack/v0.3.0/infrastructure-components.yaml
clusterctl init --config=out/managerless/infrastructure-metalstack/clusterctl-v0.3.0.yaml --infrastructure=metalstack -v3
make cluster

kubectl apply -f ./out/cluster.yaml
make manager && ./bin/manager-linux-amd64

# in another terminal
watch metalctl machine ls
# "Phoned Home" should be observed eventually

# in another terminal
watch kubectl get cluster
# "Provisioned" should be observed eventually
```

You can now join any number of control-plane nodes by copying certificate authorities and 
service account keys on each node and then running the following as root:
   kubeadm join 100.255.254.3:6443 \
        --token w3w158.716hh9d4tgtxoqd4 \
        --discovery-token-ca-cert-hash sha256:6813135efc2524d4f609e60c7d33feab8f561044eb226c053ca2b3f60c6432b3 \
        --control-plane

Then you can join any number of worker nodes by running the following on each as root:
    kubeadm join 100.255.254.3:6443 \
        --token w3w158.716hh9d4tgtxoqd4 \
        --discovery-token-ca-cert-hash sha256:6813135efc2524d4f609e60c7d33feab8f561044eb226c053ca2b3f60c6432b3