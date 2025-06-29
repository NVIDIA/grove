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
	"context"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetExistingPodGangNames gets the names of all existing PodGangs for the PodGangSet.
func GetExistingPodGangNames(ctx context.Context, cl client.Client, pgs *grovecorev1alpha1.PodGangSet) ([]string, error) {
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(groveschedulerv1alpha1.SchemeGroupVersion.WithKind("PodGang"))
	if err := cl.List(ctx,
		objMetaList,
		client.InNamespace(pgs.Namespace),
		client.MatchingLabels(GetPodGangSelectorLabels(pgs.ObjectMeta)),
	); err != nil {
		return nil, err
	}
	return k8sutils.FilterMapOwnedResourceNames(pgs.ObjectMeta, objMetaList.Items), nil
}

// GetPodGangSelectorLabels creates the label selector to list all the PodGangs for a PodGangSet.
func GetPodGangSelectorLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjMeta.Name),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey: component.NamePodGang,
		})
}
