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

package validation

import (
	"errors"
	"fmt"
	"testing"
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

// Helper function to create a dummy valid PodGangSet
// with a minimal configuration
// This is used to test the validation logic without needing a full setup.
func createDummyPodGangSet(name string) *grovecorev1alpha1.PodGangSet {
	return &grovecorev1alpha1.PodGangSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: grovecorev1alpha1.PodGangSetSpec{
			Replicas: 1,
			Template: grovecorev1alpha1.PodGangSetTemplateSpec{
				TerminationDelay: &metav1.Duration{Duration: 30 * time.Second},
				StartupType:      ptr.To(grovecorev1alpha1.CliqueStartupTypeAnyOrder),
				Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
					{
						Name:        "test",
						Labels:      nil,
						Annotations: nil,
						Spec: grovecorev1alpha1.PodCliqueSpec{
							Replicas:     1,
							RoleName:     "dummy-role",
							MinAvailable: ptr.To[int32](1),
						},
					},
				},
			},
		},
	}
}

// createDummyPodCliqueTemplate Helper function to create a valid PodCliqueTemplateSpec with predefined values
// This is used to test the validation logic without needing a full setup.
// It creates a PodCliqueTemplateSpec with a single container and minimal configuration.
func createDummyPodCliqueTemplate(name string) *grovecorev1alpha1.PodCliqueTemplateSpec {
	return &grovecorev1alpha1.PodCliqueTemplateSpec{
		Name: name,
		Spec: grovecorev1alpha1.PodCliqueSpec{
			Replicas:     1,
			MinAvailable: ptr.To(int32(1)),
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "test:latest",
					},
				},
			},
			RoleName: fmt.Sprintf("dummy-%s-role", name),
		},
	}
}

// Helper function to create a PodCliqueScalingGroupConfig
func createScalingGroupConfig(name string, cliqueNames []string) grovecorev1alpha1.PodCliqueScalingGroupConfig {
	return grovecorev1alpha1.PodCliqueScalingGroupConfig{
		Name:        name,
		CliqueNames: cliqueNames,
	}
}

func TestPodCliqueTemplateNameValidation(t *testing.T) {
	testCases := []struct {
		description     string
		pgsName         string
		cliqueNames     []string
		expectError     bool
		expectedErrMsg  string
		expectedErrPath string
	}{
		{
			description: "Valid PodClique template names",
			pgsName:     "inference",
			cliqueNames: []string{"prefill", "decode"},
			expectError: false,
		},
		{
			description:     "PodClique template name that exceeds character limit",
			pgsName:         "verylongpodgangsetnamethatisverylong",            // 34 chars
			cliqueNames:     []string{"verylongpodcliquenamethatexceedslimit"}, // 37 chars, total 34+37=71 > 45
			expectError:     true,
			expectedErrMsg:  "combined resource name length",
			expectedErrPath: "spec.template.cliques.name",
		},
		{
			description:     "Empty PodClique template name",
			pgsName:         "inference",
			cliqueNames:     []string{""},
			expectError:     true,
			expectedErrMsg:  "field cannot be empty",
			expectedErrPath: "spec.template.cliques.name",
		},
		{
			description:     "PodClique template name with invalid characters",
			pgsName:         "inference",
			cliqueNames:     []string{"prefill_worker"},
			expectError:     true,
			expectedErrMsg:  "invalid PodCliqueTemplateSpec name",
			expectedErrPath: "spec.template.cliques.name",
		},
		{
			description: "Multiple PodClique templates with valid names",
			pgsName:     "inference",
			cliqueNames: []string{"prefill", "decode", "embed"},
			expectError: false,
		},
		{
			description:     "Multiple PodClique templates with one invalid name",
			pgsName:         "inference",
			cliqueNames:     []string{"prefill", "verylongpodcliquenamethatexceedslimit"},
			expectError:     true,
			expectedErrMsg:  "combined resource name length",
			expectedErrPath: "spec.template.cliques.name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			pgs := createDummyPodGangSet(tc.pgsName)

			// Add PodClique templates
			for _, cliqueName := range tc.cliqueNames {
				pgs.Spec.Template.Cliques = append(pgs.Spec.Template.Cliques, createDummyPodCliqueTemplate(cliqueName))
			}

			validator := newPGSValidator(pgs, admissionv1.Create)
			warnings, err := validator.validate()

			if !tc.expectError {
				assert.NoError(t, err, "Expected no validation error for test case: %s", tc.description)
			} else {
				assert.Error(t, err, "Expected validation error for test case: %s", tc.description)
				assert.Contains(t, err.Error(), tc.expectedErrMsg, "Error message should contain expected text")
				// Check if this is an aggregate error with field errors
				var aggErr utilerrors.Aggregate
				if errors.As(err, &aggErr) && len(aggErr.Errors()) > 0 {
					var fieldErr *field.Error
					if errors.As(aggErr.Errors()[0], &fieldErr) {
						assert.Contains(t, fieldErr.Field, tc.expectedErrPath, "Error field path should match expected")
					}
				}
			}

			// Warnings should be empty for these test cases
			assert.Empty(t, warnings, "No warnings expected for these test cases")
		})
	}
}

