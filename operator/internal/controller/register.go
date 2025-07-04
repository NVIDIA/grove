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

package controller

import (
	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/controller/podclique"
	"github.com/NVIDIA/grove/operator/internal/controller/podcliquescalinggroup"
	"github.com/NVIDIA/grove/operator/internal/controller/podgangset"

	ctrl "sigs.k8s.io/controller-runtime"
)

// RegisterControllers registers all controllers with the manager.
func RegisterControllers(mgr ctrl.Manager, controllerConfig configv1alpha1.ControllerConfiguration) error {
	pgsReconciler := podgangset.NewReconciler(mgr, controllerConfig.PodGangSet)
	if err := pgsReconciler.RegisterWithManager(mgr); err != nil {
		return err
	}
	pcReconciler := podclique.NewReconciler(mgr, controllerConfig.PodClique)
	if err := pcReconciler.RegisterWithManager(mgr); err != nil {
		return err
	}
	pcsgReconciler := podcliquescalinggroup.NewReconciler(mgr, controllerConfig.PodCliqueScalingGroup)
	if err := pcsgReconciler.RegisterWithManager(mgr); err != nil {
		return err
	}
	return nil
}
