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

package validation

import (
	"context"
	"fmt"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/errors"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// ErrValidateCreatePodGangSet is the error code returned where the request to create a PodGangSet is invalid.
	ErrValidateCreatePodGangSet v1alpha1.ErrorCode = "ERR_VALIDATE_CREATE_PODGANGSET"
	// ErrValidateUpdatePodGangSet is the error code returned where the request to update a PodGangSet is invalid.
	ErrValidateUpdatePodGangSet v1alpha1.ErrorCode = "ERR_VALIDATE_UPDATE_PODGANGSET"
)

// Handler is a handler for validating PodGangSet resources.
type Handler struct {
	logger logr.Logger
}

// NewHandler creates a new handler for PodGangSet Webhook.
func NewHandler(mgr manager.Manager) *Handler {
	return &Handler{
		logger: mgr.GetLogger().WithName("webhook").WithName(HandlerName),
	}
}

// ValidateCreate validates a PodGangSet create request.
func (h *Handler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	h.logValidatorFunctionInvocation(ctx)
	pgs, err := castToPodGangSet(obj)
	if err != nil {
		return nil, errors.WrapError(err, ErrValidateCreatePodGangSet, string(admissionv1.Create), "failed to cast object to PodGangSet")
	}
	return newPGSValidator(pgs, admissionv1.Create).validate()
}

// ValidateUpdate validates a PodGangSet update request.
func (h *Handler) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) (admission.Warnings, error) {
	h.logValidatorFunctionInvocation(ctx)
	newPgs, err := castToPodGangSet(newObj)
	if err != nil {
		return nil, errors.WrapError(err, ErrValidateUpdatePodGangSet, string(admissionv1.Update), "failed to cast new object to PodGangSet")
	}
	oldPgs, err := castToPodGangSet(oldObj)
	if err != nil {
		return nil, errors.WrapError(err, ErrValidateUpdatePodGangSet, string(admissionv1.Update), "failed to cast old object to PodGangSet")
	}
	validator := newPGSValidator(newPgs, admissionv1.Update)
	warnings, err := validator.validate()
	if err != nil {
		return warnings, err
	}
	return warnings, validator.validateUpdate(oldPgs)
}

// ValidateDelete validates a PodGangSet delete request.
func (h *Handler) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func castToPodGangSet(obj runtime.Object) (*v1alpha1.PodGangSet, error) {
	pgs, ok := obj.(*v1alpha1.PodGangSet)
	if !ok {
		return nil, fmt.Errorf("expected an PodGangSet object but got %T", obj)
	}
	return pgs, nil
}

func (h *Handler) logValidatorFunctionInvocation(ctx context.Context) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		h.logger.Error(err, "failed to get request from context")
		return
	}
	h.logger.Info("PodGangSet validation webhook invoked", "name", req.Name, "namespace", req.Namespace, "operation", req.Operation, "user", req.UserInfo.Username)
}
