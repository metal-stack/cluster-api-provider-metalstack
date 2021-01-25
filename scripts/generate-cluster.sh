#!/bin/bash
# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

# might want to use a specific path to clusterctl
CLUSTERCTL=${CLUSTERCTL:-clusterctl}

# might want to use a specific config URL
CONFIG_URL=${CONFIG_URL:-""}
CONFIG_OPT=${CONFIG_OPT:-"--config=out/managerless/infrastructure-metalstack/clusterctl-${RELEASE_VERSION}.yaml"}
if [ -n "$CONFIG_URL" ]; then
	CONFIG_OPT="--config ${CONFIG_URL}"
fi

TEMPLATE_OUT=./out/cluster.yaml

DEFAULT_KUBERNETES_VERSION=1.20.2
DEFAULT_POD_CIDR="172.25.0.0/16"
DEFAULT_SERVICE_CIDR="172.26.0.0/16"
DEFAULT_MASTER_NODE_TYPE="v1-small-x86"
DEFAULT_WORKER_NODE_TYPE="v1-small-x86"
DEFAULT_NODE_IMAGE="ubuntu-cloud-init-20.04"
DEFAULT_CONTROL_PLANE_IP="100.255.254.3"

# check required environment variables
errstring=""

# if [ -z "$METAL_PROJECT_ID" ]; then
# 	errstring="${errstring} METAL_PROJECT_ID"
# fi
# if [ -z "$METAL_PARTITION" ]; then
# 	errstring="${errstring} METAL_PARTITION"
# fi

if [ -n "$errstring" ]; then
	echo "must set environment variables: ${errstring}" >&2
	exit 1
fi


# Generate a somewhat unique cluster name. This only needs to be unique per project.
RANDOM_STRING=$(LC_ALL=C tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c5 | tr '[:upper:]' '[:lower:]')
# Human friendly cluster name, limited to 6 characters
HUMAN_FRIENDLY_CLUSTER_NAME=test1
DEFAULT_CLUSTER_NAME=${HUMAN_FRIENDLY_CLUSTER_NAME}-${RANDOM_STRING}

CLUSTER_NAME=${CLUSTER_NAME:-${DEFAULT_CLUSTER_NAME}}
POD_CIDR=${POD_CIDR:-${DEFAULT_POD_CIDR}}
SERVICE_CIDR=${SERVICE_CIDR:-${DEFAULT_SERVICE_CIDR}}
WORKER_NODE_TYPE=${WORKER_NODE_TYPE:-${DEFAULT_WORKER_NODE_TYPE}}
MASTER_NODE_TYPE=${MASTER_NODE_TYPE:-${DEFAULT_MASTER_NODE_TYPE}}
NODE_IMAGE=${DEFAULT_NODE_IMAGE:-${DEFAULT_NODE_IMAGE}}
CONTROL_PLANE_IP=${DEFAULT_CONTROL_PLANE_IP:-${DEFAULT_CONTROL_PLANE_IP}}
KUBERNETES_VERSION=${KUBERNETES_VERSION:-${DEFAULT_KUBERNETES_VERSION}}
SSH_KEY=${SSH_KEY:-""}

NETWORK_ID=${METAL_NETWORK_ID:-"00000000-0000-0000-0000-000000000000"}
PROJECT_ID=${METAL_PROJECT_ID:-"00000000-0000-0000-0000-000000000000"}
PARTITION=${METAL_PARTITION:-"vagrant"}

# FIXME unused
NODE_OS=NODE_IMAGE
FACILITY=PARTITION

# and now export them all so envsubst can use them
export PROJECT_ID PARTITION NETWORK_ID NODE_IMAGE CONTROL_PLANE_IP WORKER_NODE_TYPE MASTER_NODE_TYPE POD_CIDR SERVICE_CIDR SSH_KEY KUBERNETES_VERSION NODE_OS FACILITY
echo "${CLUSTERCTL} -v3 ${CONFIG_OPT} --infrastructure=metalstack config cluster ${CLUSTER_NAME} > $TEMPLATE_OUT"
${CLUSTERCTL} -v3 ${CONFIG_OPT} --infrastructure=metalstack config cluster ${CLUSTER_NAME} > $TEMPLATE_OUT

echo "Done! See output file at ${TEMPLATE_OUT}. Run:"
echo "   kubectl apply -f ${TEMPLATE_OUT}"
exit 0
