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

package webhook

import (
	"fmt"
	"log/slog"

	"github.com/NVIDIA/grove/operator/internal/webhook/admission/pgs/defaulting"
	"github.com/NVIDIA/grove/operator/internal/webhook/admission/pgs/validation"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// RegisterWebhooks registers the webhooks with the controller manager.
func RegisterWebhooks(mgr manager.Manager) error {
	defaultingWebhook := defaulting.NewHandler(mgr)
	slog.Info("Registering webhook with manager", "handler", defaulting.HandlerName)
	if err := defaultingWebhook.RegisterWithManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %v", defaulting.HandlerName, err)
	}
	validatingWebhook := validation.NewHandler(mgr)
	slog.Info("Registering webhook with manager", "handler", validation.HandlerName)
	if err := validatingWebhook.RegisterWithManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %v", validation.HandlerName, err)
	}
	return nil
}