func TestScalingGroupPodCliqueNameValidation(t *testing.T) {
	testCases := []struct {
		description     string
		pgsName         string
		scalingGroups   []grovecorev1alpha1.PodCliqueScalingGroupConfig
		cliqueTemplates []string
		expectError     bool
		expectedErrMsg  string
		expectedErrPath string
	}{
		{
			description: "Valid scaling group with valid PodClique names",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				createScalingGroupConfig("workers", []string{"prefill", "decode"}),
			},
			cliqueTemplates: []string{"prefill", "decode"},
			expectError:     false,
		},
		{
			description: "Scaling group with PodClique names exceeding character limit",
			pgsName:     "verylongpodgangsetname",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				createScalingGroupConfig("verylongscalinggroup", []string{"verylongpodcliquename"}),
			},
			cliqueTemplates: []string{"verylongpodcliquename"},
			expectError:     true,
			expectedErrMsg:  "combined resource name length",
			expectedErrPath: "spec.template.podCliqueScalingGroups.cliqueNames.name",
		},
		{
			description: "Multiple scaling groups with valid names",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				createScalingGroupConfig("workers", []string{"prefill"}),
				createScalingGroupConfig("embedders", []string{"embed"}),
			},
			cliqueTemplates: []string{"prefill", "embed"},
			expectError:     false,
		},
		{
			description: "Scaling group with mixed valid/invalid PodClique names",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				createScalingGroupConfig("workers", []string{"prefill", "verylongpodcliquenamethatexceedslimit"}),
			},
			cliqueTemplates: []string{"prefill", "verylongpodcliquenamethatexceedslimit"},
			expectError:     true,
			expectedErrMsg:  "combined resource name length",
			expectedErrPath: "spec.template.podCliqueScalingGroups.cliqueNames.name",
		},
		{
			description: "Scaling group referencing non-existent PodClique",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				createScalingGroupConfig("workers", []string{"nonexistent"}),
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     true,
			expectedErrMsg:  "unidentified PodClique names found",
			expectedErrPath: "spec.template.podCliqueScalingGroups.cliqueNames",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			pgs := createDummyPodGangSet(tc.pgsName)

			// Add PodClique templates
			for _, cliqueName := range tc.cliqueTemplates {
				pgs.Spec.Template.Cliques = append(pgs.Spec.Template.Cliques, createDummyPodCliqueTemplate(cliqueName))
			}

			// Add scaling groups
			pgs.Spec.Template.PodCliqueScalingGroupConfigs = tc.scalingGroups

			validator := newPGSValidator(pgs, admissionv1.Create)
			warnings, err := validator.validate()

			if tc.expectError {
				assert.Error(t, err, "Expected validation error for test case: %s", tc.description)
				assert.Contains(t, err.Error(), tc.expectedErrMsg, "Error message should contain expected text")
			} else {
				assert.NoError(t, err, "Expected no validation error for test case: %s", tc.description)
			}

			// Warnings should be empty for these test cases
			assert.Empty(t, warnings, "No warnings expected for these test cases")
		})
	}
}

