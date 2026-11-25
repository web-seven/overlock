# Image for the operator.
IMG ?= ghcr.io/web-seven/overlock-operator:latest
# Overlock K3s image, the operator image it bundles and its K3s version.
K3S_IMG ?= ghcr.io/web-seven/overlock-k3s:latest
K3S_OPERATOR_IMG ?= $(IMG)
K3S_VERSION ?= $(shell go run ./hack/k3s-image k3s-version)
K3S_PLATFORMS ?= linux/amd64,linux/arm64

LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_TOOLS_VERSION ?= v0.14.0
KUSTOMIZE_VERSION ?= v5.3.0
KUBECTL ?= kubectl

.PHONY: all
all: build build-operator

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate CRDs and RBAC from the API and controller markers.
	$(CONTROLLER_GEN) crd rbac:roleName=overlock-operator paths="./api/..." paths="./internal/controller/..." \
		output:crd:artifacts:config=config/crd/bases output:rbac:artifacts:config=config/rbac

.PHONY: generate
generate: controller-gen ## Generate DeepCopy implementations for the API types.
	$(CONTROLLER_GEN) object paths="./api/..."

.PHONY: fmt
fmt: ## Run go fmt.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: test
test: manifests generate ## Run tests.
	go test ./cmd/... ./pkg/... ./internal/... ./api/...

##@ Build

.PHONY: build
build: ## Build the overlock CLI.
	go build -o overlock ./cmd/cli

.PHONY: build-operator
build-operator: manifests generate ## Build the overlock operator.
	go build -o bin/overlock-operator ./cmd/operator

.PHONY: run-operator
run-operator: manifests generate ## Run the operator against the current kubeconfig context.
	go run ./cmd/operator

.PHONY: docker-build-operator
docker-build-operator: ## Build the operator image.
	docker build -f Dockerfile.operator -t $(IMG) .

.PHONY: docker-push-operator
docker-push-operator: ## Push the operator image.
	docker push $(IMG)

.PHONY: k3s-image-context
k3s-image-context: ## Generate the image list and operator manifests of the Overlock K3s image.
	go run ./hack/k3s-image images --operator-image $(K3S_OPERATOR_IMG) > k3s/images.txt
	go run ./hack/k3s-image manifests --operator-image $(K3S_OPERATOR_IMG) > k3s/operator.yaml

.PHONY: docker-build-k3s
docker-build-k3s: k3s-image-context ## Build the Overlock K3s image for the local platform.
	docker build --build-arg K3S_VERSION=$(K3S_VERSION) -t $(K3S_IMG) k3s

.PHONY: docker-push-k3s
docker-push-k3s: k3s-image-context ## Build and push the multi-arch Overlock K3s image.
	docker buildx build --platform $(K3S_PLATFORMS) --build-arg K3S_VERSION=$(K3S_VERSION) -t $(K3S_IMG) --push k3s

##@ Deployment

.PHONY: install
install: manifests kustomize ## Install CRDs into the current cluster.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Remove CRDs from the current cluster.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy the operator into the current cluster.
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Remove the operator. Environments are deleted first, while the operator still runs, so their deletion policy is applied.
	-$(KUBECTL) delete environments.overlock.app --all --wait --timeout=15m
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found -f -

##@ Tools

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: kustomize
kustomize: $(KUSTOMIZE)
$(KUSTOMIZE): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-24s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) }' $(MAKEFILE_LIST)
