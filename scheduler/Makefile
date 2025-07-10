# /*
# Copyright 2025 The Grove Authors.
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
KUBECONFIG_PATH := $(MODULE_HACK_DIR)/kind/kubeconfig
# Path of the package that ld-flags writes to
LD_PACKAGE_PATH := "k8s.io/component-base"
PROGRAM_NAME    := "kube-scheduler"

# Include tools targets
include $(REPO_HACK_DIR)/tools.mk

kind-up kind-down deploy-second-scheduler replace-default-scheduler: export KUBECONFIG = $(KUBECONFIG_PATH)

# the scheduler version should be different, not the same as the k8s version

# Builds the scheduler with the Grove plugin
.PHONY: build
build:
	@PACKAGE_PATH=${LD_PACKAGE_PATH} PROGRAM_NAME=${PROGRAM_NAME} $(MODULE_HACK_DIR)/build.sh

# Builds grove-kube-scheduler for the specified platform.
# Usage: make build-<os>-<arch>
# Example: make build-linux-amd64, make build-darwin-amd64, make build-darwin-arm64, make build-linux-arm64
.PHONY: build-%
build-%:
	@GOOS=$$(echo $* | cut -d- -f 1) GOARCH=$$(echo $* | cut -d- -f 2) PACKAGE_PATH=${LD_PACKAGE_PATH} PROGRAM_NAME=${PROGRAM_NAME} $(MODULE_HACK_DIR)/build.sh

# Lints the codebase using GOLANGCI_LINT.
.PHONY: lint
lint: $(GOLANGCI_LINT)
	@$(GOLANGCI_LINT) run -c $(REPO_ROOT)/.golangci.yaml ./...

# Formats the codebase
.PHONY: format
format: $(GOIMPORTS_REVISER)
	@$(REPO_HACK_DIR)/format.sh  ./api/ ./client/ ./cmd/ ./internal/ ./plugins/


# Make targets for tests
# -------------------------------------------------------------

# Run all unit tests
.PHONY: test-unit
test-unit:
	@go test ./...

# Run all unit tests with code coverage
.PHONY: test-cover
test-cover:
	@go test ./... -coverprofile=coverage.out

# Generate HTML coverage report
.PHONY: cover-html
cover-html: test-cover
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

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

# Make targets for deploying kube-scheduler to a local k8s cluster via skaffold (when applicable)
# -------------------------------------------------------------

# Deploys the grove plugin enabled kube-scheduler to a local k8s cluster using skaffold.
.PHONY: deploy-second-scheduler
deploy-second-scheduler: $(SKAFFOLD)
	@PACKAGE_PATH=${LD_PACKAGE_PATH} PROGRAM_NAME=${PROGRAM_NAME} $(MODULE_HACK_DIR)/deploy-local.sh

# Replaces the default kube-scheduler in kind, with the grove plugin enabled kube-scheduler.
.PHONY: replace-default-scheduler
replace-default-scheduler: $(SKAFFOLD) $(YQ)
	@PACKAGE_PATH=${LD_PACKAGE_PATH} PROGRAM_NAME=${PROGRAM_NAME} $(MODULE_HACK_DIR)/replace-scheduler.sh

# Resets the kube-scheduler in kind with the default scheduler. Run after `replace-default-scheduler`.
.PHONY: reset-scheduler
reset-scheduler:
	@$(MODULE_HACK_DIR)/reset-scheduler.sh
