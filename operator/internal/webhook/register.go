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
	defaulting2 "github.com/NVIDIA/grove/operator/internal/webhook/admission/pgs/defaulting"
	validation2 "github.com/NVIDIA/grove/operator/internal/webhook/admission/pgs/validation"
	"log/slog"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func RegisterWebhooks(mgr manager.Manager) error {
	validatingWebhook := validation2.NewHandler(mgr)
	slog.Info("Registering webhook with manager", "handler", validation2.HandlerName)
	if err := validatingWebhook.RegisterWithManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %v", validation2.HandlerName, err)
	}
	defaultingWebhook := defaulting2.NewHandler(mgr)
	slog.Info("Registering webhook with manager", "handler", defaulting2.HandlerName)
	if err := defaultingWebhook.RegisterWithManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %v", defaulting2.HandlerName, err)
	}
	return nil
}
