#!/usr/bin/env bash

set -ex

version_type=$1
previous_version=$(hack/version.sh)
released_version=$(hack/bump-version.sh $version_type)
commits=$(git log --pretty=format:"* %s" $previous_version..HEAD)

echo 'Build manifests for the new release'
VERSION=${released_version} IMAGE_TAG=${released_version} make gen-manifests
git add manifests/vm-import-operator/${released_version}

echo 'Upgrade README.md with the released manifests'
sed -i "s/\(.*kubectl apply.*\)${previous_version}\(.*\)/\1${released_version}\2/g" README.md

echo 'Bump versions in Makefile'
sed -i "s/VERSION_REPLACES ?= .*/VERSION_REPLACES ?= ${released_version}/" Makefile

echo 'Prepare release notes'
cat << EOF > version/description
$released_version

TODO: Add description here


TODO: keep at every category the
      commits that make sense

Features:
$commits

Bugs:
$commits

Docs:
$commits
EOF

${EDITOR:-vi} version/description

echo 'Commit updates'
git checkout -b release-$released_version
git commit -a -s -m "Release $released_version"
