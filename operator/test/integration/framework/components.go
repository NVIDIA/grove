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

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/controller/podclique"
	"github.com/NVIDIA/grove/operator/internal/controller/podcliquescalinggroup"
	"github.com/NVIDIA/grove/operator/internal/controller/podgangset"
	"github.com/NVIDIA/grove/operator/internal/webhook/admission/pgs/defaulting"
	"github.com/NVIDIA/grove/operator/internal/webhook/admission/pgs/validation"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// RegisterPodGangSetController registers the PodGangSet controller with the manager
func RegisterPodGangSetController(mgr manager.Manager, t *testing.T) error {
	config := configv1alpha1.PodGangSetControllerConfiguration{
		ConcurrentSyncs: ptr.To(1),
	}
	t.Logf("Creating PodGangSet reconciler with concurrency=%d", *config.ConcurrentSyncs)
	reconciler := podgangset.NewReconciler(mgr, config)
	if err := reconciler.RegisterWithManager(mgr); err != nil {
		return fmt.Errorf("failed to register PodGangSet controller: %w", err)
	}
	return nil
}

// RegisterPodCliqueController registers the PodClique controller with the manager
func RegisterPodCliqueController(mgr manager.Manager, t *testing.T) error {
	config := configv1alpha1.PodCliqueControllerConfiguration{
		ConcurrentSyncs: ptr.To(1),
	}
	t.Logf("Creating PodClique reconciler with concurrency=%d", *config.ConcurrentSyncs)
	reconciler := podclique.NewReconciler(mgr, config)
	if err := reconciler.RegisterWithManager(mgr); err != nil {
		return fmt.Errorf("failed to register PodClique controller: %w", err)
	}
	return nil
}

// RegisterScalingGroupController registers the PodCliqueScalingGroup controller with the manager
func RegisterScalingGroupController(mgr manager.Manager, t *testing.T) error {
	config := configv1alpha1.PodCliqueScalingGroupControllerConfiguration{
		ConcurrentSyncs: ptr.To(1),
	}
	t.Logf("Creating PodCliqueScalingGroup reconciler with concurrency=%d", *config.ConcurrentSyncs)
	reconciler := podcliquescalinggroup.NewReconciler(mgr, config)
	if err := reconciler.RegisterWithManager(mgr); err != nil {
		return fmt.Errorf("failed to register PodCliqueScalingGroup controller: %w", err)
	}
	return nil
}

// RegisterValidationWebhook registers the validation webhook with the manager
func RegisterValidationWebhook(mgr manager.Manager, t *testing.T) error {
	t.Logf("Creating validation webhook handler")
	validatingWebhook := validation.NewHandler(mgr)
	if err := validatingWebhook.RegisterWithManager(mgr); err != nil {
		return fmt.Errorf("failed to register validation webhook: %w", err)
	}
	return nil
}

// RegisterMutationWebhook registers the mutation webhook with the manager
func RegisterMutationWebhook(mgr manager.Manager, t *testing.T) error {
	t.Logf("Creating mutation webhook handler")
	defaultingWebhook := defaulting.NewHandler(mgr)
	if err := defaultingWebhook.RegisterWithManager(mgr); err != nil {
		return fmt.Errorf("failed to register mutation webhook: %w", err)
	}
	return nil
}

// CreateNamespaces creates the provided namespaces using the given client
func CreateNamespaces(ctx context.Context, client client.Client, namespaces map[string]*corev1.Namespace, t *testing.T) error {
	for _, ns := range namespaces {
		t.Logf("Creating namespace: name=%s, labels=%v", ns.Name, ns.Labels)
		if err := client.Create(ctx, ns); err != nil {
			return fmt.Errorf("failed to create namespace %s: %w", ns.Name, err)
		}
		t.Logf("Namespace %s created successfully", ns.Name)
	}
	return nil
}
