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

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// WebhookConfigurationBuilder builds webhook configurations
type WebhookConfigurationBuilder struct{}

// NewWebhookConfigurationBuilder creates a new webhook configuration builder
func NewWebhookConfigurationBuilder() *WebhookConfigurationBuilder {
	return &WebhookConfigurationBuilder{}
}

// BuildMutatingConfig builds a mutating webhook configuration
func (w *WebhookConfigurationBuilder) BuildMutatingConfig() *admissionregistrationv1.MutatingWebhookConfiguration {
	webhookConfig := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: MutatingWebhookConfigName,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name: PGSMutatingWebhookName,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name: TestWebhookServiceName,
					Path: ptr.To(MutationWebhookPath),
				},
			},
			Rules:                   defaultWebhookRules(),
			FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
			MatchPolicy:             ptr.To(admissionregistrationv1.Exact),
			SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNone),
			TimeoutSeconds:          ptr.To[int32](DefaultWebhookTimeout),
			AdmissionReviewVersions: []string{"v1"},
		}},
	}
	return webhookConfig
}

// BuildValidatingConfig builds a validating webhook configuration
func (w *WebhookConfigurationBuilder) BuildValidatingConfig() *admissionregistrationv1.ValidatingWebhookConfiguration {
	webhookConfig := admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: ValidatingWebhookConfigName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name: PGSValidatingWebhookName,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name: TestWebhookServiceName,
					Path: ptr.To(ValidationWebhookPath),
				},
			},
			Rules:                   defaultWebhookRules(),
			FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
			SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNone),
			TimeoutSeconds:          ptr.To[int32](DefaultWebhookTimeout),
			AdmissionReviewVersions: []string{"v1"},
		}},
	}
	return &webhookConfig
}

func defaultWebhookRules() []admissionregistrationv1.RuleWithOperations {
	return []admissionregistrationv1.RuleWithOperations{
		{
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"grove.io"},
				APIVersions: []string{"v1alpha1"},
				Resources:   []string{"podgangsets"},
			},
		},
	}
}
