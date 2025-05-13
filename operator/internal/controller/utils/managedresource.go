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
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasExpectedOwner checks if the owner references of a resource match the expected owner kind.
func HasExpectedOwner(expectedOwnerKind string, ownerRefs []metav1.OwnerReference) bool {
	if len(ownerRefs) == 0 || len(ownerRefs) > 1 {
		return false
	}
	return ownerRefs[0].Kind == expectedOwnerKind
}

// IsManagedByGrove checks if the pod is managed by Grove by inspecting its labels.
// All grove managed resources will have a standard set of default labels.
func IsManagedByGrove(labels map[string]string) bool {
	if val, ok := labels[v1alpha1.LabelManagedByKey]; ok {
		return v1alpha1.LabelManagedByValue == val
	}
	return false
}
