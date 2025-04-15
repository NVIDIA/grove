TOOLS_DIR         := $(HACK_DIR)/tools
TOOLS_BIN_DIR     := $(TOOLS_DIR)/bin
GOLANGCI_LINT     := $(TOOLS_BIN_DIR)/golangci-lint
GO_ADD_LICENSE    := $(TOOLS_BIN_DIR)/addlicense
GOIMPORTS_REVISER := $(TOOLS_BIN_DIR)/goimports-reviser

# default tool versions
# -------------------------------------------------------------------------
GOLANGCI_LINT_VERSION     ?= v2.1.1
GOIMPORTS_REVISER_VERSION ?= v3.9.1
GO_ADD_LICENSE_VERSION    ?= v1.1.1

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
$(GOLANGCI_LINT):
	@# CGO_ENABLED has to be set to 1 in order for golangci-lint to be able to load plugins
	@# see https://github.com/golangci/golangci-lint/issues/1276
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) CGO_ENABLED=1 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(GOIMPORTS_REVISER):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/incu6us/goimports-reviser/v3@$(GOIMPORTS_REVISER_VERSION)

$(GO_ADD_LICENSE):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/google/addlicense@$(GO_ADD_LICENSE_VERSION)
