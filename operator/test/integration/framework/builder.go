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
	"context"
	"fmt"
	"testing"

	groveclient "github.com/NVIDIA/grove/operator/internal/client"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// EnvBuilder builds a test environment with a fluent API
type EnvBuilder struct {
	t *testing.T

	// Core components
	env    *envtest.Environment
	ctx    context.Context
	cancel context.CancelFunc

	// Configuration
	crds           []*apiextensionsv1.CustomResourceDefinition
	scheme         *runtime.Scheme
	webhookBuilder *WebhookConfigurationBuilder
	// Controllers
	controllers    map[ControllerType]bool
	webhooks       map[WebhookType]bool
	webhookOptions envtest.WebhookInstallOptions
	// Namespaces
	namespaces map[string]*corev1.Namespace

	// Pre-created objects
	objects []client.Object
}

// NewEnvBuilder creates a new environment builder with sensible defaults
func NewEnvBuilder(t *testing.T) *EnvBuilder {
	return &EnvBuilder{
		t:              t,
		scheme:         groveclient.Scheme, // Use Grove's production scheme
		controllers:    make(map[ControllerType]bool),
		webhooks:       make(map[WebhookType]bool),
		namespaces:     make(map[string]*corev1.Namespace),
		webhookBuilder: NewWebhookConfigurationBuilder(),
	}
}

// WithCRDs adds custom CRDs to the test environment
func (b *EnvBuilder) WithCRDs(crds ...*apiextensionsv1.CustomResourceDefinition) *EnvBuilder {
	b.crds = append(b.crds, crds...)
	return b
}

// WithController enables a specific controllers
func (b *EnvBuilder) WithController(controllerType ControllerType) *EnvBuilder {
	b.controllers[controllerType] = true
	return b
}

// WithValidationWebhook enables validation webhooks
func (b *EnvBuilder) WithValidationWebhook() *EnvBuilder {
	return b.withWebhook(WebhookValidation)
}

// WithMutationWebhook enables mutation webhooks
func (b *EnvBuilder) WithMutationWebhook() *EnvBuilder {
	return b.withWebhook(WebhookMutation)
}

// withWebhook is a private helper that handles webhook configuration
func (b *EnvBuilder) withWebhook(webhookType WebhookType) *EnvBuilder {
	switch webhookType {
	case WebhookValidation:
		validatingConfig := b.webhookBuilder.BuildValidatingConfig()
		b.webhookOptions.ValidatingWebhooks = append(b.webhookOptions.ValidatingWebhooks, validatingConfig)
	case WebhookMutation:
		mutatingConfig := b.webhookBuilder.BuildMutatingConfig()
		b.webhookOptions.MutatingWebhooks = append(b.webhookOptions.MutatingWebhooks, mutatingConfig)
	}

	b.webhooks[webhookType] = true
	return b
}

// WithNamespace creates a namespace
func (b *EnvBuilder) WithNamespace(name string) *EnvBuilder {
	return b.createNamespace(name, nil)
}

// WithLabeledNamespace creates a namespace with labels
func (b *EnvBuilder) WithLabeledNamespace(name string, labels map[string]string) *EnvBuilder {
	return b.createNamespace(name, labels)
}

// createNamespace is a private helper that creates a namespace with optional labels
func (b *EnvBuilder) createNamespace(name string, labels map[string]string) *EnvBuilder {
	b.namespaces[name] = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	return b
}

// WithObjects adds multiple objects to be created
func (b *EnvBuilder) WithObjects(objs ...client.Object) *EnvBuilder {
	b.objects = append(b.objects, objs...)
	return b
}

// Build creates the test environment but does not start it
func (b *EnvBuilder) Build() (*TestEnv, error) {
	b.ctx, b.cancel = context.WithCancel(context.Background())

	if err := b.prepareCRDs(); err != nil {
		return nil, err
	}

	b.configureEnvironment()

	return b.createTestEnv(), nil
}

// prepareCRDs loads and configures all required CRDs
func (b *EnvBuilder) prepareCRDs() error {
	// Define CRD loaders with their descriptions
	crdLoaders := []struct {
		name   string
		loader func() ([]*apiextensionsv1.CustomResourceDefinition, error)
	}{
		{"operator", getOperatorCRDs},
		{"scheduler", getSchedulerCRDs},
	}

	// Load all CRDs using unified error handling
	for _, crdLoader := range crdLoaders {
		crds, err := crdLoader.loader()
		if err != nil {
			return fmt.Errorf("failed to load %s CRDs: %w", crdLoader.name, err)
		}
		b.crds = append(b.crds, crds...)
	}

	return nil
}

// configureEnvironment sets up the envtest environment
func (b *EnvBuilder) configureEnvironment() {
	b.env = &envtest.Environment{
		CRDs:                  b.crds,
		Scheme:                b.scheme,
		WebhookInstallOptions: b.webhookOptions,
	}
}

// createTestEnv creates the TestEnv instance
func (b *EnvBuilder) createTestEnv() *TestEnv {
	return &TestEnv{
		T:       b.t,
		Client:  nil, // Will be created in Start()
		Manager: nil, // Will be created in Start()
		Ctx:     b.ctx,
		Objects: b.objects,

		// Internal fields for lifecycle management
		env:    b.env,
		cancel: b.cancel,

		namespaceConfigs: b.namespaces,
		controllers:      b.controllers,
		webhooks:         b.webhooks,
	}
}
