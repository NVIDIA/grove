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
	"testing"

	groveconfigv1alpha1 "github.com/ai-dynamo/grove/operator/api/config/v1alpha1"
	testutils "github.com/ai-dynamo/grove/operator/test/utils"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// TestRegisterWithManager tests webhook registration with the controller manager.
func TestRegisterWithManager(t *testing.T) {
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

	handler := NewHandler(mgr, groveconfigv1alpha1.AuthorizerConfig{
		Enabled: true,
	}, "system:serviceaccount:default:test-sa")
	err := handler.RegisterWithManager(mgr)
	require.NoError(t, err)
}
