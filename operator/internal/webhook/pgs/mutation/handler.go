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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
)

// MutatingHandler is a handler for mutating PodGangSet resources.
type MutatingHandler struct {
	client  client.Client
	decoder admission.Decoder
	logger  logr.Logger
}

// NewHandler creates a new handler for PodGangSet Webhook.
func NewHandler(mgr manager.Manager) (*MutatingHandler, error) {
	return &MutatingHandler{
		client:  mgr.GetClient(),
		decoder: admission.NewDecoder(mgr.GetScheme()),
		logger:  mgr.GetLogger(),
	}, nil
}

// Handle adds defaults to PodGangSet resource.
func (h *MutatingHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := h.logger.WithValues("name", req.Name, "namespace", req.Namespace, "resource", fmt.Sprintf("%s/%s", req.Kind.Group, req.Kind.Kind), "operation", req.Operation, "user", req.UserInfo.Username)
	log.V(1).Info("Grove mutation webhook invoked")

	return h.mutate(req)
}

func (h *MutatingHandler) mutate(req admission.Request) admission.Response {
	switch req.Operation {
	case v1.Create, v1.Update:
		pgs := &v1alpha1.PodGangSet{}
		if err := h.decoder.Decode(req, pgs); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		h.mutateCreate(pgs)

		mPgs, err := json.Marshal(pgs)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, mPgs)
	}
	return admission.Response{}
}

func (h *MutatingHandler) mutateCreate(pgs *v1alpha1.PodGangSet) {
	mutatePodGangSet(pgs)
}
