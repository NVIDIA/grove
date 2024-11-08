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

REPO_ROOT           := $(shell dirname "$(realpath $(lastword $(MAKEFILE_LIST)))")
HACK_DIR            := $(REPO_ROOT)/hack

# Include tools targets
include $(HACK_DIR)/tools.mk

.PHONY: build
build:
	@$(HACK_DIR)/build.sh

.PHONY: lint
lint: $(GOLANGCI_LINT)
	@$(GOLANGCI_LINT) run -c ../.golangci.yaml ./...

.PHONY: format
format: $(GOIMPORTS_REVISER)
	@$(HACK_DIR)/format.sh ./api/ ./internal/ ./cmd/

.PHONY: generate
generate: $(CODE_GENERATOR) $(CONTROLLER_GEN)
	@$(HACK_DIR)/generate.sh

.PHONY: add-license-headers
add-license-headers: $(GO_ADD_LICENSE)
	@$(HACK_DIR)/add-license-headers.sh

# Make targets for local development and testing
# -------------------------------------------------------------
.PHONY: kind-up
kind-up: $(KIND) $(YQ)
	@$(HACK_DIR)/kind-up.sh

.PHONY: kind-down
kind-down: $(KIND)
	@$(HACK_DIR)/kind-down.sh