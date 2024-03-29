# Cluster API Provider for metal-stack

Infrastructure provider for Kubernetes [Cluster API](https://cluster-api.sigs.k8s.io/) project, that allows you to deploy and manage Kubernetes cluster on [metal-stack](https://metal-stack.io/).

## Compatibility with Cluster API versions
|                                        | Cluster API v1alpha3 (v0.3.x) | Cluster API v1alpha4 (v0.4.x) |
| -------------------------------------- | ----------------------------- | ----------------------------- |
| metal-stack Provider v1alpha3 (v0.3.x) | ✓                             |
| metal-stack Provider v1alpha4 (v0.4.x) |                               | ✓

## Compatibility with Kubernetes versions
|                                        | Kubernetes 1.21             |
| -------------------------------------- | --------------------------- |
| metal-stack Provider v1alpha3 (v0.3.x) | ✓                           |
| metal-stack Provider v1alpha4 (v0.4.x) | ✓                           |

## Docs
You can check docs [here](docs/contents.md) -- there we have high-level description of architecture, resources and controllers. And also guide for testing and developing this provider locally.
