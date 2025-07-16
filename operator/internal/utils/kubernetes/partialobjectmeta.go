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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ListExistingPartialObjectMetadata gets the PartialObjectMetadata for a GVK in a given namespace and matching labels.
func ListExistingPartialObjectMetadata(ctx context.Context, cl client.Client, gvk schema.GroupVersionKind, ownerObjMeta metav1.ObjectMeta, selectorLabels map[string]string) ([]metav1.PartialObjectMetadata, error) {
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(gvk)
	if err := cl.List(ctx,
		objMetaList,
		client.InNamespace(ownerObjMeta.Namespace),
		client.MatchingLabels(selectorLabels),
	); err != nil {
		return nil, err
	}
	return objMetaList.Items, nil
}
