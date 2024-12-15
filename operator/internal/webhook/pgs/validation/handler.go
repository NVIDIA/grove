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
	v1 "k8s.io/api/admissionregistration/v1"
	"net/http"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"
)

//const handlerName = "podgangset-validation-webhook"

// Handler is a handler for validating PodGangSet resources.
type Handler struct {
	client  client.Client
	decoder admission.Decoder
	logger  logr.Logger
}

// NewHandler creates a new handler for PodGangSet Webhook.
func NewHandler(mgr manager.Manager) *Handler {
	return &Handler{
		client:  mgr.GetClient(),
		decoder: admission.NewDecoder(mgr.GetScheme()),
		logger:  mgr.GetLogger(),
	}
}

// Handle validates operations done on PodGangSet and PodGang resources.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := h.logger.WithValues("name", req.Name, "namespace", req.Namespace, "resource", fmt.Sprintf("%s/%s", req.Kind.Group, req.Kind.Kind), "operation", req.Operation, "user", req.UserInfo.Username)
	log.V(1).Info("PodGangSet validation webhook invoked")

	// TODO: check that req.UserInfo.Username can do validation
	return h.validate(req)
}

func (h *Handler) validate(req admission.Request) admission.Response {
	switch req.Operation {
	case admissionv1.Connect:
		// No validation for connect requests.
		return admission.Allowed(fmt.Sprintf("operation %q on PodGangSet is allowed", v1.Connect))

	case admissionv1.Create:
		pgs := &v1alpha1.PodGangSet{}
		if err := h.decoder.Decode(req, pgs); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return h.validateCreate(pgs)

	case admissionv1.Update:
		newPgs := &v1alpha1.PodGangSet{}
		if err := h.decoder.DecodeRaw(req.Object, newPgs); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		oldPgs := &v1alpha1.PodGangSet{}
		if err := h.decoder.DecodeRaw(req.OldObject, oldPgs); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return h.validateUpdate(newPgs, oldPgs)

	case admissionv1.Delete:
		// In reference to PR: https://github.com/kubernetes/kubernetes/pull/76346
		// OldObject contains the object being deleted
		pgs := &v1alpha1.PodGangSet{}
		if err := h.decoder.DecodeRaw(req.OldObject, pgs); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return h.validateDelete()

	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unknown operation %q", req.Operation))
	}
}

func (h *Handler) validateCreate(pgs *v1alpha1.PodGangSet) admission.Response {
	warnings, err := newValidator(admissionv1.Create, pgs).validate()
	if err != nil {
		return admission.Denied(err.Error()).WithWarnings(warnings...)
	}
	return admission.Allowed("PodGangSet is valid").WithWarnings(warnings...)
}

func (h *Handler) validateUpdate(newPgs, oldPgs *v1alpha1.PodGangSet) admission.Response {
	//validate new PodGangSet
	warnings, err := newValidator(admissionv1.Update, newPgs).validate()
	if err != nil {
		return admission.Denied(err.Error()).WithWarnings(warnings...)
	}
	// validate updates to the PodGangSet
	// TODO: implement
	return admission.Denied("not implemented")
}

func (h *Handler) validateDelete() admission.Response {
	return admission.Allowed("PodGangSet deletion is allowed")
}
