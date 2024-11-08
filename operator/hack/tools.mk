TOOLS_DIR 			:= $(HACK_DIR)/tools
TOOLS_BIN_DIR       := $(TOOLS_DIR)/bin

CONTROLLER_GEN      := $(TOOLS_BIN_DIR)/controller-gen
SETUP_ENVTEST       := $(TOOLS_BIN_DIR)/setup-envtest
KIND                := $(TOOLS_BIN_DIR)/kind
GOLANGCI_LINT       := $(TOOLS_BIN_DIR)/golangci-lint
GOIMPORTS_REVISER   := $(TOOLS_BIN_DIR)/goimports-reviser
CODE_GENERATOR	    := $(TOOLS_BIN_DIR)/code-generator
YQ					:= $(TOOLS_BIN_DIR)/yq
GO_ADD_LICENSE      := $(TOOLS_BIN_DIR)/addlicense

# default tool versions
# -------------------------------------------------------------------------
CONTROLLER_GEN_VERSION ?= $(call version_gomod,sigs.k8s.io/controller-tools)
KIND_VERSION ?= v0.24.0
GOLANGCI_LINT_VERSION ?= v1.60.3
GOIMPORTS_REVISER_VERSION ?= v3.6.5
CODE_GENERATOR_VERSION ?= $(call version_gomod,k8s.io/api)
YQ_VERSION ?= v4.44.3
GO_ADD_LICENSE_VERSION ?= v1.1.1

export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

# Common
# -------------------------------------------------------------------------
# Use this function to get the version of a go module from go.mod
version_gomod = $(shell go list -mod=mod -f '{{ .Version }}' -m $(1))

.PHONY: clean-tools-bin
clean-tools-bin:
	rm -rf $(TOOLS_BIN_DIR)/*

# Tools
# -------------------------------------------------------------------------

$(CONTROLLER_GEN):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

$(SETUP_ENVTEST):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install sigs.k8s.io/controller-runtime/tools/setup-envtest

$(GOLANGCI_LINT):
	@# CGO_ENABLED has to be set to 1 in order for golangci-lint to be able to load plugins
	@# see https://github.com/golangci/golangci-lint/issues/1276
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) CGO_ENABLED=1 go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(GOIMPORTS_REVISER):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/incu6us/goimports-reviser/v3@$(GOIMPORTS_REVISER_VERSION)

CODE_GENERATOR_ROOT = $(shell go env GOMODCACHE)/k8s.io/code-generator@$(CODE_GENERATOR_VERSION)
$(CODE_GENERATOR):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) GO111MODULE=on go install k8s.io/code-generator/cmd/client-gen@$(CODE_GENERATOR_VERSION)
	cp -f $(CODE_GENERATOR_ROOT)/kube_codegen.sh $(TOOLS_BIN_DIR)/

$(KIND):
	curl -Lo $(KIND) https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-$(SYSTEM_NAME)-$(SYSTEM_ARCH)
	chmod +x $(KIND)

$(YQ):
	curl -Lo $(YQ) https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$(SYSTEM_NAME)_$(SYSTEM_ARCH)
	chmod +x $(YQ)

$(GO_ADD_LICENSE):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/google/addlicense@$(GO_ADD_LICENSE_VERSION)
