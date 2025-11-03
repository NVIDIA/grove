// /*
// Copyright 2024 The Grove Authors.
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

package validation

import (
	"testing"

	grovecorev1alpha1 "github.com/ai-dynamo/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

// TestValidateCliqueDependencies tests validation of clique dependencies for cycles and unknown cliques.
func TestValidateCliqueDependencies(t *testing.T) {
	fldPath := field.NewPath("spec", "template", "cliques")

	tests := []struct {
		// name identifies this test case
		name string
		// cliques is the list of clique templates to validate
		cliques []*grovecorev1alpha1.PodCliqueTemplateSpec
		// expectError indicates whether validation should fail
		expectError bool
		// errorContains is a substring expected in the error message
		errorContains string
	}{
		{
			name: "valid dependencies with no cycles",
			cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
				{
					Name: "clique1",
					Spec: grovecorev1alpha1.PodCliqueSpec{
						StartsAfter: []string{},
					},
				},
				{
					Name: "clique2",
					Spec: grovecorev1alpha1.PodCliqueSpec{
						StartsAfter: []string{"clique1"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "circular dependency between two cliques",
			cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
				{
					Name: "clique1",
					Spec: grovecorev1alpha1.PodCliqueSpec{
						StartsAfter: []string{"clique2"},
					},
				},
				{
					Name: "clique2",
					Spec: grovecorev1alpha1.PodCliqueSpec{
						StartsAfter: []string{"clique1"},
					},
				},
			},
			expectError:   true,
			errorContains: "circular dependencies",
		},
		{
			name: "dependency on unknown clique",
			cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
				{
					Name: "clique1",
					Spec: grovecorev1alpha1.PodCliqueSpec{
						StartsAfter: []string{"unknown-clique"},
					},
				},
			},
			expectError:   true,
			errorContains: "unknown clique names found",
		},
		{
			name: "three-way circular dependency",
			cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
				{
					Name: "clique1",
					Spec: grovecorev1alpha1.PodCliqueSpec{
						StartsAfter: []string{"clique3"},
					},
				},
				{
					Name: "clique2",
					Spec: grovecorev1alpha1.PodCliqueSpec{
						StartsAfter: []string{"clique1"},
					},
				},
				{
					Name: "clique3",
					Spec: grovecorev1alpha1.PodCliqueSpec{
						StartsAfter: []string{"clique2"},
					},
				},
			},
			expectError:   true,
			errorContains: "circular dependencies",
		},
		{
			name: "no dependencies passes validation",
			cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
				{
					Name: "clique1",
					Spec: grovecorev1alpha1.PodCliqueSpec{
						StartsAfter: []string{},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateCliqueDependencies(tt.cliques, fldPath)
			if tt.expectError {
				assert.NotEmpty(t, errs)
				if tt.errorContains != "" {
					errorString := ""
					for _, err := range errs {
						errorString += err.Error()
					}
					assert.Contains(t, errorString, tt.errorContains)
				}
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

// TestValidateScaleConfig tests validation of autoscaling configuration.
func TestValidateScaleConfig(t *testing.T) {
	fldPath := field.NewPath("spec", "autoScalingConfig")

	tests := []struct {
		// name identifies this test case
		name string
		// scaleConfig is the autoscaling configuration to validate
		scaleConfig *grovecorev1alpha1.AutoScalingConfig
		// minAvailable is the minimum available pods
		minAvailable int32
		// expectError indicates whether validation should fail
		expectError bool
		// errorContains is a substring expected in the error message
		errorContains string
	}{
		{
			name: "valid scale config",
			scaleConfig: &grovecorev1alpha1.AutoScalingConfig{
				MinReplicas: ptr.To(int32(2)),
				MaxReplicas: 5,
			},
			minAvailable: 1,
			expectError:  false,
		},
		{
			name: "minReplicas less than minAvailable returns error",
			scaleConfig: &grovecorev1alpha1.AutoScalingConfig{
				MinReplicas: ptr.To(int32(1)),
				MaxReplicas: 5,
			},
			minAvailable:  2,
			expectError:   true,
			errorContains: "must be greater than or equal to podCliqueSpec.minAvailable",
		},
		{
			name: "maxReplicas less than minReplicas returns error",
			scaleConfig: &grovecorev1alpha1.AutoScalingConfig{
				MinReplicas: ptr.To(int32(5)),
				MaxReplicas: 3,
			},
			minAvailable:  1,
			expectError:   true,
			errorContains: "must be greater than or equal to podCliqueSpec.minReplicas",
		},
		{
			name: "minReplicas equal to maxReplicas passes validation",
			scaleConfig: &grovecorev1alpha1.AutoScalingConfig{
				MinReplicas: ptr.To(int32(5)),
				MaxReplicas: 5,
			},
			minAvailable: 1,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateScaleConfig(tt.scaleConfig, tt.minAvailable, fldPath)
			if tt.expectError {
				assert.NotEmpty(t, errs)
				if tt.errorContains != "" {
					errorString := ""
					for _, err := range errs {
						errorString += err.Error()
					}
					assert.Contains(t, errorString, tt.errorContains)
				}
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}
