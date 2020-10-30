include ../common.mk

KUBECTL := $(PWD)/bin/kubectl
KUSTOMIZE := $(PWD)/bin/kustomize
KIND_CLUSTER_NAME=website-e2e

GO_FILES := $(shell find .. -path ../vendor -prune -o -path ../e2e -prune -o -name '*.go' -print)

launch-kind: kind
	if [ ! "$(shell kind get clusters | grep $(KIND_CLUSTER_NAME))" ]; then \
		kind create cluster --name=$(KIND_CLUSTER_NAME) --config kind-config.yaml --image kindest/node:v$(KUBERNETES_VERSION) --wait 180s; \
		echo "127.0.0.1\twebsite-operator.example.com\n" | sudo tee -a /etc/hosts > /dev/null; \
	fi

shutdown-kind: kind
	if [ "$(shell kind get clusters | grep $(KIND_CLUSTER_NAME))" ]; then \
		kind delete cluster --name=$(KIND_CLUSTER_NAME) || true; \
		sudo sed -i -e '/website-operator.example.com/d' /etc/hosts; \
	fi

setup-cluster:
	$(KUBECTL) apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml
	$(KUBECTL) wait pod --all -n cert-manager --for condition=Ready --timeout 180s

	#$(KUBECTL) create ns prometheus
	#helm install prometheus stable/prometheus-operator --namespace=prometheus --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false
	$(KUBECTL) wait pod --all -n prometheus --for condition=Ready --timeout 180s

	$(KUBECTL) apply -f https://projectcontour.io/quickstart/contour.yaml
	$(KUBECTL) wait pod -l app=contour -n projectcontour --for condition=Ready --timeout 180s
	$(KUBECTL) wait pod -l app=envoy -n projectcontour --for condition=Ready --timeout 180s

#test: launch-kind load-images setup-cluster $(KUBECTL) $(KUSTOMIZE)
test: load-operator-images
	$(KUBECTL) config use-context kind-$(KIND_CLUSTER_NAME)
	$(KUSTOMIZE) build ./manifests/manager | $(KUBECTL) apply -f -
	$(KUSTOMIZE) build ./manifests/website | $(KUBECTL) apply -f -
	#env E2ETEST=1 go test -count=1 -v . -args -ginkgo.progress -ginkgo.v

.PHONY: load-images
load-images: load-operator-images load-checker-image load-nginx-image load-node-image load-elasticsearch-image

.PHONY: load-operator-images
load-operator-images:
	cd ../ && make build-operator-image TAG=dev
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)website-operator:dev); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		kind load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)website-operator:dev; \
	fi

.PHONY: load-checker-images
load-checker-image:
	cd ../ && make build-checker-image TAG=dev
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)repo-checker:dev); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		kind load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)repo-checker:dev; \
	fi

load-node-image:
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)node:12.19.0); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		kind load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)node:12.19.0; \
	fi

load-nginx-image:
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)nginx:1.18.0); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		kind load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)nginx:1.18.0; \
	fi

load-elasticsearch-image:
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)elasticsearch:7.9.3); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		kind load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)elasticsearch:7.9.3; \
	fi

$(KUBECTL):
	mkdir -p bin
	curl -sfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	chmod 755 $(KUBECTL)

$(KUSTOMIZE):
	mkdir -p bin
	curl -sSLf https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz | tar xzf - > kustomize
	mv kustomize $(KUSTOMIZE)

.PHONY: kind
kind:
	if [ ! "$(shell kind version | grep $(KIND_VERSION))" ]; then \
		cd /tmp; env GOFLAGS= GO111MODULE=on go get sigs.k8s.io/kind@v$(KIND_VERSION); \
	fi

.PHONY: clean
clean:
	rm -rf bin
