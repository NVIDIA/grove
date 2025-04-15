# /*
# Copyright 2024 The Grove Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# */


TOOLS_DIR         := $(HACK_DIR)/tools
TOOLS_BIN_DIR     := $(TOOLS_DIR)/bin
CONTROLLER_GEN    := $(TOOLS_BIN_DIR)/controller-gen
GOLANGCI_LINT     := $(TOOLS_BIN_DIR)/golangci-lint
CODE_GENERATOR    := $(TOOLS_BIN_DIR)/code-generator
GO_ADD_LICENSE    := $(TOOLS_BIN_DIR)/addlicense
GOIMPORTS_REVISER := $(TOOLS_BIN_DIR)/goimports-reviser

# default tool versions
# -------------------------------------------------------------------------
CONTROLLER_GEN_VERSION    ?= $(call version_gomod,sigs.k8s.io/controller-tools)
CODE_GENERATOR_VERSION    ?= $(call version_gomod,k8s.io/api)
GOLANGCI_LINT_VERSION     ?= v2.1.1
GO_ADD_LICENSE_VERSION    ?= v1.1.1
GOIMPORTS_REVISER_VERSION ?= v3.9.1

export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

# Common
# -------------------------------------------------------------------------
# Use this function to get the version of a go module from go.mod
version_gomod = $(shell go list -mod=mod -f '{{ .Version }}' -m $(1))

.PHONY: clean-tools-bin
clean-tools-bin:
	@rm -rf $(TOOLS_BIN_DIR)/*

# Tools
# -------------------------------------------------------------------------

$(CONTROLLER_GEN):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

$(GOLANGCI_LINT):
	@# CGO_ENABLED has to be set to 1 in order for golangci-lint to be able to load plugins
	@# see https://github.com/golangci/golangci-lint/issues/1276
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) CGO_ENABLED=1 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(GOIMPORTS_REVISER):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/incu6us/goimports-reviser/v3@$(GOIMPORTS_REVISER_VERSION)

CODE_GENERATOR_ROOT = $(shell go env GOMODCACHE)/k8s.io/code-generator@$(CODE_GENERATOR_VERSION)
$(CODE_GENERATOR):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) GO111MODULE=on go install k8s.io/code-generator/cmd/client-gen@$(CODE_GENERATOR_VERSION)
	cp -f $(CODE_GENERATOR_ROOT)/kube_codegen.sh $(TOOLS_BIN_DIR)/

$(GO_ADD_LICENSE):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/google/addlicense@$(GO_ADD_LICENSE_VERSION)
