package pclq

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/NVIDIA/grove/operator/internal/component/pclq/pod"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// CreateOperatorRegistry initializes the operator registry for the PodClique reconciler.
func CreateOperatorRegistry(mgr manager.Manager) component.OperatorRegistry[v1alpha1.PodClique] {
	cl := mgr.GetClient()
	reg := component.NewOperatorRegistry[v1alpha1.PodClique]()
	reg.Register(component.KindPod, pod.New(cl, mgr.GetScheme()))
	return reg
}
