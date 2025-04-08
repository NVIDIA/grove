package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	// MemberCliques is a list of member cliques in the PodGang.
	MemberCliques []MemberClique `json:"memberSet"`
}

// MemberClique defines a set of members in a PodGang that share the same PodTemplateSpec.
type MemberClique struct {
	// PodReferences is a list of references to the Pods that are part of this member set.
	PodReferences []types.NamespacedName `json:"podReferences"`
	// MinReplicas is the number of replicas that needs to be gang scheduled.
	// If the MinReplicas is greater than len(PodReferences) then scheduler makes the best effort to schedule as many pods beyond
	// MinReplicas. However, guaranteed gang scheduling is only provided for MinReplicas.
	MinReplicas int32 `json:"minReplicas"`
}

// PodGangStatus defines the status of a PodGang.
type PodGangStatus struct {
	// SchedulingPhase is the current phase of scheduling for the PodGang.
	SchedulingPhase string `json:"schedulingPhase"`
	// Conditions is a list of conditions that describe the current state of the PodGang.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