func TestPodGangSetIntegrationValidation(t *testing.T) {
	testCases := []struct {
		description      string
		pgsName          string
		cliqueTemplates  []string
		scalingGroups    []grovecorev1alpha1.PodCliqueScalingGroupConfig
		expectError      bool
		expectedErrCount int
		expectedErrMsgs  []string
	}{
		{
			description:     "Valid PodGangSet with templates and scaling groups",
			pgsName:         "inference",
			cliqueTemplates: []string{"prefill", "decode", "embed"},
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				createScalingGroupConfig("workers", []string{"prefill", "decode"}),
			},
			expectError: false,
		},
		{
			description:     "PodGangSet with invalid template and scaling group names",
			pgsName:         "verylongpodgangsetname",
			cliqueTemplates: []string{"verylongpodcliquename"},
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				createScalingGroupConfig("verylongscalinggroup", []string{"verylongpodcliquename"}),
			},
			expectError:      true,
			expectedErrCount: 3, // Multiple field paths get validation errors (name, scaling group name, metadata name)
			expectedErrMsgs:  []string{"combined resource name length"},
		},
		{
			description:     "PodGangSet with maximum valid character usage",
			pgsName:         "pgs",                            // 3 chars
			cliqueTemplates: []string{"cliquename20charssss"}, // 20 chars total (3+20=23, well under 45)
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				createScalingGroupConfig("sg", []string{"cliquename20charssss"}), // 3+2+20=25, under 45
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			pgs := createDummyPodGangSet(tc.pgsName)

			// Add PodClique templates
			for _, cliqueName := range tc.cliqueTemplates {
				pgs.Spec.Template.Cliques = append(pgs.Spec.Template.Cliques, createDummyPodCliqueTemplate(cliqueName))
			}

			// Add scaling groups
			pgs.Spec.Template.PodCliqueScalingGroupConfigs = tc.scalingGroups

			validator := newPGSValidator(pgs, admissionv1.Create)
			warnings, err := validator.validate()

			if !tc.expectError {
				assert.NoError(t, err, "Expected no validation error for test case: %s", tc.description)
			} else {
				assert.Error(t, err, "Expected validation error for test case: %s", tc.description)
				// Check error count if specified
				if tc.expectedErrCount > 0 {
					var aggErr utilerrors.Aggregate
					if errors.As(err, &aggErr) {
						assert.Len(t, aggErr.Errors(), tc.expectedErrCount, "Expected specific number of validation errors")
					}
				}

				// Check error messages if specified
				for _, expectedMsg := range tc.expectedErrMsgs {
					assert.Contains(t, err.Error(), expectedMsg, "Error should contain expected message")
				}
			}

			// Warnings should be empty for these test cases
			assert.Empty(t, warnings, "No warnings expected for these test cases")
		})
	}
}

