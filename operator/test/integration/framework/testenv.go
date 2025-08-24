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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// TestEnv represents a built and started test environment
type TestEnv struct {
	T       *testing.T
	Client  client.Client
	Manager manager.Manager
	Ctx     context.Context
	Objects []client.Object

	// Internal fields for lifecycle management
	env              *envtest.Environment
	cancel           context.CancelFunc
	started          bool
	namespaceConfigs map[string]*corev1.Namespace
	controllers      map[ControllerType]bool
	webhooks         map[WebhookType]bool
}

// Start starts the test environment and all its components
func (te *TestEnv) Start() error {
	if te.started {
		return nil // Already started
	}

	overallStart := time.Now()
	te.T.Logf("=== Starting Test Environment ===")
	te.T.Logf("Environment configuration: controllers=%v, webhooks=%v, namespaces=%d",
		te.getControllerNames(), te.getWebhookNames(), len(te.namespaceConfigs))

	envSetup := NewEnvironmentSetup(te.Ctx, te.env, te.T)

	// Initialize logger
	envSetup.InitializeLogger()

	// Start control plane
	if err := envSetup.StartControlPlane(); err != nil {
		return err
	}

	// Setup client
	kubeClient, err := envSetup.SetupClient()
	if err != nil {
		return err
	}
	te.Client = kubeClient

	// Create namespaces
	if err = envSetup.CreateRequiredNamespaces(te.namespaceConfigs); err != nil {
		return err
	}

	// Setup and start manager
	if err := te.setupAndStartManager(envSetup); err != nil {
		return err
	}

	te.started = true
	overallDuration := time.Since(overallStart)
	te.T.Logf("=== Test Environment Ready (total setup: %v) ===", overallDuration)
	return nil
}

// Shutdown stops the test environment and cleans up resources
func (te *TestEnv) Shutdown() {
	if !te.started {
		return // already stopped or never started
	}

	te.T.Logf("=== Shutting Down Test Environment ===")

	if te.cancel != nil {
		te.cancel()
	}

	if te.env != nil {
		te.T.Logf("Stopping envtest environment")
		_ = te.env.Stop()
	}

	te.started = false
	te.T.Logf("Test environment shutdown complete")
}

// getControllerNames returns a slice of enabled controller names for logging
func (te *TestEnv) getControllerNames() []string {
	var names []string
	for controllerType := range te.controllers {
		names = append(names, string(controllerType))
	}
	return names
}

// getWebhookNames returns a slice of enabled webhook names for logging
func (te *TestEnv) getWebhookNames() []string {
	var names []string
	for webhookType := range te.webhooks {
		names = append(names, string(webhookType))
	}
	return names
}

// setupAndStartManager creates, configures, and starts the manager if needed
func (te *TestEnv) setupAndStartManager(envSetup *EnvironmentSetup) error {
	te.T.Logf("Creating controller manager with %d controllers and %d webhooks",
		len(te.controllers), len(te.webhooks))

	mgr, err := envSetup.CreateManager(te.webhooks, te.env.WebhookInstallOptions)
	if err != nil {
		return err
	}
	te.T.Logf("Controller manager created successfully")
	te.Manager = mgr

	te.T.Logf("Registering components: controllers=%v, webhooks=%v",
		te.getControllerNames(), te.getWebhookNames())
	if err = envSetup.RegisterComponents(te.Manager, te.controllers, te.webhooks); err != nil {
		return err
	}
	te.T.Logf("All components registered successfully")

	te.T.Logf("Starting controller manager and waiting for cache sync")
	if err := envSetup.StartManager(te.Manager); err != nil {
		return err
	}
	te.T.Logf("Manager started successfully, cache synced")
	// Switch to manager's client after startup
	te.Client = te.Manager.GetClient()
	return nil
}
