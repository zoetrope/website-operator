include common.mk

TAG ?= latest
CRD_OPTIONS ?= "crd:crdVersions=v1"

KUBEBUILDER_ASSETS := $(PWD)/bin
export KUBEBUILDER_ASSETS

CONTROLLER_GEN := $(PWD)/bin/controller-gen
KUBEBUILDER := $(PWD)/bin/kubebuilder
KUSTOMIZE := $(PWD)/bin/kustomize

WEBSITE_OPERATOR = build/website-operator
REPO_CHECKER = build/repo-checker
INSTALL_YAML = build/install.yaml
GO_FILES := $(shell find . -type f -name '*.go')
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

all: $(WEBSITE_OPERATOR) $(REPO_CHECKER)

# Run tests
test: generate manifests setup
	go test -race -v -count 1 ./...
	go vet ./...
	test -z $$(gofmt -s -l . | tee /dev/stderr)
	staticcheck ./...

# Build manager binary
$(WEBSITE_OPERATOR): $(GO_FILES) generate
	mkdir -p build
	go build -o $@ ./cmd/website-operator

$(REPO_CHECKER): $(GO_FILES)
	mkdir -p build
	go build -o $@ ./cmd/repo-checker

$(INSTALL_YAML): $(KUSTOMIZE)
	mkdir -p build
	$(KUSTOMIZE) build ./config/release > $@

# Generate manifests e.g. CRD, RBAC etc.
manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate code
generate: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
build-operator-image: $(WEBSITE_OPERATOR)
	cp $(WEBSITE_OPERATOR) ./docker/website-operator
	docker build --no-cache -t ${REGISTRY}website-operator:${TAG} ./docker/website-operator

# Push the docker image
push-operator-image:
	docker push ${REGISTRY}website-operator:${TAG}

build-checker-image: $(REPO_CHECKER)
	cp $(REPO_CHECKER) ./docker/repo-checker
	docker build --no-cache -t ${REGISTRY}repo-checker:${TAG} ./docker/repo-checker

# Push the docker image
push-checker-image:
	docker push ${REGISTRY}repo-checker:${TAG}

.PHONY: setup
setup: staticcheck $(KUBEBUILDER) $(CONTROLLER_GEN)

.PHONY: staticcheck
staticcheck:
	if ! which staticcheck >/dev/null; then \
		cd /tmp; env GOFLAGS= GO111MODULE=on go get honnef.co/go/tools/cmd/staticcheck; \
	fi

$(KUBEBUILDER):
	rm -rf tmp && mkdir -p tmp
	mkdir -p bin
	curl -sfL https://go.kubebuilder.io/dl/$(KUBEBUILDER_VERSION)/$(GOOS)/$(GOARCH) | tar -xz -C tmp/
	mv tmp/kubebuilder_$(KUBEBUILDER_VERSION)_$(GOOS)_$(GOARCH)/bin/* bin/
	curl -sfL https://github.com/kubernetes/kubernetes/archive/v$(KUBERNETES_VERSION).tar.gz | tar zxf - -C tmp/
	mv tmp/kubernetes-$(KUBERNETES_VERSION) tmp/kubernetes
	cd tmp/kubernetes; make all WHAT="cmd/kube-apiserver"
	mv tmp/kubernetes/_output/bin/kube-apiserver bin/
	rm -rf tmp

$(CONTROLLER_GEN):
	mkdir -p bin
	env GOBIN=$(PWD)/bin GOFLAGS= go install sigs.k8s.io/controller-tools/cmd/controller-gen

$(KUSTOMIZE):
	mkdir -p bin
	curl -sSLf https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz | tar xzf - > kustomize
	mv kustomize $(KUSTOMIZE)

.PHONY: clean
clean:
	rm -rf ./bin
	rm -rf ./build
	rm -f ./docker/website-operator/website-operator
	rm -f ./docker/repo-checker/repo-checker
