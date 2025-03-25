package pgs

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/NVIDIA/grove/operator/internal/component/pgs/podclique"
	"github.com/NVIDIA/grove/operator/internal/component/pgs/service"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// CreateOperatorRegistry initializes the operator registry for the PodGangSet reconciler.
func CreateOperatorRegistry(mgr manager.Manager) component.OperatorRegistry[v1alpha1.PodGangSet] {
	cl := mgr.GetClient()
	reg := component.NewOperatorRegistry[v1alpha1.PodGangSet]()
	reg.Register(component.KindPodClique, podclique.New(cl, mgr.GetScheme()))
	reg.Register(component.KindHeadlessService, service.New(cl))
	return reg
}
