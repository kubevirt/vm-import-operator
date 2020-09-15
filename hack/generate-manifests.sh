#!/usr/bin/env bash
set -e

PROJECT_ROOT="$(readlink -e $(dirname "$BASH_SOURCE[0]")/..)"
DEPLOY_DIR="${DEPLOY_DIR:-${PROJECT_ROOT}/manifests}"
CONTAINER_PREFIX="${CONTAINER_PREFIX:-quay.io/kubevirt}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
IMAGE_PULL_POLICY="${IMAGE_PULL_POLICY:-Always}"
KUBEVIRT_NAMESPACE="${REPLACE_KUBEVIRT_NAMESPACE:-kubevirt-hyperconverged}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-vm-import-operator}"
CONTROLLER_IMAGE="${CONTROLLER_IMAGE:-vm-import-controller}"
VIRTV2V_IMAGE="${VIRTV2V_IMAGE:-vm-import-virtv2v}"
CSV_VERSION_REPLACES=${VERSION_REPLACES//v}
CSV_VERSION=${VERSION//v}

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
		-e "s/{{IMAGE_TAG}}/$IMAGE_TAG/g" \
		-e "s/{{IMAGE_PULL_POLICY}}/$IMAGE_PULL_POLICY/g" \
        -e "s/{{OPERATOR_IMAGE}}/$OPERATOR_IMAGE/g" \
        -e "s/{{CONTROLLER_IMAGE}}/$CONTROLLER_IMAGE/g" \
        -e "s/{{VIRTV2V_IMAGE}}/$VIRTV2V_IMAGE/g" \
	$infile > $file
done

go build -o ${PROJECT_ROOT}/tools/csv-generator/csv-generator ${PROJECT_ROOT}/tools/csv-generator/csv-generator.go
file="${DEPLOY_DIR}/vm-import-operator/${VERSION}/vm-import-operator.${CSV_VERSION}.clusterserviceversion.yaml"
rendered=$( \
	${PROJECT_ROOT}/tools/csv-generator/csv-generator \
	--csv-version=${CSV_VERSION} \
	--replaces-csv-version=${CSV_VERSION_REPLACES} \
	--namespace=${KUBEVIRT_NAMESPACE} \
	--operator-version=${IMAGE_TAG} \
	--operator-image="${CONTAINER_PREFIX}/${OPERATOR_IMAGE}:${IMAGE_TAG}" \
	--controller-image="${CONTAINER_PREFIX}/${CONTROLLER_IMAGE}:${IMAGE_TAG}" \
	--virtv2v-image="${CONTAINER_PREFIX}/${VIRTV2V_IMAGE}:${IMAGE_TAG}" \
	--image-pull-policy=${IMAGE_PULL_POLICY} \
)
if [[ ! -z "$rendered" ]]; then
	echo -e "$rendered" > $file
fi

(cd ${PROJECT_ROOT}/tools/csv-generator && go clean)
