#!/usr/bin/env bash

set -e

function install_cnao {
  CNAO_VERSION=${CNAO_VERSION:-'0.35.1'}
  ./cluster/kubectl.sh apply -f https://raw.githubusercontent.com/kubevirt/cluster-network-addons-operator/master/manifests/cluster-network-addons/${CNAO_VERSION}/namespace.yaml
  ./cluster/kubectl.sh apply -f https://raw.githubusercontent.com/kubevirt/cluster-network-addons-operator/master/manifests/cluster-network-addons/${CNAO_VERSION}/network-addons-config.crd.yaml
  ./cluster/kubectl.sh apply -f https://raw.githubusercontent.com/kubevirt/cluster-network-addons-operator/master/manifests/cluster-network-addons/${CNAO_VERSION}/operator.yaml
  ./cluster/kubectl.sh apply -f ./cluster/manifests/cnao.yaml
  ./cluster/kubectl.sh wait networkaddonsconfig cluster --for condition=Available --timeout=600s
}

function install_cdi {
    export VERSION=$(curl -s https://github.com/kubevirt/containerized-data-importer/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
    ./cluster/kubectl.sh apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/$VERSION/cdi-operator.yaml
    ./cluster/kubectl.sh apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/$VERSION/cdi-cr.yaml
    ./cluster/kubectl.sh wait cdis.cdi.kubevirt.io/cdi --for=condition=Available --timeout=600s
}

function install_kubevirt {
    export KUBEVIRT_VER=$(curl -s https://github.com/kubevirt/kubevirt/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
    ./cluster/kubectl.sh apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VER}/kubevirt-operator.yaml
    ./cluster/kubectl.sh create configmap -n kubevirt kubevirt-config --from-literal=feature-gates=DataVolumes
    ./cluster/kubectl.sh apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VER}/kubevirt-cr.yaml
    ./cluster/kubectl.sh -n kubevirt wait kv kubevirt --for condition=Available --timeout=600s
}

function install_imageio {
    ./cluster/kubectl.sh apply -f cluster/manifests/imageio.yaml
    ./cluster/kubectl.sh -n cdi wait deploy imageio-deployment --for condition=Available --timeout=1200s
    ./cluster/kubectl.sh -n cdi port-forward -n cdi service/imageio 12346:12346 &
}

# Install golang to run generate manifests
function ensure_golang {
    GOVERSION='go1.14.2.linux-amd64.tar.gz'
    if [[ "$(go version 2>&1)" =~ "not found" ]]; then
        wget -q https://dl.google.com/go/${GOVERSION}
        tar -C /usr/local -xzf ${GOVERSION}
    fi
}
