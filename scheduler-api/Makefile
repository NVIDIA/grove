
REPO_ROOT := $(shell dirname "$(realpath $(lastword $(MAKEFILE_LIST)))")
HACK_DIR  := $(REPO_ROOT)/hack

# Include tools targets
include $(HACK_DIR)/tools.mk

# Generates code and CRDs.
.PHONY: generate
generate: $(CODE_GENERATOR) $(CONTROLLER_GEN)
	@$(HACK_DIR)/generate.sh

# Lints the codebase using GOLANGCI_LINT.
.PHONY: lint
lint: $(GOLANGCI_LINT)
	@$(GOLANGCI_LINT) run -c ../.golangci.yaml ./...

# Formats the codebase
.PHONY: format
format: $(GOIMPORTS_REVISER)
	@$(HACK_DIR)/format.sh ./api/

# Add license headers to all files
.PHONY: add-license-headers
add-license-headers: $(GO_ADD_LICENSE)
	@$(HACK_DIR)/add-license-headers.sh
