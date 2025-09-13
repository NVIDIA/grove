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

package kubernetes

import (
	"errors"
	"fmt"
	"strconv"

	apicommon "github.com/NVIDIA/grove/operator/api/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errReplicaIndexIntConversion           = errors.New("failed to convert replica index to int")
	errNotFoundPodGangSetReplicaIndexLabel = fmt.Errorf("label %s not found on resource", apicommon.LabelPodCliqueSetReplicaIndex)
)

// GetPodCliqueSetReplicaIndex extracts the PodCliqueSet replica index from the labels on the managed resource.
func GetPodCliqueSetReplicaIndex(objMeta metav1.ObjectMeta) (int, error) {
	pgsReplicaIndexStr, ok := objMeta.GetLabels()[apicommon.LabelPodCliqueSetReplicaIndex]
	if !ok {
		return 0, errNotFoundPodGangSetReplicaIndexLabel
	}
	pgsReplicaIndex, err := strconv.Atoi(pgsReplicaIndexStr)
	if err != nil {
		return 0, fmt.Errorf("%w: %w invalid PodCliqueSet replica index label value set on resource %v", errReplicaIndexIntConversion, err, GetObjectKeyFromObjectMeta(objMeta))
	}
	return pgsReplicaIndex, nil
}
