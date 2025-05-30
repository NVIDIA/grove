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

package authorization

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Handler is the PodGangSet authorization admission webhook handler.
type Handler struct {
	Logger  logr.Logger
	Decoder admission.Decoder
}

// Handle handles requests and admits them if they are authorized.
func (h *Handler) Handle(_ context.Context, _ admission.Request) admission.Response {
	return admission.Response{}
}
