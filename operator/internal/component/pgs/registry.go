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

package pgs

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/NVIDIA/grove/operator/internal/component/pgs/podclique"
	"github.com/NVIDIA/grove/operator/internal/component/pgs/role"
	"github.com/NVIDIA/grove/operator/internal/component/pgs/rolebinding"
	"github.com/NVIDIA/grove/operator/internal/component/pgs/service"
	"github.com/NVIDIA/grove/operator/internal/component/pgs/serviceaccount"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// CreateOperatorRegistry initializes the operator registry for the PodGangSet reconciler.
func CreateOperatorRegistry(mgr manager.Manager) component.OperatorRegistry[v1alpha1.PodGangSet] {
	cl := mgr.GetClient()
	reg := component.NewOperatorRegistry[v1alpha1.PodGangSet]()
	reg.Register(component.KindPodClique, podclique.New(cl, mgr.GetScheme()))
	reg.Register(component.KindHeadlessService, service.New(cl, mgr.GetScheme()))
	reg.Register(component.KindRole, role.New(cl, mgr.GetScheme()))
	reg.Register(component.KindRoleBinding, rolebinding.New(cl, mgr.GetScheme()))
	reg.Register(component.KindServiceAccount, serviceaccount.New(cl, mgr.GetScheme()))
	return reg
}
