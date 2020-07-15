#!/usr/bin/env bash

set -e

function debug {
    ./cluster/kubectl.sh get events
    ./cluster/kubectl.sh get all -n $1
    ./cluster/kubectl.sh get pod -n $1 | awk 'NR>1 {print $1}' | xargs ./cluster/kubectl.sh logs -n $1 --tail=50
}

function install_cdi {
    export VERSION="v1.18.2"
    ./cluster/kubectl.sh apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/$VERSION/cdi-operator.yaml
    ./cluster/kubectl.sh apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/$VERSION/cdi-cr.yaml
    ./cluster/kubectl.sh wait cdis.cdi.kubevirt.io/cdi --for=condition=Available --timeout=1200s || debug cdi
}

function install_kubevirt {
    export KUBEVIRT_VER=$(curl -s https://github.com/kubevirt/kubevirt/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
    ./cluster/kubectl.sh apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VER}/kubevirt-operator.yaml
    ./cluster/kubectl.sh create configmap -n kubevirt kubevirt-config --from-literal=feature-gates=DataVolumes,ImportWithoutTemplate
    ./cluster/kubectl.sh apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VER}/kubevirt-cr.yaml
    ./cluster/kubectl.sh -n kubevirt wait kv kubevirt --for condition=Available --timeout=1200s || debug kubevirt
}

# Install golang to run generate manifests
function ensure_golang {
    GOVERSION='go1.14.2.linux-amd64.tar.gz'
    if [[ "$(go version 2>&1)" =~ "not found" ]]; then
        wget -q https://dl.google.com/go/${GOVERSION}
        tar -C /usr/local -xzf ${GOVERSION}
    fi
}

function install_templates {
    if [[ "$KUBEVIRT_PROVIDER" =~ (ocp|okd)- ]]; then
      export TEMPLATES_VER=$(curl -s https://github.com/kubevirt/common-templates/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
      ./cluster/kubectl.sh apply -f https://github.com/kubevirt/common-templates/releases/download/${TEMPLATES_VER}/common-templates-${TEMPLATES_VER}.yaml
    fi
}

function configure_nfs() {
  #Configure static nfs service and storage class, so we can create NFS PVs during test run.
  ./cluster/kubectl.sh apply -f ./cluster/manifests/nfs/nfs-sc.yaml
  ./cluster/kubectl.sh apply -f ./cluster/manifests/nfs/nfs-service.yaml
  ./cluster/kubectl.sh apply -f ./cluster/manifests/nfs/nfs-server.yaml

  # We don't provide any provisioner for sc, so creating PV manually:
  nfsIP=`./cluster/kubectl.sh get service nfs-service -o=jsonpath='{.spec.clusterIP}'`
  cat <<EOF | ./cluster/kubectl.sh apply -f -
apiVersion: v1
kind: PersistentVolume
metadata:
  name: nfs-pv-0
spec:
  accessModes:
  - ReadWriteMany
  capacity:
    storage: 34Gi
  persistentVolumeReclaimPolicy: Retain
  storageClassName: nfs
  volumeMode: Filesystem
  nfs:
    path: /
    server: $nfsIP
EOF

}
