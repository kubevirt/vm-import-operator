#!/usr/bin/env bash

set -e

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
    ./cluster/kubectl.sh apply -f cluster/imageio.yaml
    ./cluster/kubectl.sh -n cdi wait deploy imageio-deployment --for condition=Available --timeout=1200s
}

# Install golang to run generate manifests
function ensure_golang {
    export PATH=$PATH:/usr/local/go/bin
    GOVERSION='go1.14.2.linux-amd64.tar.gz'
    if [[ "$(go version 2>&1)" =~ "not found" ]]; then
        wget https://dl.google.com/go/${GOVERSION}
        sudo tar -C /usr/local -xzf ${GOVERSION}
    fi
}
