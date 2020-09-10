#!/bin/bash
#
# Copyright 2018-2020 Red Hat, Inc.
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

export KUBECTL=${KUBECTL:-'./cluster/kubectl.sh'}
KUBEVIRT_NAMESPACE=${KUBEVIRT_NAMESPACE:-'kubevirt'}
DEFAULT_SC=${DEFAULT_SC:-'local'}
NFS_SC=${NFS_SC:-'nfs'}

echo "Using: "
echo "  KUBECTL: $KUBECTL"
echo "  KUBECONFIG: $KUBECONFIG"
echo "  IMAGEIO_NAMESPACE: $IMAGEIO_NAMESPACE"
echo "  VCSIM_NAMESPACE: $VCSIM_NAMESPACE"
echo "  DEFAULT_SC: $DEFAULT_SC"
echo "  NFS_SC: $NFS_SC"

./cluster/imageio-install.sh "$IMAGEIO_NAMESPACE"
./cluster/vcsim-install.sh "$VCSIM_NAMESPACE"

FAKEOVIRT_CA_PATH=${FAKEOVIRT_CA_PATH:-$(pwd)/_out/fakeovirt-ca.pem}

$KUBECTL -n "$IMAGEIO_NAMESPACE" exec deploy/imageio-deployment -c imageiotest -- cat /tmp/certs/ca.pem > "$FAKEOVIRT_CA_PATH"

$KUBECTL apply -f tests/os-mapping/os-mapping.yaml

go test ./tests/ovirt ./tests/vmware --v -timeout 120m -kubeconfig "$KUBECONFIG" -ovirt-ca "$FAKEOVIRT_CA_PATH" -imageio-namespace "$IMAGEIO_NAMESPACE" -kubevirt-namespace "$KUBEVIRT_NAMESPACE" -default-sc "$DEFAULT_SC" -nfs-sc "$NFS_SC"
