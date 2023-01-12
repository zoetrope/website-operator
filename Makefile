include common.mk

SHELL := /bin/bash
TAG ?= latest
CRD_OPTIONS = "crd:crdVersions=v1"

BIN_DIR := $(shell pwd)/bin

WEBSITE_OPERATOR = bin/website-operator
REPO_CHECKER = bin/repo-checker
UI = bin/ui
INSTALL_YAML = bin/install.yaml
GO_FILES := $(shell find . -type f -name '*.go')
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)


all: $(WEBSITE_OPERATOR) $(REPO_CHECKER) $(UI)

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kustomize build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	kustomize build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate setup-envtest ## fmt vet ## Run tests.
	source <($(SETUP_ENVTEST) use -p env); go test -v -count 1 ./...

.PHONY: dev
dev:
	ctlptl apply -f ./cluster.yaml
	$(MAKE) -C ./e2e/ setup-cluster

.PHONY: stop-dev
stop-dev:
	ctlptl delete -f ./cluster.yaml

##@ Build

$(WEBSITE_OPERATOR): $(GO_FILES) generate
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ ./cmd/website-operator

$(REPO_CHECKER): $(GO_FILES)
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ ./cmd/repo-checker

$(UI): $(GO_FILES)
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ ./cmd/ui

.PHONY: frontend
frontend:
	cd ui/frontend && npm install && npm run build

$(INSTALL_YAML):
	mkdir -p bin
	kustomize build ./config/release > $@

.PHONY: build-operator-image
build-operator-image: $(WEBSITE_OPERATOR)
	cp $(WEBSITE_OPERATOR) ./docker/website-operator
	docker build --no-cache -t ${REGISTRY}website-operator:${TAG} ./docker/website-operator

.PHONY: push-operator-image
push-operator-image:
	docker push ${REGISTRY}website-operator:${TAG}

.PHONY: build-checker-image
build-checker-image: $(REPO_CHECKER)
	cp $(REPO_CHECKER) ./docker/repo-checker
	docker build --no-cache -t ${REGISTRY}repo-checker:${TAG} ./docker/repo-checker

.PHONY: push-checker-image
push-checker-image:
	docker push ${REGISTRY}repo-checker:${TAG}

.PHONY: build-ui-image
build-ui-image: $(UI) frontend
	rm -f ./docker/ui/ui
	cp $(UI) ./docker/ui
	rm -rf ./docker/ui/dist
	cp -r ui/frontend/dist ./docker/ui/
	docker build --no-cache -t ${REGISTRY}website-operator-ui:${TAG} ./docker/ui

.PHONY: push-ui-image
push-ui-image:
	docker push ${REGISTRY}website-operator-ui:${TAG}

.PHONY: setup
setup: setup-envtest

SETUP_ENVTEST := $(BIN_DIR)/setup-envtest
.PHONY: setup-envtest
setup-envtest: ## Download setup-envtest locally if necessary
	# see https://github.com/kubernetes-sigs/controller-runtime/tree/master/tools/setup-envtest
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: clean
clean:
	rm -rf ./bin
	rm -f ./docker/website-operator/website-operator
	rm -f ./docker/repo-checker/repo-checker
