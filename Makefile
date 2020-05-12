all: fmt check

# Always keep the last released version here
VERSION_REPLACES ?= v0.0.0
VERSION ?= v0.0.1
export VERSION := $(VERSION)

TARGET_NAMESPACE ?= kubevirt-hyperconverged
DEPLOY_DIR ?= manifests

# Image registry variables
QUAY_USER ?= $(USER)
IMAGE_REGISTRY ?= quay.io/$(QUAY_USER)
IMAGE_TAG ?= latest
OPERATOR_IMAGE ?= vm-import-operator
CONTROLLER_IMAGE ?= vm-import-controller

# Git parameters
GITHUB_REPOSITORY ?= https://github.com/kubevirt/vm-import-operator
GITHUB_USER ?= kubevirt
GITHUB_TOKEN ?=
EXTRA_RELEASE_ARGS ?=

TARGETS = \
	gen-k8s \
	gen-k8s-check \
	goimports \
	goimports-check \
	vet \
	whitespace \
	whitespace-check

export GOFLAGS=-mod=vendor
export GO111MODULE=on

GINKGO_EXTRA_ARGS ?=
GINKGO_ARGS ?= --v -r --progress $(GINKGO_EXTRA_ARGS)
GINKGO ?= build/_output/bin/ginkgo

OPERATOR_SDK ?= build/_output/bin/operator-sdk

GITHUB_RELEASE ?= build/_output/bin/github-release

$(GINKGO): go.mod
	GOBIN=$$(pwd)/build/_output/bin/ go install ./vendor/github.com/onsi/ginkgo/ginkgo

$(OPERATOR_SDK): go.mod
	GOBIN=$$(pwd)/build/_output/bin/ go install ./vendor/github.com/operator-framework/operator-sdk/cmd/operator-sdk

$(GITHUB_RELEASE): go.mod
	GOBIN=$$(pwd)/build/_output/bin/ go install ./vendor/github.com/aktau/github-release

# Make does not offer a recursive wildcard function, so here's one:
rwildcard=$(wildcard $1$2) $(foreach d,$(wildcard $1*),$(call rwildcard,$d/,$2))

# Gather needed source files and directories to create target dependencies
directories := $(filter-out ./ ./vendor/ ,$(sort $(dir $(wildcard ./*/))))
all_sources=$(call rwildcard,$(directories),*) $(filter-out $(TARGETS), $(wildcard *))
cmd_sources=$(call rwildcard,cmd/,*.go)
pkg_sources=$(call rwildcard,pkg/,*.go)
apis_sources=$(call rwildcard,pkg/apis,*.go)

fmt: whitespace goimports

goimports: $(cmd_sources) $(pkg_sources)
	go run ./vendor/golang.org/x/tools/cmd/goimports -w ./pkg ./cmd
	touch $@

whitespace: $(all_sources)
	./hack/whitespace.sh --fix
	touch $@

check: whitespace-check vet goimports-check gen-k8s-check test/unit

whitespace-check: $(all_sources)
	./hack/whitespace.sh
	touch $@

vet: $(cmd_sources) $(pkg_sources)
	go vet ./pkg/... ./cmd/...
	touch $@

goimports-check: $(cmd_sources) $(pkg_sources)
	go run ./vendor/golang.org/x/tools/cmd/goimports -d ./pkg ./cmd
	touch $@

test/unit: $(GINKGO)
	$(GINKGO) $(GINKGO_ARGS) ./pkg/ ./cmd/

controller-build:
	docker build -f build/controller/Dockerfile -t $(IMAGE_REGISTRY)/$(CONTROLLER_IMAGE):$(IMAGE_TAG) .

operator-build:
	docker build -f build/operator/Dockerfile -t $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE):$(IMAGE_TAG) .

docker-build: controller-build operator-build

controller-push:
	docker push $(IMAGE_REGISTRY)/$(CONTROLLER_IMAGE):$(IMAGE_TAG)

operator-push:
	docker push $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE):$(IMAGE_TAG)

docker-push: controller-push operator-push

cluster-up:
	./cluster/up.sh

cluster-down:
	./cluster/down.sh

cluster-sync: cluster-operator-push cluster-operator-install

cluster-operator-push:
	./cluster/operator-push.sh

cluster-operator-install:
	./cluster/operator-install.sh

cluster-clean:
	./cluster/clean.sh

gen-manifests:
	DEPLOY_DIR=$(DEPLOY_DIR) \
	CONTAINER_PREFIX=$(IMAGE_REGISTRY) \
	CONTAINER_TAG=$(IMAGE_TAG) \
	VERSION_REPLACES=$(VERSION_REPLACES) \
	REPLACE_KUBEVIRT_NAMESPACE=$(TARGET_NAMESPACE) \
	OPERATOR_IMAGE=$(OPERATOR_IMAGE) \
		./hack/generate-manifests.sh

gen-k8s: $(OPERATOR_SDK) $(apis_sources)
	$(OPERATOR_SDK) generate k8s
	GOFLAGS=-mod= ./hack/update-codegen.sh
	touch $@

gen-k8s-check: $(apis_sources)
	./hack/verify-codegen.sh
	touch $@

prepare-patch:
	./hack/prepare-release.sh patch
prepare-minor:
	./hack/prepare-release.sh minor
prepare-major:
	./hack/prepare-release.sh major

release: $(GITHUB_RELEASE)
	DESCRIPTION=version/description \
	GITHUB_RELEASE=$(GITHUB_RELEASE) \
	GITHUB_REPOSITORY=$(GITHUB_REPOSITORY) \
	GITHUB_USER=$(GITHUB_USER) \
	GITHUB_TOKEN=$(GITHUB_TOKEN) \
	EXTRA_RELEASE_ARGS=$(EXTRA_RELEASE_ARGS) \
	TAG=$(shell hack/version.sh) \
	  hack/release.sh \
	    $(shell find manifests/vm-import-operator/$(shell hack/version.sh) -type f)

vendor:
	go mod tidy
	go mod vendor

.PHONY: \
	all \
	check \
	docker-build \
	docker-push \
	gen-manifests \
	test/unit \
	prepare-patch \
	prepare-minor \
	prepare-major \
	vendor \
	gen-k8s \
	release
