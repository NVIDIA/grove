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

MODULE_ROOT     := $(shell dirname "$(realpath $(lastword $(MAKEFILE_LIST)))")
MODULE_HACK_DIR := $(MODULE_ROOT)/hack
REPO_ROOT       := $(shell dirname "$(MODULE_ROOT)")
REPO_HACK_DIR   := $(REPO_ROOT)/hack

# Include tools targets
include $(REPO_HACK_DIR)/tools.mk

# Builds grove-operator for the current platform.
.PHONY: build
build:
	@$(MODULE_HACK_DIR)/build.sh

# Builds grove-operator for the specified platform.
# Usage: make build-<os>-<arch>
# Example: make build-linux-amd64, make build-darwin-amd64, make build-darwin-arm64, make build-linux-arm64
.PHONY: build-%
build-%:
	@GOOS=$$(echo $* | cut -d- -f 1) GOARCH=$$(echo $* | cut -d- -f 2) $(MODULE_HACK_DIR)/build.sh

# Lints the codebase using GOLANGCI_LINT.
.PHONY: lint
lint: $(GOLANGCI_LINT)
	@$(GOLANGCI_LINT) run -c $(REPO_ROOT)/.golangci.yaml ./...

# Formats the codebase
.PHONY: format
format: $(GOIMPORTS_REVISER)
	@$(REPO_HACK_DIR)/format.sh ./api/ ./internal/ ./cmd/

# Add license headers to all files
.PHONY: add-license-headers
add-license-headers: $(GO_ADD_LICENSE)
	@$(REPO_HACK_DIR)/add-license-headers.sh

# Make targets for local development and testing
# -------------------------------------------------------------
# Starts a local k8s cluster using kind.
.PHONY: kind-up
kind-up: $(KIND) $(YQ)
	@$(MODULE_HACK_DIR)/kind-up.sh

# Stops the local k8s cluster.
.PHONY: kind-down
kind-down: $(KIND)
	@$(MODULE_HACK_DIR)/kind-down.sh

# Make targets for docker images
# -------------------------------------------------------------

# Builds the docker image for the specified platform.
# Usage: make docker-build-<arch>
# Example: make docker-build-amd64, make docker-build-arm64
.PHONY: docker-build-%
docker-build-%:
	@GOARCH=$$(echo $* | cut -d- -f 1) $(MODULE_HACK_DIR)/docker-build.sh

# Make targets for deploying grove-operator to a local k8s cluster via skaffold
# -------------------------------------------------------------
# Deploys grove-operator to a local k8s cluster using skaffold.
ifndef CERT_EXPIRY
override CERT_EXPIRY = 12h
endif

ifndef NAMESPACE
override NAMESPACE = default
endif

.PHONY: deploy-local
deploy-local: $(SKAFFOLD)
	@$(MODULE_HACK_DIR)/prepare-local-deploy.sh $(NAMESPACE) $(CERT_EXPIRY)
	@$(MODULE_HACK_DIR)/deploy-local.sh run -m grove-operator -n $(NAMESPACE)

# Deploys grove-operator to a local k8s cluster using skaffold in dev mode.
.PHONY: deploy-local-dev
deploy-local-dev: $(SKAFFOLD)
	@$(MODULE_HACK_DIR)/prepare-local-deploy.sh $(NAMESPACE) $(CERT_EXPIRY)
	@$(MODULE_HACK_DIR)/deploy-local.sh dev --cleanup=false --keep-running-on-failure -m grove-operator --trigger='manual' -n $(NAMESPACE)

# Deploys grove-operator to a local k8s cluster using skaffold in debug mode.
.PHONY: deploy-local-debug
deploy-local-debug: $(SKAFFOLD)
	@$(MODULE_HACK_DIR)/prepare-local-deploy.sh $(NAMESPACE) $(CERT_EXPIRY)
	@$(MODULE_HACK_DIR)/deploy-local.sh debug --cleanup=false -m grove-operator -p debug -n $(NAMESPACE)

# Undeploys grove-operator from a local k8s cluster using skaffold.
.PHONY: undeploy-local
undeploy-local: $(SKAFFOLD)
	@$(SKAFFOLD) delete -m grove-operator -n $(NAMESPACE)

# Deploys all the add-ons to a local k8s cluster.
.PHONY: deploy-addons
deploy-addons:
	@$(MODULE_HACK_DIR)/deploy-addons.sh --all

.PHONY: clean-chart-resources
clean-chart-resources:
	@rm -f $(MODULE_ROOT)/charts/crds/*.yaml
	@rm -rf $(MODULE_ROOT)/charts/pki-resources/*
