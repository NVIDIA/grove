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

package podgangset

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/NVIDIA/grove/operator/internal/component/podgangset/hpa"
	"github.com/NVIDIA/grove/operator/internal/component/podgangset/podclique"
	"github.com/NVIDIA/grove/operator/internal/component/podgangset/podcliquescalinggroup"
	"github.com/NVIDIA/grove/operator/internal/component/podgangset/podgang"
	"github.com/NVIDIA/grove/operator/internal/component/podgangset/role"
	"github.com/NVIDIA/grove/operator/internal/component/podgangset/rolebinding"
	"github.com/NVIDIA/grove/operator/internal/component/podgangset/service"
	"github.com/NVIDIA/grove/operator/internal/component/podgangset/serviceaccount"

	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// CreateOperatorRegistry initializes the operator registry for the PodGangSet reconciler.
func CreateOperatorRegistry(mgr manager.Manager, eventRecorder record.EventRecorder) component.OperatorRegistry[v1alpha1.PodGangSet] {
	cl := mgr.GetClient()
	reg := component.NewOperatorRegistry[v1alpha1.PodGangSet]()
	reg.Register(component.KindPodClique, podclique.New(cl, mgr.GetScheme(), eventRecorder))
	reg.Register(component.KindHeadlessService, service.New(cl, mgr.GetScheme()))
	reg.Register(component.KindRole, role.New(cl, mgr.GetScheme()))
	reg.Register(component.KindRoleBinding, rolebinding.New(cl, mgr.GetScheme()))
	reg.Register(component.KindServiceAccount, serviceaccount.New(cl, mgr.GetScheme()))
	reg.Register(component.KindPodCliqueScalingGroup, podcliquescalinggroup.New(cl, mgr.GetScheme(), eventRecorder))
	reg.Register(component.KindHorizontalPodAutoscaler, hpa.New(cl, mgr.GetScheme()))
	reg.Register(component.KindPodGang, podgang.New(cl, mgr.GetScheme(), eventRecorder))
	return reg
}
