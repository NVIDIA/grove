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

package utils

import (
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
)

func TestFindScalingGroupConfigForClique(t *testing.T) {
	// Create test scaling group configurations
	scalingGroupConfigs := []grovecorev1alpha1.PodCliqueScalingGroupConfig{
		{
			Name:         "sga",
			CliqueNames:  []string{"pca", "pcb"},
			MinAvailable: func() *int32 { v := int32(2); return &v }(),
			Replicas:     func() *int32 { v := int32(5); return &v }(),
		},
		{
			Name:         "sgb",
			CliqueNames:  []string{"pcc", "pcd", "pce"},
			MinAvailable: func() *int32 { v := int32(1); return &v }(),
			Replicas:     func() *int32 { v := int32(3); return &v }(),
		},
		{
			Name:         "sgc",
			CliqueNames:  []string{"pcf"},
			MinAvailable: func() *int32 { v := int32(1); return &v }(),
			Replicas:     func() *int32 { v := int32(2); return &v }(),
		},
	}

	tests := []struct {
		name               string
		cliqueName         string
		expectedFound      bool
		expectedConfigName string
	}{
		{
			name:               "clique found in first scaling group",
			cliqueName:         "pca",
			expectedFound:      true,
			expectedConfigName: "sga",
		},
		{
			name:               "clique found in second scaling group",
			cliqueName:         "pcd",
			expectedFound:      true,
			expectedConfigName: "sgb",
		},
		{
			name:               "clique found in third scaling group",
			cliqueName:         "pcf",
			expectedFound:      true,
			expectedConfigName: "sgc",
		},
		{
			name:               "clique not found in any scaling group",
			cliqueName:         "nonexistent",
			expectedFound:      false,
			expectedConfigName: "",
		},
		{
			name:               "empty clique name",
			cliqueName:         "",
			expectedFound:      false,
			expectedConfigName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, found := FindScalingGroupConfigForClique(scalingGroupConfigs, tt.cliqueName)

			if found != tt.expectedFound {
				t.Errorf("FindScalingGroupConfigForClique() found = %v, expectedFound = %v", found, tt.expectedFound)
			}

			if tt.expectedFound {
				if config.Name != tt.expectedConfigName {
					t.Errorf("FindScalingGroupConfigForClique() config.Name = %v, expectedConfigName = %v", config.Name, tt.expectedConfigName)
				}
			} else {
				// When not found, config should be zero value
				if config.Name != "" {
					t.Errorf("FindScalingGroupConfigForClique() expected empty config when not found, got config.Name = %v", config.Name)
				}
			}
		})
	}
}

func TestFindScalingGroupConfigForClique_EmptyConfigs(t *testing.T) {
	// Test with empty slice of scaling group configurations
	var emptyConfigs []grovecorev1alpha1.PodCliqueScalingGroupConfig

	config, found := FindScalingGroupConfigForClique(emptyConfigs, "anyClique")

	if found {
		t.Errorf("FindScalingGroupConfigForClique() with empty configs should return found = false, got found = true")
	}

	if config.Name != "" {
		t.Errorf("FindScalingGroupConfigForClique() with empty configs should return empty config, got config.Name = %v", config.Name)
	}
}
