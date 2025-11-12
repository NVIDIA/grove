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
	"context"
	"os"
	"testing"
	"time"

	grovecorev1alpha1 "github.com/ai-dynamo/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestClusterTopologyCRDValidation(t *testing.T) {
	testEnv, k8sClient := setupEnvTest(t)
	defer teardownEnvTest(t, testEnv)

	tests := []struct {
		name          string
		ct            *grovecorev1alpha1.ClusterTopology
		expectError   bool
		errorContains string
	}{
		{
			name: "valid cluster topology with single level",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "test-valid-single"},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							Key:    "topology.kubernetes.io/region",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid cluster topology with multiple levels",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "test-valid-multiple"},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							Key:    "topology.kubernetes.io/region",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "topology.kubernetes.io/zone",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainHost,
							Key:    "kubernetes.io/hostname",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "reject invalid domain",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "test-invalid-domain"},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: "invalid-domain", // Not in enum
							Key:    "test.io/key",
						},
					},
				},
			},
			expectError:   true,
			errorContains: "supported values",
		},
		{
			name: "reject empty key",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "test-empty-key"},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							Key:    "", // Empty key
						},
					},
				},
			},
			expectError:   true,
			errorContains: "spec.levels[0].key",
		},
		{
			name: "reject key too long",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "test-key-too-long"},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							// Create a key longer than 63 characters (MaxLength validation)
							Key: "this-is-a-very-long-key-that-exceeds-the-maximum-allowed-length-of-63-characters",
						},
					},
				},
			},
			expectError:   true,
			errorContains: "spec.levels[0].key",
		},
		{
			name: "reject empty levels array",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "test-empty-levels"},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{}, // Empty array
				},
			},
			expectError:   true,
			errorContains: "spec.levels",
		},
		{
			name: "reject too many levels",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "test-too-many-levels"},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{Domain: grovecorev1alpha1.TopologyDomainRegion, Key: "key1"},
						{Domain: grovecorev1alpha1.TopologyDomainZone, Key: "key2"},
						{Domain: grovecorev1alpha1.TopologyDomainDataCenter, Key: "key3"},
						{Domain: grovecorev1alpha1.TopologyDomainBlock, Key: "key4"},
						{Domain: grovecorev1alpha1.TopologyDomainRack, Key: "key5"},
						{Domain: grovecorev1alpha1.TopologyDomainHost, Key: "key6"},
						{Domain: grovecorev1alpha1.TopologyDomainNuma, Key: "key7"},
						{Domain: "region", Key: "key8"}, // 8th level - exceeds max
					},
				},
			},
			expectError:   true,
			errorContains: "spec.levels",
		},
		{
			name: "reject missing domain",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "test-missing-domain"},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Key: "test.io/key", // Missing domain
						},
					},
				},
			},
			expectError: true,
			// API server should reject due to Required validation
		},
		{
			name: "reject missing key",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{Name: "test-missing-key"},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							// Missing key
						},
					},
				},
			},
			expectError: true,
			// API server should reject due to Required validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := k8sClient.Create(context.Background(), tt.ct)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				// Clean up successfully created resource
				_ = k8sClient.Delete(context.Background(), tt.ct)
			}
		})
	}
}

// setupEnvTest creates an envtest environment with ClusterTopology CRDs loaded
func setupEnvTest(t *testing.T) (*envtest.Environment, client.Client) {
	// Check if KUBEBUILDER_ASSETS is set
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("Skipping envtest: KUBEBUILDER_ASSETS not set. Run 'make test-envtest' to execute this test.")
	}

	// Point to CRD directory (relative from this test file location)
	crdPath := "../../../../../api/core/v1alpha1/crds"

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{crdPath},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)

	// Create client with proper scheme
	testScheme := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(testScheme))
	utilruntime.Must(grovecorev1alpha1.AddToScheme(testScheme))

	k8sClient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	// Give CRDs a moment to be fully ready
	time.Sleep(100 * time.Millisecond)

	return testEnv, k8sClient
}

// teardownEnvTest stops the envtest environment
func teardownEnvTest(t *testing.T, testEnv *envtest.Environment) {
	err := testEnv.Stop()
	require.NoError(t, err)
}
