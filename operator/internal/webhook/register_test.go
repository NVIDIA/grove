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
	"os"
	"testing"

	configv1alpha1 "github.com/ai-dynamo/grove/operator/api/config/v1alpha1"
	"github.com/ai-dynamo/grove/operator/internal/constants"
	testutils "github.com/ai-dynamo/grove/operator/test/utils"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// TestGenerateReconcilerServiceAccountUsername tests the generation of service account usernames
// in the Kubernetes format.
func TestGenerateReconcilerServiceAccountUsername(t *testing.T) {
	tests := []struct {
		// name identifies this test case
		name string
		// namespace is the Kubernetes namespace of the service account
		namespace string
		// serviceAccountName is the name of the service account
		serviceAccountName string
		// expected is the expected formatted username string
		expected string
	}{
		{
			name:               "standard namespace and service account",
			namespace:          "default",
			serviceAccountName: "grove-operator",
			expected:           "system:serviceaccount:default:grove-operator",
		},
		{
			name:               "custom namespace and service account",
			namespace:          "grove-system",
			serviceAccountName: "operator-sa",
			expected:           "system:serviceaccount:grove-system:operator-sa",
		},
		{
			name:               "hyphenated names",
			namespace:          "my-custom-namespace",
			serviceAccountName: "my-service-account",
			expected:           "system:serviceaccount:my-custom-namespace:my-service-account",
		},
		{
			name:               "empty namespace",
			namespace:          "",
			serviceAccountName: "test-sa",
			expected:           "system:serviceaccount::test-sa",
		},
		{
			name:               "empty service account",
			namespace:          "default",
			serviceAccountName: "",
			expected:           "system:serviceaccount:default:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateReconcilerServiceAccountUsername(tt.namespace, tt.serviceAccountName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRegisterWebhooks_WithoutAuthorizer tests webhook registration when authorizer is disabled.
func TestRegisterWebhooks_WithoutAuthorizer(t *testing.T) {
	cl := testutils.NewTestClientBuilder().Build()
	mgr := &testutils.FakeManager{
		Client: cl,
		Scheme: cl.Scheme(),
		Logger: logr.Discard(),
	}

	// Create a real webhook server
	server := webhook.NewServer(webhook.Options{
		Port: 9443,
	})
	mgr.WebhookServer = server

	// Authorizer disabled
	authorizerConfig := configv1alpha1.AuthorizerConfig{
		Enabled: false,
	}

	err := RegisterWebhooks(mgr, authorizerConfig)
	require.NoError(t, err)
}

// TestRegisterWebhooks_WithAuthorizerMissingEnvVar tests that registration fails
// when authorizer is enabled but required environment variable is missing.
func TestRegisterWebhooks_WithAuthorizerMissingEnvVar(t *testing.T) {
	cl := testutils.NewTestClientBuilder().Build()
	mgr := &testutils.FakeManager{
		Client: cl,
		Scheme: cl.Scheme(),
		Logger: logr.Discard(),
	}

	// Create a real webhook server
	server := webhook.NewServer(webhook.Options{
		Port: 9443,
	})
	mgr.WebhookServer = server

	// Ensure env var is not set
	originalEnv := os.Getenv(constants.EnvVarServiceAccountName)
	if err := os.Unsetenv(constants.EnvVarServiceAccountName); err != nil {
		require.NoError(t, err)
	}
	defer func() {
		if originalEnv != "" {
			os.Setenv(constants.EnvVarServiceAccountName, originalEnv)
		}
	}()

	// Authorizer enabled
	authorizerConfig := configv1alpha1.AuthorizerConfig{
		Enabled: true,
	}

	err := RegisterWebhooks(mgr, authorizerConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), constants.EnvVarServiceAccountName)
}

// TestRegisterWebhooks_WithAuthorizerMissingNamespaceFile tests that registration fails
// when authorizer is enabled but namespace file is missing.
func TestRegisterWebhooks_WithAuthorizerMissingNamespaceFile(t *testing.T) {
	cl := testutils.NewTestClientBuilder().Build()
	mgr := &testutils.FakeManager{
		Client: cl,
		Scheme: cl.Scheme(),
		Logger: logr.Discard(),
	}

	// Create a real webhook server
	server := webhook.NewServer(webhook.Options{
		Port: 9443,
	})
	mgr.WebhookServer = server

	// Set env var
	originalEnv := os.Getenv(constants.EnvVarServiceAccountName)
	os.Setenv(constants.EnvVarServiceAccountName, "test-sa")
	defer func() {
		if originalEnv != "" {
			os.Setenv(constants.EnvVarServiceAccountName, originalEnv)
		} else {
			_ = os.Unsetenv(constants.EnvVarServiceAccountName)
		}
	}()

	// Authorizer enabled - will fail on reading non-existent namespace file
	authorizerConfig := configv1alpha1.AuthorizerConfig{
		Enabled: true,
	}

	err := RegisterWebhooks(mgr, authorizerConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error reading namespace file")
}
