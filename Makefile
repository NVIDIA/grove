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
REPO_ROOT           := $(shell dirname "$(realpath $(lastword $(MAKEFILE_LIST)))")
REPO_HACK_DIR       := $(REPO_ROOT)/hack

include $(REPO_HACK_DIR)/tools.mk

.PHONY: build
build:
	@echo "> Building Grove Operator"
	@make --directory=operator build-operator
	@echo "> Building Grove Init Container"
	@make --directory=operator build-initc
	@echo "> Building Grove Scheduler"
	@make --directory=scheduler build

# Lints the entire codebase (all modules) using GOLANGCI_LINT.
.PHONY: lint
lint:
	@echo "> Linting operator/api"
	@make --directory=operator/api lint
	@echo "> Linting operator"
	@make --directory=operator lint
	@echo "> Linting scheduler/api"
	@make --directory=scheduler/api lint
	@echo "> Linting scheduler"
	@make --directory=scheduler lint

# Formats the entire codebase (all modules)
.PHONY: format
format:
	@echo "> Formatting operator"
	@make --directory=operator format
	@echo "> Formatting scheduler"
	@make --directory=scheduler format

# Generates code and CRDs for the entire codebase (all relevant modules)
.PHONY: generate
generate:
	@echo "> Generating code for operator api"
	@make --directory=operator/api generate
	@echo "> Generating code for scheduler api"
	@make --directory=scheduler/api generate

# Add license headers to all files
.PHONY: add-license-headers
add-license-headers: $(GO_ADD_LICENSE)
	@$(REPO_HACK_DIR)/add-license-headers.sh

# Generates API documentation for Grove Operator and Scheduler APIs
.PHONY: generate-api-docs
generate-api-docs: $(CRD_REF_DOCS)
	@$(REPO_HACK_DIR)/generate-api-docs.sh

# Runs unit tests for the entire codebase (all modules)
.PHONY: test-unit
test-unit:
	@echo "> Running tests for operator"
	@make --directory=operator test-unit
	@echo "> Running tests for scheduler"
	@make --directory=scheduler test-unit

.PHONY: test-cover
test-cover:
	@echo "> Running tests with coverage for operator"
	@make --directory=operator test-cover
	@echo "> Running tests with coverage for scheduler"
	@make --directory=scheduler test-cover

# Generates HTML coverage reports for the entire codebase (all modules)
.PHONY: cover-html
cover-html:
	@echo "> Generating HTML coverage report for operator"
	@make --directory=operator cover-html
	@echo "> Generating HTML coverage report for scheduler"
	@make --directory=scheduler cover-html
