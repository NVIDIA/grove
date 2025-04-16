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

# Lints the entire codebase (all modules) using GOLANGCI_LINT.
.PHONY: lint
lint:
	@echo "> Linting operator"
	@make --directory=operator lint
	@echo "> Linting scheduler-api"
	@make --directory=scheduler-api lint
	@echo "> Linting scheduler-plugins"
	@make --directory=scheduler-plugins lint

# Formats the entire codebase (all modules)
.PHONY: format
format:
	@echo "> Formatting operator"
	@make --directory=operator format
	@echo "> Formatting scheduler-api"
	@make --directory=scheduler-api format
	@echo "> Formatting scheduler-plugins"
	@make --directory=scheduler-plugins format

# Generates code and CRDs for the entire codebase (all relevant modules)
.PHONY: generate
generate:
	@echo "> Generating code for operator"
	@make --directory=operator generate
	@echo "> Generating code for scheduler-api"
	@make --directory=scheduler-api generate

# Add license headers to all files (all modules)
.PHONY: add-license-headers
add-license-headers:
	@echo "> Adding license headers to operator"
	@make --directory=operator add-license-headers
	@echo "> Adding license headers to scheduler-api"
	@make --directory=scheduler-api add-license-headers
	@echo "> Adding license headers to scheduler-plugins"
	@make --directory=scheduler-plugins add-license-headers

