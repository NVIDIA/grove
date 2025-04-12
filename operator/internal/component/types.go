package component

import (
	"context"
	"fmt"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constants for common operations that an Operator can perform.
const (
	// OperationSync represents a sync operation.
	OperationSync = "Sync"
	// OperationDelete represents a delete operation.
	OperationDelete = "Delete"
)

// Following components provide a name for each managed component whose lifecycle
// is managed by grove operator and are provisioned as part of a PodGangSet
// These component names will be set against v1alpha1.LabelComponentKey label key on
// respective components.
const (
	// NamePodClique is the value for v1alpha1.LabelComponentKey for a PodClique resource.
	NamePodClique = "pgs-podclique"
	// NamePodGangHeadlessService is the value for v1alpha1.LabelComponentKey for a Headless service for a Pod Gang.
	NamePodGangHeadlessService = "pgs-headless-service"
)

// GroveCustomResourceType defines a type bound for generic types.
type GroveCustomResourceType interface {
	v1alpha1.PodGangSet | v1alpha1.PodClique
}

// Operator is a facade that manages one or more resources that are provisioned for a PodGangSet.
type Operator[T GroveCustomResourceType] interface {
	// Sync synchronizes all resources that this Operator manages. If a component does not exist then it will
	// create it. If there are changes in the owning PodGangSet resource that transpires changes to one or more resources
	// managed by this Operator then those component(s) will be either be updated or a deletion is triggered.
	Sync(ctx context.Context, logger logr.Logger, obj *T) error
	// Delete triggers the deletion of all resources that this Operator manages.
	Delete(ctx context.Context, logger logr.Logger, objMeta metav1.ObjectMeta) error
}

// Kind represents kind of a resource.
type Kind string

const (
	// KindPodGangSet indicates that kind of the component is a PodGangSet.
	KindPodGangSet Kind = "PodGangSet"
	// KindPodClique indicates that the resource is a PodClique.
	KindPodClique Kind = "PodClique"
	// KindServiceAccount indicates that the resource is a ServiceAccount.
	KindServiceAccount Kind = "ServiceAccount"
	// KindRole indicates that the resource is a Role.
	KindRole Kind = "Role"
	// KindRoleBinding indicates that the resource is a RoleBinding.
	KindRoleBinding Kind = "RoleBinding"
	// KindHeadlessService indicates that the resource is a headless Service.
	KindHeadlessService Kind = "HeadlessService"
	// KindNetworkPolicy indicates that the resource is a NetworkPolicy.
	KindNetworkPolicy Kind = "NetworkPolicy"
	// KindHorizontalPodAutoscaler indicates that the resource is a HorizontalPodAutoscaler.
	KindHorizontalPodAutoscaler Kind = "HorizontalPodAutoscaler"
	// KindPod indicates that the resource is a Pod.
	KindPod Kind = "Pod"
	// KindPersistentVolumeClaim indicates that the resource is a PersistentVolumeClaim.
	KindPersistentVolumeClaim Kind = "PersistentVolumeClaim"
)

// OperatorRegistry is a facade that gives access to all component operators.
type OperatorRegistry[T GroveCustomResourceType] interface {
	// Register registers a component operator against the kind of component it operates on.
	Register(kind Kind, operator Operator[T])
	// GetOperator gets a component operator that operates on the given kind.
	GetOperator(kind Kind) (Operator[T], error)
	// GetAllOperators returns all component operators.
	GetAllOperators() map[Kind]Operator[T]
}

type _registry[T GroveCustomResourceType] struct {
	operators map[Kind]Operator[T]
}

func NewOperatorRegistry[T GroveCustomResourceType]() OperatorRegistry[T] {
	return &_registry[T]{
		operators: make(map[Kind]Operator[T]),
	}
}

func (r *_registry[T]) Register(kind Kind, operator Operator[T]) {
	r.operators[kind] = operator
}

func (r *_registry[T]) GetOperator(kind Kind) (Operator[T], error) {
	operator, ok := r.operators[kind]
	if !ok {
		return nil, fmt.Errorf("operator for kind %s not found", kind)
	}
	return operator, nil
}

func (r *_registry[T]) GetAllOperators() map[Kind]Operator[T] {
	return r.operators
}
