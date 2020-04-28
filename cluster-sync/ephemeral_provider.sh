#!/usr/bin/env bash
set -e

source cluster-sync/install.sh

function verify() {
  echo 'Wait until all nodes are ready'
  until [[ $(_kubectl get nodes --no-headers | wc -l) -eq $(_kubectl get nodes --no-headers | grep " Ready" | wc -l) ]]; do
    sleep 1
  done
  echo "cluster node are ready!"
}

function configure_storage() {
  echo "Storage already configured ..."
}

function configure_nfs() {
  #Configure static nfs service and storage class, so we can create NFS PVs during test run.
  _kubectl apply -f ./cluster-sync/nfs/nfs-sc.yaml
  _kubectl apply -f ./cluster-sync/nfs/nfs-service.yaml -n $CDI_NAMESPACE
  _kubectl apply -f ./cluster-sync/nfs/nfs-server.yaml -n $CDI_NAMESPACE
  _kubectl patch storageclass nfs -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}
