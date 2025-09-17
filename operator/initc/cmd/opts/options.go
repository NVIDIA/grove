// /*
// Copyright 2025 The Grove Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package opts

import (
	"fmt"
	"strconv"
	"strings"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/version"

	"github.com/spf13/pflag"
)

// Error codes for CLI option parsing.
const (
	errCodeInvalidInput grovecorev1alpha1.ErrorCode = "ERR_INVALID_INPUT"
)

// Operation names for error tracking.
const (
	operationParseFlag = "OperationParseFlag"
)

// CLIOptions defines the configuration that is passed to the init container.
// It contains the PodClique dependencies with their minimum available replica requirements.
type CLIOptions struct {
	// podCliques contains PodClique names and their minimum available replicas in the format "name:replicas"
	podCliques []string
}

// RegisterFlags registers all the command line flags for the init container.
// Flags include PodClique dependencies and version information.
func (c *CLIOptions) RegisterFlags(fs *pflag.FlagSet) {
	// Register the podcliques flag for specifying dependencies
	// Format: --podcliques=<podclique-fqn>:<minAvailable-replicas>
	// Example: --podcliques=podclique-a:3 --podcliques=podclique-b:4
	pflag.StringArrayVarP(&c.podCliques, "podcliques", "p", nil, "podclique name and minAvailable replicas seperated by comma, repeated for each podclique")
	// Register version flags
	version.AddFlags(fs)
}

// GetPodCliqueDependencies parses the PodClique flag values and returns them as a map
// where the key is the PodClique name and the value is the minimum available replicas required.
func (c *CLIOptions) GetPodCliqueDependencies() (map[string]int, error) {
	// Initialize the result map
	podCliqueDependencies := make(map[string]int)

	// Parse each PodClique dependency specification
	for _, pair := range c.podCliques {
		// Skip empty entries
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split the name:replicas format
		nameAndMinAvailable := strings.Split(pair, ":")
		if len(nameAndMinAvailable) != 2 {
			return nil, groveerr.New(errCodeInvalidInput, operationParseFlag, fmt.Sprintf("expected two values per podclique, found %d", len(nameAndMinAvailable)))
		}

		// Parse the replica count as an integer
		replicas, err := strconv.Atoi(nameAndMinAvailable[1])
		if err != nil {
			return nil, groveerr.WrapError(err, errCodeInvalidInput, operationParseFlag, "failed to convert replicas to int")
		}

		// Store the PodClique name and its minimum available replicas
		podCliqueDependencies[strings.TrimSpace(nameAndMinAvailable[0])] = replicas
	}

	return podCliqueDependencies, nil
}

// InitializeCLIOptions creates a new CLIOptions instance and parses command line flags.
// It registers all available flags and returns the parsed configuration.
func InitializeCLIOptions() (CLIOptions, error) {
	// Initialize the configuration with empty PodClique list
	config := CLIOptions{
		podCliques: make([]string, 0),
	}
	// Register all command line flags
	config.RegisterFlags(pflag.CommandLine)
	// Parse the command line arguments
	pflag.Parse()
	return config, nil
}
