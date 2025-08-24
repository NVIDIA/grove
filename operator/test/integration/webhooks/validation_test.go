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

package webhooks_test

import (
	"testing"
	"time"

	"github.com/NVIDIA/grove/operator/test/integration/framework"
	"github.com/NVIDIA/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPodGangSetValidationWebhook(t *testing.T) {
	// Setup test environment with webhooks enabled
	env, err := framework.NewEnvBuilder(t).
		WithMutationWebhook().
		WithValidationWebhook().
		WithNamespace("test-ns").
		Build()
	require.NoError(t, err)
	// Start the environment
	err = env.Start()
	require.NoError(t, err)
	defer env.Shutdown()

	// Read the CA certificate from the webhook server

	t.Logf("Created ValidatingWebhookConfiguration for testing")

	// Test 1: Valid PodGangSet should succeed
	t.Run("ValidPodGangSet", func(t *testing.T) {
		validPGS := utils.NewPodGangSetBuilder("valid-pgs", "test-ns").
			WithMinimal().
			Build()

		// Valid PodGangSet should be accepted
		err := env.Client.Create(env.Ctx, validPGS)
		require.NoError(t, err, "Valid PodGangSet should be accepted by webhook")
		t.Logf("Valid PodGangSet was accepted as expected")
	})

	// Test 2: Invalid PodGangSet should be rejected by validation webhook
	t.Run("InvalidPodGangSet", func(t *testing.T) {
		// Name is too long (>45 characters) - should be rejected by validation webhook
		invalidPGS := utils.NewPodGangSetBuilder(
			"this-podgangset-nddddddddddddddfdfdfdfdfdfdfdfdfdfdfddddddddddddddame-is-way-too-long-and-should-be-rejected-by-validation",
			"test-ns").
			WithMinimal().
			Build()

		time.Sleep(10 * time.Second) // Ensure some delay
		// Invalid PodGangSet should be rejected
		err = env.Client.Create(env.Ctx, invalidPGS)
		require.Error(t, err, "Invalid PodGangSet should be rejected by validation webhook")

		// Verify it's a webhook validation error (admission webhook errors contain validation errors)
		assert.Contains(t, err.Error(), "admission webhook", "Error should be from admission webhook")
		assert.Contains(t, err.Error(), "denied the request", "Error should indicate webhook denial")

		t.Logf("Invalid PodGangSet was correctly rejected: %v", err)
	})
}
