#!/bin/bash -e

export CDI_INSTALL_TIMEOUT=${CDI_INSTALL_TIMEOUT:-120}

source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh
source ./cluster-sync/${KUBEVIRT_PROVIDER}/provider.sh

# Set controller verbosity to 3 for functional tests.
export VERBOSITY=3

# Install CDI
install_cdi

wait_cdi_crd_installed $CDI_INSTALL_TIMEOUT

# Install CDI CR
echo "Waiting 480 seconds for CDI to become available"
_kubectl wait cdis.cdi.kubevirt.io/cdi --for=condition=Available --timeout=480s

install_kubevirt

configure_storage

# We skip the functional test additions for external provider for now, as they're specific
if [ "${KUBEVIRT_PROVIDER}" != "external" ]; then
  _kubectl apply -f "./cluster-sync/imageio/imageio.yaml"
  _kubectl wait deploy/imageio-deployment -n cdi --for=condition=Available --timeout=480s
fi

# Setup vm-import-opreator
export RELEASE_VERSION=v0.15.2
export GO111MODULE=on

set -x
curl -LO https://github.com/operator-framework/operator-sdk/releases/download/${RELEASE_VERSION}/operator-sdk-${RELEASE_VERSION}-x86_64-linux-gnu
chmod +x operator-sdk-${RELEASE_VERSION}-x86_64-linux-gnu && sudo mkdir -p /usr/local/bin/ && sudo cp operator-sdk-${RELEASE_VERSION}-x86_64-linux-gnu /usr/local/bin/operator-sdk && rm operator-sdk-${RELEASE_VERSION}-x86_64-linux-gnu
operator-sdk --verbose build quay.io/kubevirt/vm-import-opreator

_kubectl apply -f ./deploy/crds/v2v_v1alpha1_resourcemapping_crd.yaml
_kubectl apply -f ./deploy/crds/v2v_v1alpha1_virtualmachineimport_crd.yaml
_kubectl apply -f ./deploy/service_account.yaml
_kubectl apply -f ./deploy/role.yaml
_kubectl apply -f ./deploy/role_binding.yaml
_kubectl apply -f ./deploy/operator.yaml
