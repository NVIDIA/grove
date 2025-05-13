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
	corev1 "k8s.io/api/core/v1"
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
	metav1.ObjectMeta `json:"metadata,omitempty"`
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
	// NetworkPackStrategy defines the strategy for packing pods on nodes while minimizing network switch hops.
	// +optional
	NetworkPackStrategy *NetworkPackStrategy `json:"networkPackStrategy,omitempty"`
	// SpreadConstraints defines the constraints for spreading PodGang's filtered by the same label selector, across domains identified by a topology key.
	// +optional
	SpreadConstraints []corev1.TopologySpreadConstraint `json:"spreadConstraints,omitempty"`
	// PriorityClassName is the name of the PriorityClass to be used for the PodGangSet.
	// If specified, indicates the priority of the PodGangSet. "system-node-critical" and
	// "system-cluster-critical" are two special keywords which indicate the
	// highest priorities with the former being the highest priority. Any other
	// name must be defined by creating a PriorityClass object with that name.
	// If not specified, the pod priority will be default or zero if there is no default.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// TerminationDelay is the delay after which the gang termination will be triggered.
	// A gang is a candidate for termination if number of running pods fall below a threshold for any PodClique.
	// If a PodGang remains a candidate past TerminationDelay then it will be terminated. This allows additional time
	// to the kube-scheduler to re-schedule sufficient pods in the PodGang that will result in having the total number of
	// running pods go above the threshold.
	// +optional
	TerminationDelay *metav1.Duration `json:"terminationDelay,omitempty"`
	// Affinity defines the preferred affinity criteria to another PodGang.
	Affinity *metav1.LabelSelector `json:"affinity,omitempty"`
	// ReuseReservationRef holds the reference to another PodGang resource scheduled previously.
	// During updates, an operator can suggest to reuse the reservation of the previous PodGang for a newer version of the
	// PodGang resource. This is a suggestion for the scheduler and not a requirement that must be met. If the scheduler plugin
	// finds that the reservation done previously was network optimised and there are no better alternatives available, then it
	// will reuse the reservation. If there are better alternatives available, then the scheduler will ignore this suggestion.
	// +optional
	ReuseReservationRef *NamespacedName `json:"reuseReservationRef,omitempty"`
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

// NetworkPackStrategy defines the strategy for packing pods across nodes while minimizing network switch hops.
// An attempt will always be made to ensure that the pods are packed optimally minimizing the total number of network switch hops.
// Pack strategy only describes if this is a strict requirement or a best-effort.
// +kubebuilder:validation:Enum={BestEffort,Strict}
type NetworkPackStrategy string

const (
	// BestEffort pack strategy makes the best effort for optimal placement of pods but does not guarantee it.
	BestEffort NetworkPackStrategy = "BestEffort"
	// Strict pack strategy strives for the most optimal placement for pods assuming sufficient capacity.
	// If optimal placement cannot be achieved then pods will remain pending.
	Strict NetworkPackStrategy = "Strict"
)

// PodGangStatus defines the status of a PodGang.
type PodGangStatus struct {
	// SchedulingPhase is the current phase of scheduling for the PodGang.
	SchedulingPhase string `json:"schedulingPhase"`
	// Conditions is a list of conditions that describe the current state of the PodGang.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// PlacementScore is network optimality score for the PodGang. If the choice that the scheduler has made corresponds to the
	// best possible placement of the pods in the PodGang, then the score will be 1.0. Higher the score, better the placement.
	PlacementScore *float64 `json:"placementScore,omitempty"`
}
