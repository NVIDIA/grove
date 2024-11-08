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

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//const handlerName = "podgangset-validation-webhook"

// Handler is a handler for validating PodGangSet resources.
type Handler struct {
	client client.Client
	logger logr.Logger
}

// Handle validates operations done on PodGangSet and PodGang resources.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	panic("implement me")
}