func TestErrorFieldPaths(t *testing.T) {
	testCases := []struct {
		description       string
		pgsName           string
		cliqueNames       []string
		scalingGroups     []grovecorev1alpha1.PodCliqueScalingGroupConfig
		expectedFieldErrs []string
	}{
		{
			description: "Template name validation error paths",
			pgsName:     "verylongpodgangsetnamethatisverylong",            // 34 chars
			cliqueNames: []string{"verylongpodcliquenamethatexceedslimit"}, // 37 chars, total > 45
			expectedFieldErrs: []string{
				"spec.template.cliques.name",
				"metadata.name",
			},
		},
		{
			description: "Scaling group name validation error paths",
			pgsName:     "verylongpodgangsetname",
			cliqueNames: []string{"validname"},
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				createScalingGroupConfig("verylongscalinggroup", []string{"verylongpodcliquename"}),
			},
			expectedFieldErrs: []string{
				"spec.template.podCliqueScalingGroups.cliqueNames.name",
				"spec.template.podCliqueScalingGroups.name",
				"metadata.name",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			pgs := createDummyPodGangSet(tc.pgsName)

			// Add PodClique templates
			for _, cliqueName := range tc.cliqueNames {
				pgs.Spec.Template.Cliques = append(pgs.Spec.Template.Cliques, createDummyPodCliqueTemplate(cliqueName))
			}

			// Add scaling groups
			pgs.Spec.Template.PodCliqueScalingGroupConfigs = tc.scalingGroups

			validator := newPGSValidator(pgs, admissionv1.Create)
			_, err := validator.validate()

			assert.Error(t, err, "Expected validation error")

			var actualFieldPaths []string
			if aggErr, ok := err.(utilerrors.Aggregate); ok {
				for _, e := range aggErr.Errors() {
					if fieldErr, ok := e.(*field.Error); ok {
						actualFieldPaths = append(actualFieldPaths, fieldErr.Field)
					}
				}
			}

			// Check that all expected field paths are present
			for _, expectedPath := range tc.expectedFieldErrs {
				found := false
				for _, actualPath := range actualFieldPaths {
					if actualPath == expectedPath {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected field path %s not found in actual errors: %v", expectedPath, actualFieldPaths)
			}
		})
	}
}

func TestValidatePodNameConstraints(t *testing.T) {
	testCases := []struct {
		description string
		pgsName     string
		pcsgName    string
		pclqName    string
		expectError bool
	}{
		{
			description: "Valid PCSG component names",
			pgsName:     "inference",
			pcsgName:    "workers",
			pclqName:    "prefill",
			expectError: false,
		},
		{
			description: "Valid non-PCSG component names",
			pgsName:     "inference",
			pcsgName:    "",
			pclqName:    "prefill",
			expectError: false,
		},
		{
			description: "Resource names exceed 45 characters",
			pgsName:     "verylongpgsname",
			pcsgName:    "verylongpcsgname",
			pclqName:    "verylongpclqname",
			expectError: true,
		},
		{
			description: "Non-PCSG resource names exceed 45 characters",
			pgsName:     "verylongpodgangsetnamethatisverylong",
			pcsgName:    "",
			pclqName:    "verylongpodcliquename",
			expectError: true,
		},
		{
			description: "Maximum valid character usage for PCSG",
			pgsName:     "pgs",
			pcsgName:    "sg",
			pclqName:    "cliquename20charssss",
			expectError: false,
		},
		{
			description: "Maximum valid character usage for non-PCSG",
			pgsName:     "pgsnametwentychars123",
			pcsgName:    "",
			pclqName:    "cliquenametwentyfourchar",
			expectError: false,
		},
		{
			description: "Edge case - exactly 45 characters",
			pgsName:     "twentycharspgsnames12",
			pcsgName:    "",
			pclqName:    "twentyfourcharspclqname1",
			expectError: false,
		},
		{
			description: "Edge case - 46 characters (should fail)",
			pgsName:     "twentyonecharspgsnames",
			pcsgName:    "",
			pclqName:    "twentyfivecharspclqname12",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := validatePodNameConstraints(tc.pgsName, tc.pcsgName, tc.pclqName)
			if tc.expectError {
				assert.Error(t, err, "Expected error for component names: pgs=%s, pcsg=%s, pclq=%s", tc.pgsName, tc.pcsgName, tc.pclqName)
			} else {
				assert.NoError(t, err, "Expected no error for component names: pgs=%s, pcsg=%s, pclq=%s", tc.pgsName, tc.pcsgName, tc.pclqName)
			}
		})
	}
}

