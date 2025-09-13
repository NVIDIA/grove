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
	"fmt"
	"strconv"
	"strings"

	apicommon "github.com/NVIDIA/grove/operator/api/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPodCliqueSetReplicaIndexFromPodCliqueFQN extracts the PodCliqueSet replica index from a Pod Clique FQN name.
func GetPodCliqueSetReplicaIndexFromPodCliqueFQN(pgsName, pclqFQNName string) (int, error) {
	replicaStartIndex := len(pgsName) + 1 // +1 for the hyphen
	hyphenIndex := strings.Index(pclqFQNName[replicaStartIndex:], "-")
	if hyphenIndex == -1 {
		return -1, fmt.Errorf("PodClique FQN is not in the expected format of <pcs-name>-<pcs-replica-index>-<pclq-template-name>: %s", pclqFQNName)
	}
	replicaEndIndex := replicaStartIndex + hyphenIndex
	return strconv.Atoi(pclqFQNName[replicaStartIndex:replicaEndIndex])
}

// GetPodCliqueNameFromPodCliqueFQN get unqualified PodClique name from FQN.
func GetPodCliqueNameFromPodCliqueFQN(pclqObjectMeta metav1.ObjectMeta) (string, error) {
	pclqObjectKey := client.ObjectKey{Name: pclqObjectMeta.Name, Namespace: pclqObjectMeta.Namespace}

	// Check if the PodClique is part of a PodCliqueScalingGroup
	pcsgName, ok := pclqObjectMeta.Labels[apicommon.LabelPodCliqueScalingGroup]
	if ok {
		// get the pcsg replica index
		pcsgReplicaIndex, replicaIndexLabelFound := pclqObjectMeta.Labels[apicommon.LabelPodCliqueScalingGroupReplicaIndex]
		if !replicaIndexLabelFound {
			return "", fmt.Errorf("missing label %s on PodClique: %v", apicommon.LabelPodCliqueScalingGroupReplicaIndex, pclqObjectKey)
		}
		pclqNamePrefix := fmt.Sprintf("%s-%s-", pcsgName, pcsgReplicaIndex)
		return pclqObjectMeta.Name[len(pclqNamePrefix):], nil
	}

	// If it is not part of PCSG then the PCLQ is a standalone PCLQ which is part of PGS
	pgsName, ok := pclqObjectMeta.Labels[apicommon.LabelPartOfKey]
	if !ok {
		return "", fmt.Errorf("missing label %s on PodClique: %v", apicommon.LabelPartOfKey, pclqObjectKey)
	}
	// Get the PGS replica index
	pgsReplicaIndex, ok := pclqObjectMeta.Labels[apicommon.LabelPodCliqueSetReplicaIndex]
	if !ok {
		return "", fmt.Errorf("missing label %s on PodClique: %v", apicommon.LabelPodCliqueSetReplicaIndex, pclqObjectKey)
	}
	pclqNamePrefix := fmt.Sprintf("%s-%s-", pgsName, pgsReplicaIndex)
	return pclqObjectMeta.Name[len(pclqNamePrefix):], nil
}
