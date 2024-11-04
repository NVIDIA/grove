// /*
// Copyright 2024.
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

package pgs

import (
	"github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const controllerName = "podgangset-controller"

func (r *Reconciler) RegisterWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{}).
		For(&v1alpha1.PodGang{}).
		Complete(r)
}
