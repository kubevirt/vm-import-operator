#!/bin/bash -e

export CDI_INSTALL_TIMEOUT=${CDI_INSTALL_TIMEOUT:-120}

source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh
source ./cluster-sync/${KUBEVIRT_PROVIDER}/provider.sh

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

# TODO: fetch it dynamically
export OPERATOR_VERSION=v0.0.1

_kubectl apply -f manifests/vm-import-operator/${OPERATOR_VERSION}/v2v_v1alpha1_resourcemapping_crd.yaml
_kubectl apply -f manifests/vm-import-operator/${OPERATOR_VERSION}/v2v_v1alpha1_virtualmachineimport_crd.yaml
_kubectl apply -f manifests/vm-import-operator/${OPERATOR_VERSION}/config_map.yaml
_kubectl apply -f manifests/vm-import-operator/${OPERATOR_VERSION}/operator.yaml
