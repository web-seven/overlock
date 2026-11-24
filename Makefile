# Image for the operator.
IMG ?= ghcr.io/web-seven/overlock-operator:latest

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
	go build -o overlock ./cmd/overlock

.PHONY: build-operator
build-operator: manifests generate ## Build the overlock operator.
	go build -o bin/overlock-operator ./cmd/overlock-operator

.PHONY: run-operator
run-operator: manifests generate ## Run the operator against the current kubeconfig context.
	go run ./cmd/overlock-operator

.PHONY: docker-build-operator
docker-build-operator: ## Build the operator image.
	docker build -f Dockerfile.operator -t $(IMG) .

.PHONY: docker-push-operator
docker-push-operator: ## Push the operator image.
	docker push $(IMG)

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
	-$(KUBECTL) delete environments.overlock.io --all --wait --timeout=15m
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
