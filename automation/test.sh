#!/bin/bash
#
# This file is part of the KubeVirt project
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
#
# Copyright 2017 Red Hat, Inc.
#

# CI considerations: $TARGET is used by the jenkins build, to distinguish what to test
# Currently considered $TARGET values:
#     kubernetes-release: Runs all functional tests on a release kubernetes setup
#     openshift-release: Runs all functional tests on a release openshift setup

set -ex

export KUBEVIRT_PROVIDER=$TARGET

KUBEVIRT_NUM_NODES=1
KUBEVIRT_MEMORY_SIZE=8192

if [ ! -d "cluster-up/cluster/$KUBEVIRT_PROVIDER" ]; then
  echo "The cluster provider $KUBEVIRT_PROVIDER does not exist"
  exit 1
fi

kubectl() { cluster-up/kubectl.sh "$@"; }

./cluster-up/down.sh

./cluster-up/up.sh

# Wait for nodes to become ready
set +e
kubectl_rc=0
retry_counter=0
while [[ $retry_counter -lt 30 ]] && [[ $kubectl_rc -ne 0 || -n "$(kubectl get nodes --no-headers | grep NotReady)" ]]; do
    echo "Waiting for all nodes to become ready ..."
    kubectl get nodes --no-headers
    kubectl_rc=$?
    retry_counter=$((retry_counter + 1))
    sleep 10
done
set -e

if [ $retry_counter -eq 30 ]; then
	echo "Not all nodes are up"
	exit 1
fi

echo "Nodes are ready:"
kubectl get nodes

./cluster-sync/sync.sh

kubectl version

# Run functional tests
kubectl create -f tests/cirros/secret.yml
kubectl create -f tests/cirros/mapping.yml
kubectl create -f tests/cirros/cirros.yml
