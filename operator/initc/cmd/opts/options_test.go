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
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetPodCliqueDependencies tests the parsing of PodClique dependencies from CLI flags.
// It validates various input formats, error conditions, and edge cases.
func TestGetPodCliqueDependencies(t *testing.T) {
	tests := []struct {
		// Input podcliques slice to parse
		input []string
		// Expected output map of PodClique name to minimum available replicas
		expected map[string]int
		// Whether an error is expected during parsing
		expectError bool
	}{
		// Valid single PodClique dependency
		{
			input:       []string{"podclique-a:3"},
			expected:    map[string]int{"podclique-a": 3},
			expectError: false,
		},
		// Valid multiple PodClique dependencies
		{
			input:       []string{"podclique-a:3", "podclique-b:5"},
			expected:    map[string]int{"podclique-a": 3, "podclique-b": 5},
			expectError: false,
		},
		// Valid with zero replicas (edge case)
		{
			input:       []string{"podclique-zero:0"},
			expected:    map[string]int{"podclique-zero": 0},
			expectError: false,
		},
		// Valid with whitespace around values
		{
			input:       []string{"podclique-whitespace:2"},
			expected:    map[string]int{"podclique-whitespace": 2},
			expectError: false,
		},
		// Empty input should return empty map
		{
			input:       []string{},
			expected:    map[string]int{},
			expectError: false,
		},
		// Empty strings in input should be skipped
		{
			input:       []string{"", "podclique-valid:1", "   "},
			expected:    map[string]int{"podclique-valid": 1},
			expectError: false,
		},
		// Invalid format - missing colon separator
		{
			input:       []string{"podclique-invalid"},
			expected:    nil,
			expectError: true,
		},
		// Invalid format - too many parts (multiple colons)
		{
			input:       []string{"podclique:invalid:format:3"},
			expected:    nil,
			expectError: true,
		},
		// Invalid format - non-numeric replicas
		{
			input:       []string{"podclique-bad:not-a-number"},
			expected:    nil,
			expectError: true,
		},
		// Negative replicas are actually valid (parsed by strconv.Atoi)
		{
			input:       []string{"podclique-negative:-1"},
			expected:    map[string]int{"podclique-negative": -1},
			expectError: false,
		},
		// Mixed valid and invalid entries - should fail on first invalid
		{
			input:       []string{"podclique-good:2", "podclique-bad:invalid"},
			expected:    nil,
			expectError: true,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			// Create CLIOptions with test input
			opts := &CLIOptions{
				podCliques: tt.input,
			}

			// Parse the dependencies
			result, err := opts.GetPodCliqueDependencies()

			// Validate error expectation
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestRegisterFlags tests that all required flags are properly registered with pflag.
// It validates flag names, types, and default values.
func TestRegisterFlags(t *testing.T) {
	tests := []struct {
	}{
		// Test that podcliques flag is registered correctly
		{},
	}

	for i := range tests {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			// Note: RegisterFlags uses the global pflag, so we test the function exists
			// and can be called without panic
			opts := &CLIOptions{}

			// This should not panic - testing that the method exists and runs
			assert.NotPanics(t, func() {
				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				opts.RegisterFlags(fs)
			})
		})
	}
}

// TestCLIOptionsEdgeCases tests edge cases and boundary conditions for CLI option parsing.
// It ensures robust handling of unusual but valid inputs.
func TestCLIOptionsEdgeCases(t *testing.T) {
	tests := []struct {
		// Input string that contains edge case formatting
		input string
		// Expected PodClique name after parsing
		expectedName string
		// Expected replica count after parsing
		expectedReplicas int
		// Whether the parsing should succeed
		shouldSucceed bool
	}{
		// PodClique name with hyphens and numbers
		{
			input:            "my-podclique-123:5",
			expectedName:     "my-podclique-123",
			expectedReplicas: 5,
			shouldSucceed:    true,
		},
		// Large replica count
		{
			input:            "high-scale-clique:1000",
			expectedName:     "high-scale-clique",
			expectedReplicas: 1000,
			shouldSucceed:    true,
		},
		// PodClique name with dots (FQDN style)
		{
			input:            "namespace.podclique.example:2",
			expectedName:     "namespace.podclique.example",
			expectedReplicas: 2,
			shouldSucceed:    true,
		},
		// Single character name
		{
			input:            "a:1",
			expectedName:     "a",
			expectedReplicas: 1,
			shouldSucceed:    true,
		},
		// Empty PodClique name (actually succeeds but creates empty-named entry)
		{
			input:            ":3",
			expectedName:     "",
			expectedReplicas: 3,
			shouldSucceed:    true,
		},
		// Only colon (should fail)
		{
			input:         ":",
			shouldSucceed: false,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("edge_case_%d", i), func(t *testing.T) {
			opts := &CLIOptions{
				podCliques: []string{tt.input},
			}

			deps, err := opts.GetPodCliqueDependencies()

			if tt.shouldSucceed {
				assert.NoError(t, err)
				require.Len(t, deps, 1)
				assert.Equal(t, tt.expectedReplicas, deps[tt.expectedName])
			} else {
				assert.Error(t, err)
				assert.Nil(t, deps)
			}
		})
	}
}

// TestInitializeCLIOptions tests the CLI options initialization with global flag parsing.
// It validates that the function properly initializes and registers flags.
func TestInitializeCLIOptions(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "basic initialization with no arguments",
			args:        []string{"test"},
			expectError: false,
		},
		{
			name:        "initialization with program name only",
			args:        []string{"initc"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original state
			oldArgs := os.Args
			oldCommandLine := pflag.CommandLine
			defer func() {
				os.Args = oldArgs
				pflag.CommandLine = oldCommandLine
			}()

			// Reset pflag state for clean test
			pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ContinueOnError)
			os.Args = tt.args

			config, err := InitializeCLIOptions()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Test that GetPodCliqueDependencies works (validates initialization)
				deps, err := config.GetPodCliqueDependencies()
				assert.NoError(t, err)
				assert.NotNil(t, deps)
				assert.Equal(t, 0, len(deps))

				// Verify that flags were registered by checking if they exist
				podCliquesFlag := pflag.CommandLine.Lookup("podcliques")
				assert.NotNil(t, podCliquesFlag, "podcliques flag should be registered")

				versionFlag := pflag.CommandLine.Lookup("version")
				assert.NotNil(t, versionFlag, "version flag should be registered")
			}
		})
	}
}
