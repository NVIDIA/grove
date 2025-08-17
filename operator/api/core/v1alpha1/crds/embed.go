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

package crds

import _ "embed"

var (
	//go:embed grove.io_podcliques.yaml
	podCliqueCRD string
	//go:embed grove.io_podgangsets.yaml
	podGangSetCRD string
	//go:embed grove.io_podcliquescalinggroups.yaml
	podCliqueScalingGroupCRD string
)

// PodCliqueCRD returns the PodClique CRD
func PodCliqueCRD() string {
	return podCliqueCRD
}

// PodGangSetCRD returns the PodGangSet CRD
func PodGangSetCRD() string {
	return podGangSetCRD
}

// PodCliqueScalingGroupCRD returns the PodCliqueScalingGroup CRD
func PodCliqueScalingGroupCRD() string {
	return podCliqueScalingGroupCRD
}
