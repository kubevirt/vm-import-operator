#!/bin/bash -e

git tag $TAG
git push https://github.com/kubevirt/vm-import-operator $TAG

$GITHUB_RELEASE release -u kubevirt -r vm-import-operator \
    --tag $TAG \
    --name $TAG \
    --description "$(cat $DESCRIPTION)"

for resource in "$@" ;do
    $GITHUB_RELEASE upload -u kubevirt -r vm-import-operator \
        --name $(basename $resource) \
        --tag $TAG \
        --file $resource
done
