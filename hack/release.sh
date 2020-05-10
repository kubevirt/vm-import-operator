#!/bin/bash -e

git tag $TAG
git push $GITHUB_REPOSITORY $TAG

GITHUB_TOKEN=${GITHUB_TOKEN} $GITHUB_RELEASE release -u $GITHUB_USER -r vm-import-operator \
    --tag $TAG \
    --name $TAG \
    --description "$(cat $DESCRIPTION)" \
    $EXTRA_RELEASE_ARGS

for resource in "$@" ;do
    GITHUB_TOKEN=${GITHUB_TOKEN} $GITHUB_RELEASE upload -u $GITHUB_USER -r vm-import-operator \
        --name $(basename $resource) \
        --tag $TAG \
        --file $resource
done
