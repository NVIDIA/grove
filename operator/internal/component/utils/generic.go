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
	"github.com/NVIDIA/grove/operator/internal/component"

	"github.com/samber/lo"
)

// groupByLabel groups Kubernetes objects by the specified label key using lo utilities.
func groupByLabel[T component.GroveCustomResourceType, P interface {
	GetLabels() map[string]string
	*T
}](items []T, labelKey string) map[string][]T {
	itemsWithLabel := lo.Filter(items, func(item T, _ int) bool {
		var p P = &item
		_, ok := p.GetLabels()[labelKey]
		return ok
	})
	return lo.GroupBy(itemsWithLabel, func(item T) string {
		var p P = &item
		return p.GetLabels()[labelKey]
	})
}
