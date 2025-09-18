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

// Package podcliquescalinggroup provides component operators for managing PodCliqueScalingGroup resources.
// It orchestrates the creation, updating, and deletion of PodClique resources based on scaling requirements.
package podcliquescalinggroup

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/NVIDIA/grove/operator/internal/component/podcliquescalinggroup/podclique"

	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// CreateOperatorRegistry creates and configures an operator registry for PodCliqueScalingGroup reconciliation.
// It registers all component operators needed to manage PodCliqueScalingGroup resources and their dependencies.
func CreateOperatorRegistry(mgr manager.Manager, eventRecorder record.EventRecorder) component.OperatorRegistry[v1alpha1.PodCliqueScalingGroup] {
	// Initialize the operator registry for PodCliqueScalingGroup resources
	reg := component.NewOperatorRegistry[v1alpha1.PodCliqueScalingGroup]()

	// Register the PodClique operator to handle PodClique lifecycle management
	reg.Register(component.KindPodClique, podclique.New(mgr.GetClient(), mgr.GetScheme(), eventRecorder))

	return reg
}
