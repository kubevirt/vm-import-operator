#!/usr/bin/env bash

set -e

function debug {
    ./cluster/kubectl.sh get events
    ./cluster/kubectl.sh get all -n $1
    ./cluster/kubectl.sh get pod -n $1 | awk 'NR>1 {print $1}' | xargs ./cluster/kubectl.sh logs -n $1 --tail=50
}

function install_cdi {
    export VERSION=$(curl -s https://api.github.com/repos/kubevirt/containerized-data-importer/releases | jq 'sort_by(.tag_name) | .[-1].tag_name' -r)
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

function check_structural_schema {
  for crd in "$@"; do
    status=$(./cluster/kubectl.sh get crd $crd -o jsonpath={.status.conditions[?\(@.type==\"NonStructuralSchema\"\)].status})
    if [ "$status" == "True" ]; then
      echo "ERROR CRD $crd is not a structural schema!, please fix"
      ./cluster/kubectl.sh get crd $crd -o yaml
      exit 1
    fi
    echo "CRD $crd is a StructuralSchema"
  done
}

function configure_block_pv() {
  ./cluster/cli.sh ssh node01 sudo -- dd if=/dev/zero of=loop0 bs=100M count=10
  ./cluster/cli.sh ssh node01 sudo -- losetup /dev/loop0 loop0
  cat <<EOF | ./cluster/kubectl.sh apply -f -
apiVersion: v1
kind: PersistentVolume
metadata:
  name: import-block-pv
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  local:
    path: /dev/loop0
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - node01
  persistentVolumeReclaimPolicy: Delete
  storageClassName: local
  volumeMode: Block
EOF
}
