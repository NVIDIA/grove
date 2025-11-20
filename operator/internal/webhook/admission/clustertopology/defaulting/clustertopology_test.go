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

package defaulting

import (
	"testing"

	"github.com/ai-dynamo/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func TestDefaultClusterTopology(t *testing.T) {
	tests := []struct {
		name     string
		input    []v1alpha1.TopologyLevel
		expected []v1alpha1.TopologyLevel
	}{
		{
			name: "single level remains unchanged",
			input: []v1alpha1.TopologyLevel{
				{Domain: v1alpha1.TopologyDomainRack, Key: "topology.kubernetes.io/rack"},
			},
			expected: []v1alpha1.TopologyLevel{
				{Domain: v1alpha1.TopologyDomainRack, Key: "topology.kubernetes.io/rack"},
			},
		},
		{
			name: "reorder from mixed order",
			input: []v1alpha1.TopologyLevel{
				{Domain: v1alpha1.TopologyDomainHost, Key: "kubernetes.io/hostname"},
				{Domain: v1alpha1.TopologyDomainRegion, Key: "topology.kubernetes.io/region"},
				{Domain: v1alpha1.TopologyDomainRack, Key: "topology.kubernetes.io/rack"},
			},
			expected: []v1alpha1.TopologyLevel{
				{Domain: v1alpha1.TopologyDomainRegion, Key: "topology.kubernetes.io/region"},
				{Domain: v1alpha1.TopologyDomainRack, Key: "topology.kubernetes.io/rack"},
				{Domain: v1alpha1.TopologyDomainHost, Key: "kubernetes.io/hostname"},
			},
		},
		{
			name: "already ordered levels remain unchanged",
			input: []v1alpha1.TopologyLevel{
				{Domain: v1alpha1.TopologyDomainRegion, Key: "topology.kubernetes.io/region"},
				{Domain: v1alpha1.TopologyDomainZone, Key: "topology.kubernetes.io/zone"},
				{Domain: v1alpha1.TopologyDomainHost, Key: "kubernetes.io/hostname"},
			},
			expected: []v1alpha1.TopologyLevel{
				{Domain: v1alpha1.TopologyDomainRegion, Key: "topology.kubernetes.io/region"},
				{Domain: v1alpha1.TopologyDomainZone, Key: "topology.kubernetes.io/zone"},
				{Domain: v1alpha1.TopologyDomainHost, Key: "kubernetes.io/hostname"},
			},
		},
		{
			name: "all topology domains reordered correctly",
			input: []v1alpha1.TopologyLevel{
				{Domain: v1alpha1.TopologyDomainNuma, Key: "topology.kubernetes.io/numa"},
				{Domain: v1alpha1.TopologyDomainHost, Key: "kubernetes.io/hostname"},
				{Domain: v1alpha1.TopologyDomainRack, Key: "topology.kubernetes.io/rack"},
				{Domain: v1alpha1.TopologyDomainBlock, Key: "topology.kubernetes.io/block"},
				{Domain: v1alpha1.TopologyDomainDataCenter, Key: "topology.kubernetes.io/datacenter"},
				{Domain: v1alpha1.TopologyDomainZone, Key: "topology.kubernetes.io/zone"},
				{Domain: v1alpha1.TopologyDomainRegion, Key: "topology.kubernetes.io/region"},
			},
			expected: []v1alpha1.TopologyLevel{
				{Domain: v1alpha1.TopologyDomainRegion, Key: "topology.kubernetes.io/region"},
				{Domain: v1alpha1.TopologyDomainZone, Key: "topology.kubernetes.io/zone"},
				{Domain: v1alpha1.TopologyDomainDataCenter, Key: "topology.kubernetes.io/datacenter"},
				{Domain: v1alpha1.TopologyDomainBlock, Key: "topology.kubernetes.io/block"},
				{Domain: v1alpha1.TopologyDomainRack, Key: "topology.kubernetes.io/rack"},
				{Domain: v1alpha1.TopologyDomainHost, Key: "kubernetes.io/hostname"},
				{Domain: v1alpha1.TopologyDomainNuma, Key: "topology.kubernetes.io/numa"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterTopology := &v1alpha1.ClusterTopology{
				Spec: v1alpha1.ClusterTopologySpec{
					Levels: tt.input,
				},
			}

			defaultClusterTopology(clusterTopology)

			assert.Equal(t, tt.expected, clusterTopology.Spec.Levels, "levels should be reordered correctly")
		})
	}
}
