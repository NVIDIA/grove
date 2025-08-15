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

package component

import (
	"context"
	"fmt"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constants for common operations that an Operator can perform.
const (
	// OperationGetExistingResourceNames represents an operation to get existing resource names.
	OperationGetExistingResourceNames = "GetExistingResourceNames"
	// OperationSync represents a sync operation.
	OperationSync = "Sync"
	// OperationDelete represents a delete operation.
	OperationDelete = "Delete"
)

// Following constants provide a name for each managed component whose lifecycle
// is managed by grove operator and are provisioned as part of a PodGangSet
// These component names will be set against grovecorev1alpha1.LabelComponentKey label key on
// respective components.
const (
	// NamePGSPodClique is the component name for a PodClique resource handled by a PodGangSet.
	NamePGSPodClique = "pgs-podclique"
	// NamePCSGPodClique is the component name for a PodClique resource handled by a PodCliqueScalingGroup.
	NamePCSGPodClique = "pcsg-podclique"
	// NamePodGangHeadlessService is the component name for a Headless service for a Pod Gang.
	NamePodGangHeadlessService = "pgs-headless-service"
	// NamePodRole is the component name for a role that is associated to all Pods that are created for a PodGangSet.
	NamePodRole = "pod-role"
	// NamePodRoleBinding is the component name for a RoleBinding to a Role that is associated to all Pods that are created for a PodGangSet.
	NamePodRoleBinding = "pod-role-binding"
	// NamePodServiceAccount is the component name for a ServiceAccount that is used by all Pods that are created for a PodGangSet.
	NamePodServiceAccount = "pod-service-account"
	// NameServiceAccountTokenSecret is the component name for a Secret for generating service account token that is used by start-up order enforcing init container in each Pod for a PodGangSet.
	NameServiceAccountTokenSecret = "pod-sa-token-secret"
	// NamePodCliqueScalingGroup is the component name for a PodCliqueScalingGroup resource.
	NamePodCliqueScalingGroup = "pgs-pod-clique-scaling-group"
	// NameHorizontalPodAutoscaler is the component name for a HorizontalPodAutoscaler that is created for a PodGangSet.
	NameHorizontalPodAutoscaler = "pgs-hpa"
	// NamePodGang is the component name for a PodGang resource.
	NamePodGang = "podgang"
)

// GroveCustomResourceType defines a type bound for generic types.
type GroveCustomResourceType interface {
	grovecorev1alpha1.PodGangSet | grovecorev1alpha1.PodClique | grovecorev1alpha1.PodCliqueScalingGroup
}

// Operator is a facade that manages one or more resources that are provisioned for a PodGangSet.
type Operator[T GroveCustomResourceType] interface {
	// GetExistingResourceNames returns the names of all the existing resources that this Operator manages.
	GetExistingResourceNames(ctx context.Context, logger logr.Logger, objMeta metav1.ObjectMeta) ([]string, error)
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
	// KindPodClique indicates that the resource is a PodClique.
	KindPodClique Kind = "PodClique"
	// KindServiceAccount indicates that the resource is a ServiceAccount.
	KindServiceAccount Kind = "ServiceAccount"
	// KindRole indicates that the resource is a Role.
	KindRole Kind = "Role"
	// KindRoleBinding indicates that the resource is a RoleBinding.
	KindRoleBinding Kind = "RoleBinding"
	// KindServiceAccountTokenSecret indicates that the resource is a Secret to generate ServiceAccount token.
	KindServiceAccountTokenSecret Kind = "ServiceAccountTokenSecret"
	// KindHeadlessService indicates that the resource is a headless Service.
	KindHeadlessService Kind = "HeadlessService"
	// KindHorizontalPodAutoscaler indicates that the resource is a HorizontalPodAutoscaler.
	KindHorizontalPodAutoscaler Kind = "HorizontalPodAutoscaler"
	// KindPod indicates that the resource is a Pod.
	KindPod Kind = "Pod"
	// KindPodCliqueScalingGroup indicates that the resource is a PodCliqueScalingGroup.
	KindPodCliqueScalingGroup Kind = "PodCliqueScalingGroup"
	// KindPodGang indicates that the resource is a PodGang.
	KindPodGang Kind = "PodGang"
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

// NewOperatorRegistry creates a new OperatorRegistry.
func NewOperatorRegistry[T GroveCustomResourceType]() OperatorRegistry[T] {
	return &_registry[T]{
		operators: make(map[Kind]Operator[T]),
	}
}

// Register registers an operator with its associated kind in the registry.
func (r *_registry[T]) Register(kind Kind, operator Operator[T]) {
	r.operators[kind] = operator
}

// GetOperator gets the operator associated with a kind from the registry.
func (r *_registry[T]) GetOperator(kind Kind) (Operator[T], error) {
	operator, ok := r.operators[kind]
	if !ok {
		return nil, fmt.Errorf("operator for kind %s not found", kind)
	}
	return operator, nil
}

// GetAllOperators gets all operators registered.
func (r *_registry[T]) GetAllOperators() map[Kind]Operator[T] {
	return r.operators
}
