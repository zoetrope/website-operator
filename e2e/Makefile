include ../common.mk

BINDIR = $(PWD)/bin
KIND := $(BINDIR)/kind
KUBECTL := $(BINDIR)/kubectl
KUSTOMIZE := $(BINDIR)/kustomize
KIND_CLUSTER_NAME=website-e2e

GO_FILES := $(shell find .. -path ../vendor -prune -o -path ../e2e -prune -o -name '*.go' -print)

launch-kind: $(KIND)
	if [ ! "$(shell $(KIND) get clusters | grep $(KIND_CLUSTER_NAME))" ]; then \
		$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config kind-config.yaml --image kindest/node:v$(KUBERNETES_VERSION) --wait 180s; \
	fi

shutdown-kind: $(KIND)
	if [ "$(shell $(KIND) get clusters | grep $(KIND_CLUSTER_NAME))" ]; then \
		$(KIND) delete cluster --name=$(KIND_CLUSTER_NAME) || true; \
	fi

setup-cluster: $(KUBECTL)
#	$(KUBECTL) apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml
#	$(KUBECTL) wait pod --all -n cert-manager --for condition=Ready --timeout 180s

	#$(KUBECTL) create ns prometheus
	#helm install prometheus stable/prometheus-operator --namespace=prometheus --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false
	#$(KUBECTL) wait pod --all -n prometheus --for condition=Ready --timeout 180s

	$(KUBECTL) apply -f https://projectcontour.io/quickstart/contour.yaml
	$(KUBECTL) wait pod -l app=contour -n projectcontour --for condition=Ready --timeout 180s
	$(KUBECTL) wait pod -l app=envoy -n projectcontour --for condition=Ready --timeout 180s

test: launch-kind load-images setup-cluster $(KUBECTL) $(KUSTOMIZE)
	$(KUBECTL) config use-context kind-$(KIND_CLUSTER_NAME)
	$(KUSTOMIZE) build ./manifests/manager | $(KUBECTL) apply -f -
	$(KUBECTL) wait pod -l control-plane=controller-manager -n website-operator-system --for condition=Ready --timeout 180s
	$(KUSTOMIZE) build ./manifests/website | $(KUBECTL) apply -f -
	env E2E_TEST=1 go test -count=1 -v . -args -ginkgo.progress -ginkgo.v -ginkgo.failFast

.PHONY: load-images
load-images: load-operator-images load-checker-image load-ui-image

.PHONY: load-operator-images
load-operator-images:
	cd ../ && make build-operator-image TAG=dev
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)website-operator:dev); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		$(KIND) load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)website-operator:dev; \
	fi

.PHONY: load-checker-images
load-checker-image:
	cd ../ && make build-checker-image TAG=dev
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)repo-checker:dev); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		$(KIND) load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)repo-checker:dev; \
	fi

.PHONY: load-ui-images
load-ui-image:
	cd ../ && make build-ui-image TAG=dev
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)website-operator-ui:dev); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		$(KIND) load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)website-operator-ui:dev; \
	fi

$(KUBECTL):
	mkdir -p $(BINDIR)
	curl -sfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	chmod 755 $(KUBECTL)

$(KUSTOMIZE):
	mkdir -p $(BINDIR)
	curl -sSLf https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz | tar xzf - > kustomize
	mv kustomize $(KUSTOMIZE)

$(KIND):
	mkdir -p $(BINDIR)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind@v$(KIND_VERSION))

.PHONY: clean
clean:
	rm -rf $(BINDIR)

define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(BINDIR) go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
