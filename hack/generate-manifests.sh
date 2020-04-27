#!/usr/bin/env bash
set -e

PROJECT_ROOT="$(readlink -e $(dirname "$BASH_SOURCE[0]")/..)"
DEPLOY_DIR="${DEPLOY_DIR:-${PROJECT_ROOT}/manifests}"
CONTAINER_PREFIX="${CONTAINER_PREFIX:-quay.io/kubevirt}"
CONTAINER_TAG="${CONTAINER_TAG:-latest}"
IMAGE_PULL_POLICY="${IMAGE_PULL_POLICY:-Always}"
KUBEVIRT_NAMESPACE="${REPLACE_KUBEVIRT_NAMESPACE:-kubevirt-hyperconverged}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-vm-import-operator}"

templates=$(cd ${PROJECT_ROOT}/templates && find . -type f -name "*.yaml.in")
for template in $templates; do
	infile="${PROJECT_ROOT}/templates/${template}"

	dir="$(dirname ${DEPLOY_DIR}/${template})"
	dir=${dir/VERSION/$VERSION}
	mkdir -p ${dir}

	file="${dir}/$(basename -s .in $template)"
	file=${file/VERSION/$VERSION}

	sed -e "s/{{NAMESPACE}}/$KUBEVIRT_NAMESPACE/g" \
	    -e "s#{{CONTAINER_PREFIX}}#$CONTAINER_PREFIX#g" \
		-e "s/{{CONTAINER_TAG}}/$CONTAINER_TAG/g" \
		-e "s/{{IMAGE_PULL_POLICY}}/$IMAGE_PULL_POLICY/g" \
        -e "s/{{OPERATOR_IMAGE}}/$OPERATOR_IMAGE/g" \
	$infile > $file
done

# TODO: Will be useful once examples are generated per version
templates=$(cd ${PROJECT_ROOT}/templates && find . -type f -name "*example_cr.yaml")
for template in $templates; do
    infile="${PROJECT_ROOT}/templates/${template}"
	dir="$(dirname ${DEPLOY_DIR}/${template})"
	dir=${dir/VERSION/$VERSION/examples}
	mkdir -p ${dir}
	cp $infile ${dir}
done