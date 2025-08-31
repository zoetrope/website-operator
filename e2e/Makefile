KUBERNETES_VERSION := v1.33.4 # renovate: kindest/node
REGISTRY := ghcr.io/zoetrope/
KIND_CLUSTER_NAME=website-e2e

GO_FILES := $(shell find .. -path ../vendor -prune -o -path ../e2e -prune -o -name '*.go' -print)

launch-kind:
	if [ ! "$(shell kind get clusters | grep $(KIND_CLUSTER_NAME))" ]; then \
		kind create cluster --name=$(KIND_CLUSTER_NAME) --config kind-config.yaml --image kindest/node:$(KUBERNETES_VERSION) --wait 180s; \
	fi

shutdown-kind:
	if [ "$(shell kind get clusters | grep $(KIND_CLUSTER_NAME))" ]; then \
		kind delete cluster --name=$(KIND_CLUSTER_NAME) || true; \
	fi

setup-cluster:
	kubectl apply -f https://projectcontour.io/quickstart/contour.yaml
	kubectl wait -n projectcontour --for condition=available --all deployments --timeout 180s

test: launch-kind load-images setup-cluster
	kubectl config use-context kind-$(KIND_CLUSTER_NAME)
	kustomize build ./manifests/manager | kubectl apply -f -
	kubectl wait pod -l control-plane=controller-manager -n website-operator-system --for condition=Ready --timeout 180s
	kustomize build ./manifests/website | kubectl apply -f -
	env E2E_TEST=1 go test -count=1 -v . -args -ginkgo.v -ginkgo.fail-fast

.PHONY: load-images
load-images:
	cd ../ && goreleaser release --clean --snapshot --skip=publish
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)website-operator:dev-amd64); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		kind load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)website-operator:dev-amd64; \
	fi
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)repo-checker:dev-amd64); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		kind load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)repo-checker:dev-amd64; \
	fi
	ID=$$(docker image inspect --format='{{.ID}}' $(REGISTRY)website-operator-ui:dev-amd64); \
	if [ ! "$$(docker exec -it $(KIND_CLUSTER_NAME)-control-plane ctr --namespace=k8s.io images list | grep $$ID)" ]; then \
		kind load docker-image --name=$(KIND_CLUSTER_NAME) $(REGISTRY)website-operator-ui:dev-amd64; \
	fi
