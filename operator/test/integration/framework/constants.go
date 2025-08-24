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

package framework

const (
	// DefaultWebhookTimeout is the default timeout for webhook requests
	DefaultWebhookTimeout = 10
	// PGSMutatingWebhookName is the name for PodGangSet mutating webhook
	PGSMutatingWebhookName = "pgs.mutating.webhooks.grove.io"
	// PGSValidatingWebhookName is the name for PodGangSet validating webhook
	PGSValidatingWebhookName = "pgs.validating.webhooks.grove.io"

	// TestWebhookServiceName is the service name used for webhook testing
	TestWebhookServiceName = "grove-operator-test-webhook"
	// MutationWebhookPath is the path for mutation webhooks
	MutationWebhookPath = "/webhooks/default-podgangset"
	// ValidationWebhookPath is the path for validation webhooks
	ValidationWebhookPath = "/webhooks/validate-podgangset"

	// MutatingWebhookConfigName is the name for mutating webhook configuration
	MutatingWebhookConfigName = "pgs-mutating-webhook-test"
	// ValidatingWebhookConfigName is the name for validating webhook configuration
	ValidatingWebhookConfigName = "pgs-validating-webhook-test"
)
