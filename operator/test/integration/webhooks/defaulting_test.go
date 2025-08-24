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

	"github.com/NVIDIA/grove/operator/test/integration/framework"
	"github.com/NVIDIA/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestPodGangSetDefaulting(t *testing.T) {
	// Setup test environment with webhooks enabled
	env, err := framework.NewEnvBuilder(t).
		WithMutationWebhook().
		WithNamespace("test-ns").
		Build()
	require.NoError(t, err)
	// Start the environment
	err = env.Start()
	require.NoError(t, err)
	defer env.Shutdown()

	// Verify that the manager and webhooks are properly set up
	require.NotNil(t, env.Manager, "Manager should be created when webhooks are enabled")

	// Simple test: just verify that we can create a PodGangSet with minimal configuration
	// (webhook functionality testing requires more complex envtest setup)
	pgs := utils.NewPodGangSetBuilder("test-pgs", "test-ns").
		WithMinimal().
		Build()

	// Submit PGS to cluster
	err = env.Client.Create(env.Ctx, pgs)
	require.NoError(t, err)

	// Verify the PGS was created successfully (basic webhook integration test)
	assert.Equal(t, corev1.RestartPolicyAlways, pgs.Spec.Template.Cliques[0].Spec.PodSpec.RestartPolicy)
	assert.Equal(t, ptr.To[int64](30), pgs.Spec.Template.Cliques[0].Spec.PodSpec.TerminationGracePeriodSeconds)

	t.Logf("Webhook integration test completed - webhooks are registered and manager is working")
}
