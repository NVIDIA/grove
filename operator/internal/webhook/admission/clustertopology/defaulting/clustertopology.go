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
	"sort"

	"github.com/ai-dynamo/grove/operator/api/core/v1alpha1"
)

// defaultClusterTopology reorders the topology levels to match the predefined hierarchy.
// The levels are sorted from broadest to narrowest scope
func defaultClusterTopology(clusterTopology *v1alpha1.ClusterTopology) {
	if len(clusterTopology.Spec.Levels) == 1 {
		// No need to reorder if there's a single level
		return
	}

	// Sort levels by their domain order using the Compare method
	sort.SliceStable(clusterTopology.Spec.Levels, func(i, j int) bool {
		return clusterTopology.Spec.Levels[i].Compare(clusterTopology.Spec.Levels[j]) < 0
	})
}
