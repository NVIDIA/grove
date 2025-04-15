REPO_ROOT := $(shell dirname "$(realpath $(lastword $(MAKEFILE_LIST)))")
HACK_DIR  := $(REPO_ROOT)/hack

include $(HACK_DIR)/tools.mk

KUBERNETES_VERSION=$(call version_gomod,k8s.io/kubernetes)
SCHEDULER_DEV_VERSION=$(KUBERNETES_VERSION)-dev

# Builds the scheduler with the Grove plugin
.PHONY: build
build:
	@go build -ldflags '-X k8s.io/component-base/version.gitVersion=$(SCHEDULER_DEV_VERSION)' -o bin/kube-scheduler cmd/main.go

# Lints the codebase using GOLANGCI_LINT.
.PHONY: lint
lint: $(GOLANGCI_LINT)
	@$(GOLANGCI_LINT) run -c ../.golangci.yaml ./...

# Formats the codebase
.PHONY: format
format: $(GOIMPORTS_REVISER)
	@$(HACK_DIR)/format.sh  ./cmd/ ./grovescheduling/ ./internal/

# Add license headers to all files
.PHONY: add-license-headers
add-license-headers: $(GO_ADD_LICENSE)
	@$(HACK_DIR)/add-license-headers.sh
