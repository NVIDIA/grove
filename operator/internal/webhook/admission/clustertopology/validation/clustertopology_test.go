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
	"testing"

	grovecorev1alpha1 "github.com/ai-dynamo/grove/operator/api/core/v1alpha1"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateCreate(t *testing.T) {
	tests := []struct {
		name           string
		ct             *grovecorev1alpha1.ClusterTopology
		expectedErr    bool
		expectedErrMsg string
	}{
		{
			name: "valid cluster topology with single level",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "topology.kubernetes.io/zone",
						},
					},
				},
			},
			expectedErr: false,
		},
		{
			name: "valid cluster topology with multiple levels",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
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
			expectedErr: false,
		},
		{
			name: "invalid - no levels defined",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "at least one topology level must be defined",
		},
		{
			name: "invalid - duplicate domain (caught by order validation)",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "topology.kubernetes.io/zone",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "custom.io/zone",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "topology levels must be in hierarchical order",
		},
		{
			name: "invalid - empty key",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "key is required",
		},
		{
			name: "invalid - key not a valid label (has space)",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "Invalid value",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "name part must consist of alphanumeric characters",
		},
		{
			name: "invalid - key prefix has invalid characters (double dots)",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "invalid..label/key",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "prefix part",
		},
		{
			name: "invalid - key name part too long",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "example.com/this-is-a-very-long-name-that-exceeds-the-maximum-length-of-sixtythree-characters",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "name part must be no more than 63 characters",
		},
		{
			name: "invalid - levels out of order (zone before region)",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "topology.kubernetes.io/zone",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							Key:    "topology.kubernetes.io/region",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "topology levels must be in hierarchical order",
		},
		{
			name: "invalid - levels out of order (host before rack)",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainHost,
							Key:    "kubernetes.io/hostname",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainRack,
							Key:    "topology.kubernetes.io/rack",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "topology levels must be in hierarchical order",
		},
		{
			name: "invalid - levels out of order (rack before datacenter)",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							Key:    "topology.kubernetes.io/region",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainRack,
							Key:    "topology.kubernetes.io/rack",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainDataCenter,
							Key:    "topology.kubernetes.io/datacenter",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "topology levels must be in hierarchical order",
		},
		{
			name: "valid - correct hierarchical order (region > zone > datacenter > rack > host > numa)",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
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
							Domain: grovecorev1alpha1.TopologyDomainDataCenter,
							Key:    "topology.kubernetes.io/datacenter",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainRack,
							Key:    "topology.kubernetes.io/rack",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainHost,
							Key:    "kubernetes.io/hostname",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainNuma,
							Key:    "topology.kubernetes.io/numa",
						},
					},
				},
			},
			expectedErr: false,
		},
		{
			name: "valid - correct hierarchical order with gaps (region > rack > numa)",
			ct: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							Key:    "topology.kubernetes.io/region",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainRack,
							Key:    "topology.kubernetes.io/rack",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainNuma,
							Key:    "topology.kubernetes.io/numa",
						},
					},
				},
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := newCTValidator(tt.ct, admissionv1.Create)
			_, err := validator.validate()

			if tt.expectedErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateUpdate(t *testing.T) {
	tests := []struct {
		name           string
		oldCT          *grovecorev1alpha1.ClusterTopology
		newCT          *grovecorev1alpha1.ClusterTopology
		expectedErr    bool
		expectedErrMsg string
	}{
		{
			name: "valid update - key changed",
			oldCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "topology.kubernetes.io/zone",
						},
					},
				},
			},
			newCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "custom.io/zone",
						},
					},
				},
			},
			expectedErr: false,
		},
		{
			name: "invalid - domain changed",
			oldCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "topology.kubernetes.io/zone",
						},
					},
				},
			},
			newCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							Key:    "topology.kubernetes.io/region",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "domain is immutable",
		},
		{
			name: "invalid - level added",
			oldCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "topology.kubernetes.io/zone",
						},
					},
				},
			},
			newCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
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
			expectedErr:    true,
			expectedErrMsg: "not allowed to add or remove topology levels",
		},
		{
			name: "invalid - level removed",
			oldCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
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
					},
				},
			},
			newCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							Key:    "topology.kubernetes.io/region",
						},
					},
				},
			},
			expectedErr:    true,
			expectedErrMsg: "not allowed to add or remove topology levels",
		},
		{
			name: "valid update - multiple levels, keys changed",
			oldCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
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
			newCT: &grovecorev1alpha1.ClusterTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-topology",
				},
				Spec: grovecorev1alpha1.ClusterTopologySpec{
					Levels: []grovecorev1alpha1.TopologyLevel{
						{
							Domain: grovecorev1alpha1.TopologyDomainRegion,
							Key:    "custom.io/region",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainZone,
							Key:    "custom.io/zone",
						},
						{
							Domain: grovecorev1alpha1.TopologyDomainHost,
							Key:    "custom.io/host",
						},
					},
				},
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := newCTValidator(tt.newCT, admissionv1.Update)
			_, err := validator.validate()
			if err != nil {
				t.Fatalf("validate() failed: %v", err)
			}

			err = validator.validateUpdate(tt.oldCT)
			if tt.expectedErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
