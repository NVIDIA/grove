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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={pg}

// PodGang defines a specification of a group of pods that should be scheduled together.
type PodGang struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:",inline"`
	// Spec defines the specification of the PodGang.
	Spec PodGangSpec `json:"spec"`
	// Status defines the status of the PodGang.
	Status PodGangStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodGangList contains a list of PodGang's.
type PodGangList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is a slice of PodGang's.
	Items []PodGang `json:"items"`
}

// PodGangSpec defines the specification of a PodGang.
type PodGangSpec struct {
	// PodGroups is a list of member pod groups in the PodGang.
	PodGroups []PodGroup `json:"podgroups"`
	// TerminationDelay is a delay timer that activates gang termination of a running PodGang
	// whenever the number of running pods violates the minReplicas constraint of any PodGroup.
	// It allows for a grace period to allow the scheduler to schedule the pods again so that they
	// are equal to or greater than the minReplicas, thus preventing unnecessary PodGang termination.
	TerminationDelay metav1.Duration `json:"terminationDelay"`
}

// PodGroup defines a set of pods in a PodGang that share the same PodTemplateSpec.
type PodGroup struct {
	// PodReferences is a list of references to the Pods that are part of this group.
	PodReferences []NamespacedName `json:"podReferences"`
	// MinReplicas is the number of replicas that needs to be gang scheduled.
	// If the MinReplicas is greater than len(PodReferences) then scheduler makes the best effort to schedule as many pods beyond
	// MinReplicas. However, guaranteed gang scheduling is only provided for MinReplicas.
	MinReplicas int32 `json:"minReplicas"`
}

// NamespacedName is a struct that contains the namespace and name of an object.
// types.NamespacedName does not have json tags, so we define our own for the time being.
// If https://github.com/kubernetes/kubernetes/issues/131313 is resolved, we can switch to using the APIMachinery type instead.
type NamespacedName struct {
	// Namespace is the namespace of the object.
	Namespace string `json:"namespace"`
	// Name is the name of the object.
	Name string `json:"name"`
}

// PodGangStatus defines the status of a PodGang.
type PodGangStatus struct {
	// SchedulingPhase is the current phase of scheduling for the PodGang.
	SchedulingPhase string `json:"schedulingPhase"`
	// Conditions is a list of conditions that describe the current state of the PodGang.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