func TestPodCliqueScalingGroupConfigValidation(t *testing.T) {
	testCases := []struct {
		description     string
		pgsName         string
		scalingGroups   []grovecorev1alpha1.PodCliqueScalingGroupConfig
		cliqueTemplates []string
		expectError     bool
		expectedErrMsg  string
	}{
		{
			description: "Valid scaling group with Replicas and MinAvailable",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					Replicas:     ptr.To(int32(4)),
					MinAvailable: ptr.To(int32(2)),
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     false,
		},
		{
			description: "Invalid Replicas (negative value)",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					Replicas:     ptr.To(int32(-1)),
					MinAvailable: ptr.To(int32(1)),
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     true,
			expectedErrMsg:  "must be greater than 0",
		},
		{
			description: "Invalid Replicas (zero value)",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					Replicas:     ptr.To(int32(0)),
					MinAvailable: ptr.To(int32(1)),
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     true,
			expectedErrMsg:  "must be greater than 0",
		},
		{
			description: "Invalid MinAvailable (negative value)",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					Replicas:     ptr.To(int32(4)),
					MinAvailable: ptr.To(int32(-1)),
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     true,
			expectedErrMsg:  "must be greater than 0",
		},
		{
			description: "Invalid MinAvailable (zero value)",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					Replicas:     ptr.To(int32(4)),
					MinAvailable: ptr.To(int32(0)),
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     true,
			expectedErrMsg:  "must be greater than 0",
		},
		{
			description: "Invalid MinAvailable > Replicas",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					Replicas:     ptr.To(int32(2)),
					MinAvailable: ptr.To(int32(4)),
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     true,
			expectedErrMsg:  "minAvailable must not be greater than replicas",
		},
		{
			description: "Valid MinAvailable = Replicas",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					Replicas:     ptr.To(int32(4)),
					MinAvailable: ptr.To(int32(4)),
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     false,
		},
		{
			description: "Invalid ScaleConfig.MinReplicas < MinAvailable",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					Replicas:     ptr.To(int32(4)),
					MinAvailable: ptr.To(int32(3)),
					ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(2)),
						MaxReplicas: 10,
					},
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     true,
			expectedErrMsg:  "scaleConfig.minReplicas must be greater than or equal to minAvailable",
		},
		{
			description: "Valid ScaleConfig.MinReplicas >= MinAvailable",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					Replicas:     ptr.To(int32(4)),
					MinAvailable: ptr.To(int32(2)),
					ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(3)),
						MaxReplicas: 10,
					},
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     false,
		},
		{
			description: "Valid when only Replicas is specified",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:        "workers",
					CliqueNames: []string{"prefill"},
					Replicas:    ptr.To(int32(4)),
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     false,
		},
		{
			description: "Valid when only MinAvailable is specified",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:         "workers",
					CliqueNames:  []string{"prefill"},
					MinAvailable: ptr.To(int32(2)),
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     false,
		},
		{
			description: "Valid when neither Replicas nor MinAvailable is specified",
			pgsName:     "inference",
			scalingGroups: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
				{
					Name:        "workers",
					CliqueNames: []string{"prefill"},
				},
			},
			cliqueTemplates: []string{"prefill"},
			expectError:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			pgs := createDummyPodGangSet(tc.pgsName)

			// Add PodClique templates
			for _, cliqueName := range tc.cliqueTemplates {
				pgs.Spec.Template.Cliques = append(pgs.Spec.Template.Cliques, createDummyPodCliqueTemplate(cliqueName))
			}

			// Add scaling groups
			pgs.Spec.Template.PodCliqueScalingGroupConfigs = tc.scalingGroups

			validator := newPGSValidator(pgs, admissionv1.Create)
			warnings, err := validator.validate()

			if tc.expectError {
				assert.Error(t, err, "Expected validation error for test case: %s", tc.description)
				assert.Contains(t, err.Error(), tc.expectedErrMsg, "Error message should contain expected text")
			} else {
				assert.NoError(t, err, "Expected no validation error for test case: %s", tc.description)
			}

			// Warnings should be empty for these test cases
			assert.Empty(t, warnings, "No warnings expected for these test cases")
		})
	}
}
