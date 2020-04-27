#!/usr/bin/env bash

set -e

declare -A IMAGES
IMAGES[gocli]="gocli@sha256:220f55f6b1bcb3975d535948d335bd0e6b6297149a3eba1a4c14cad9ac80f80d"
if [ -z $KUBEVIRTCI_PROVISION_CHECK ]; then
    IMAGES[k8s-1.16.2]="k8s-1.16.2@sha256:5bae6a5f3b996952c5ceb4ba12ac635146425909801df89d34a592f3d3502b0c"
    IMAGES[ocp-4.3]="ocp-4.3@sha256:d293f0bca338136ed136b08851de780d710c9e40e2a1d18e5a5595491dbdd1ea"
fi
export IMAGES

IMAGE_SUFFIX=""
#if [[ $KUBEVIRT_PROVIDER =~ (ocp|okd).* ]]; then
#    IMAGE_SUFFIX="-provision"
#fi

image="${IMAGES[$KUBEVIRT_PROVIDER]:-${KUBEVIRT_PROVIDER}${IMAGE_SUFFIX}:latest}"
export image
