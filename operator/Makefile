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

# Builds grove-operator for the current platform.
.PHONY: build
build:
	@$(HACK_DIR)/build.sh

# Builds grove-operator for the specified platform.
# Usage: make build-<os>-<arch>
# Example: make build-linux-amd64, make build-darwin-amd64, make build-darwin-arm64, make build-linux-arm64
.PHONY: build-%
build-%:
	@GOOS=$$(echo $* | cut -d- -f 1) GOARCH=$$(echo $* | cut -d- -f 2) $(HACK_DIR)/build.sh

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

# Make targets for docker images
# -------------------------------------------------------------
# Usage: make docker-build-<arch>
# Example: make docker-build-amd64, make docker-build-arm64
.PHONY: docker-build-%
docker-build-%:
	@$(HACK_DIR)/docker-build.sh

# Make targets for deploying grove-operator to a local k8s cluster via skaffold
# -------------------------------------------------------------
.PHONY: deploy-local
deploy-local: $(SKAFFOLD)
	@$(HACK_DIR)/deploy-local.sh run -m grove-operator

.PHONY: deploy-local-dev
deploy-local-dev: $(SKAFFOLD)
	@$(HACK_DIR)/deploy-local.sh dev --cleanup=false -m grove-operator --trigger='manual'

.PHONY: deploy-local-debug
deploy-local-debug: $(SKAFFOLD)
	@$(HACK_DIR)/deploy-local.sh debug --cleanup=false -m grove-operator -p debug

.PHONY: undeploy-local
undeploy-local: $(SKAFFOLD)
	@$(SKAFFOLD) delete -m grove-operator