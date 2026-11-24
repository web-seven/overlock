LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.14.0

.PHONY: manifests
manifests: generate controller-gen ## Generate CRDs from the API types.
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate DeepCopy implementations for the API types.
	$(CONTROLLER_GEN) object paths="./api/..."

.PHONY: build
build: ## Build the overlock CLI.
	go build -o overlock ./cmd/overlock

.PHONY: test
test: ## Run tests.
	go test ./cmd/... ./pkg/... ./internal/... ./api/...

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
