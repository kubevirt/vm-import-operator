#!/usr/bin/env bash

set -e

# The script is desinged to create conroller-related manifests.
# The manifests will be installed to allow the controller to run without
# the operator. This is for development purpose only.

PROJECT_ROOT="$(readlink -e $(dirname "$BASH_SOURCE[0]")/..)"
DEPLOY_DIR="${DEPLOY_DIR:-${PROJECT_ROOT}/build/_output/deploy}"
KUBEVIRT_NAMESPACE="${REPLACE_KUBEVIRT_NAMESPACE:-kubevirt-hyperconverged}"

mkdir -p $DEPLOY_DIR

go build -o ${PROJECT_ROOT}/tools/csv-generator/csv-generator ${PROJECT_ROOT}/tools/csv-generator/csv-generator.go
file="${DEPLOY_DIR}/vm-import-controller-local-manifests.yaml"
rendered=$( \
	${PROJECT_ROOT}/tools/csv-generator/csv-generator \
	--namespace=${KUBEVIRT_NAMESPACE} \
	--controller-only=true \
)
if [[ ! -z "$rendered" ]]; then
	echo -e "$rendered" > $file
fi

(cd ${PROJECT_ROOT}/tools/csv-generator && go clean)
echo $file