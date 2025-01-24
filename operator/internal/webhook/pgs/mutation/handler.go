// /*
// Copyright 2024 The Grove Authors.
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

package mutation

import (
	"context"
	"fmt"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
)

// Handler struct sets default values on PodGangSet CR
type Handler struct {
	logger logr.Logger
}

// Default implements webhook.CustomDefaulter
func (d *Handler) Default(ctx context.Context, obj runtime.Object) error {
	d.logger.V(1).Info("Defaulting for PodGangSet")

	pgs, ok := obj.(*v1alpha1.PodGangSet)
	if !ok {
		return fmt.Errorf("expected an PodGangSet object but got %T", obj)
	}
	defaultPodGangSet(pgs)
	return nil
}
