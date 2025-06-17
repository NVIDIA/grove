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
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetDefaultLabelsForPodGangSetManagedResources gets the default labels for resources managed by PodGangset.
func GetDefaultLabelsForPodGangSetManagedResources(pgsName string) map[string]string {
	return map[string]string{
		v1alpha1.LabelManagedByKey: v1alpha1.LabelManagedByValue,
		v1alpha1.LabelPartOfKey:    pgsName,
	}
}

// FilterMapOwnedResourceNames filters the candidate resources and returns the names of those that are owned by the given owner object meta.
func FilterMapOwnedResourceNames(ownerObjMeta metav1.ObjectMeta, candidateResources []metav1.PartialObjectMetadata) []string {
	return lo.FilterMap(candidateResources, func(objMeta metav1.PartialObjectMetadata, _ int) (string, bool) {
		if metav1.IsControlledBy(&objMeta, &ownerObjMeta) {
			return objMeta.Name, true
		}
		return "", false
	})
}

// GetFirstOwnerName returns the name of the first owner reference of the resource object meta.
func GetFirstOwnerName(resourceObjMeta metav1.ObjectMeta) string {
	if len(resourceObjMeta.OwnerReferences) == 0 {
		return ""
	}
	return resourceObjMeta.OwnerReferences[0].Name
}

// GetObjectKeyFromObjectMeta creates a client.ObjectKey from the given ObjectMeta.
func GetObjectKeyFromObjectMeta(objMeta metav1.ObjectMeta) client.ObjectKey {
	return client.ObjectKey{
		Namespace: objMeta.Namespace,
		Name:      objMeta.Name,
	}
}
