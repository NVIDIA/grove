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
	"context"
	"fmt"

	"github.com/ai-dynamo/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/ai-dynamo/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Handler sets default values on ClusterTopology resources.
type Handler struct {
	logger logr.Logger
}

// NewHandler returns a new instance of defaulting webhook handler.
func NewHandler(mgr manager.Manager) *Handler {
	return &Handler{
		logger: mgr.GetLogger().WithName("webhook").WithName(Name),
	}
}

// Default applies default values to a ClusterTopology object.
func (h *Handler) Default(ctx context.Context, obj runtime.Object) error {
	h.logger.Info("Defaulting webhook invoked for ClusterTopology")
	clusterTopology, ok := obj.(*v1alpha1.ClusterTopology)
	if !ok {
		return fmt.Errorf("expected a ClusterTopology object but got %T", obj)
	}
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}
	h.logger.Info("Applying defaults", "ClusterTopology", k8sutils.CreateObjectKeyForCreateWebhooks(clusterTopology, req))
	defaultClusterTopology(clusterTopology)
	return nil
}
