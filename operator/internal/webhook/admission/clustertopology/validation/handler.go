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

package validation

import (
	"context"
	"fmt"

	"github.com/ai-dynamo/grove/operator/api/core/v1alpha1"
	"github.com/ai-dynamo/grove/operator/internal/errors"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// ErrValidateCreateClusterTopology is the error code returned where the request to create a ClusterTopology is invalid.
	ErrValidateCreateClusterTopology v1alpha1.ErrorCode = "ERR_VALIDATE_CREATE_CLUSTERTOPOLOGY"
	// ErrValidateUpdateClusterTopology is the error code returned where the request to update a ClusterTopology is invalid.
	ErrValidateUpdateClusterTopology v1alpha1.ErrorCode = "ERR_VALIDATE_UPDATE_CLUSTERTOPOLOGY"
)

// Handler is a handler for validating ClusterTopology resources.
type Handler struct {
	logger logr.Logger
}

// NewHandler creates a new handler for ClusterTopology Webhook.
func NewHandler(mgr manager.Manager) *Handler {
	return &Handler{
		logger: mgr.GetLogger().WithName("webhook").WithName(Name),
	}
}

// ValidateCreate validates a ClusterTopology create request.
func (h *Handler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	h.logValidatorFunctionInvocation(ctx)
	clusterTopology, err := castToClusterTopology(obj)
	if err != nil {
		return nil, errors.WrapError(err, ErrValidateCreateClusterTopology, string(admissionv1.Create), "failed to cast object to ClusterTopology")
	}
	return nil, newClusterTopologyValidator(clusterTopology, admissionv1.Create).validate()
}

// ValidateUpdate validates a ClusterTopology update request.
func (h *Handler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	h.logValidatorFunctionInvocation(ctx)
	newClusterTopology, err := castToClusterTopology(newObj)
	if err != nil {
		return nil, errors.WrapError(err, ErrValidateUpdateClusterTopology, string(admissionv1.Update), "failed to cast new object to ClusterTopology")
	}
	oldClusterTopology, err := castToClusterTopology(oldObj)
	if err != nil {
		return nil, errors.WrapError(err, ErrValidateUpdateClusterTopology, string(admissionv1.Update), "failed to cast old object to ClusterTopology")
	}
	validator := newClusterTopologyValidator(newClusterTopology, admissionv1.Update)
	if err := validator.validate(); err != nil {
		return nil, err
	}
	return nil, validator.validateUpdate(oldClusterTopology)
}

// ValidateDelete validates a ClusterTopology delete request.
func (h *Handler) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// castToClusterTopology attempts to cast a runtime.Object to a ClusterTopology.
func castToClusterTopology(obj runtime.Object) (*v1alpha1.ClusterTopology, error) {
	clusterTopology, ok := obj.(*v1alpha1.ClusterTopology)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterTopology object but got %T", obj)
	}
	return clusterTopology, nil
}

// logValidatorFunctionInvocation logs details about the validation request including user and operation information.
func (h *Handler) logValidatorFunctionInvocation(ctx context.Context) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		h.logger.Error(err, "failed to get request from context")
		return
	}
	h.logger.Info("ClusterTopology validation webhook invoked", "name", req.Name, "namespace", req.Namespace, "operation", req.Operation, "user", req.UserInfo.Username)
}
