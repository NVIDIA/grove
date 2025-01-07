package v1alpha1

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.hpaPodSelector
// +kubebuilder:resource:shortName={pclq}

type PodClique struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:",inline"`
	// Spec defines the specification of a PodClique.
	Spec PodCliqueSpec `json:"spec"`
	// Status defines the status of a PodClique.
	Status PodCliqueStatus `json:"status"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodCliqueList is a list of PodClique's.
type PodCliqueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is a slice of PodClique.
	Items []PodClique `json:"items"`
}

// PodCliqueSpec defines the specification of a PodClique.
type PodCliqueSpec struct {
	// Template is the template of the pods in the clique.
	Template corev1.PodTemplateSpec `json:"template"`
	// Replicas is the number of replicas of the pods in the clique.
	Replicas int32 `json:"replicas"`
	// MinAvailable is the minimum number of pods that must be available at any given time.
	// If it is not specified then it will be defaulted to spec.Replicas for the PodClique.
	// +optional
	MinAvailable *int32 `json:"minAvailable"`
	// StartsAfter provides you a way to explicitly define the startup dependencies amongst cliques.
	// If CliqueStartupType in PodGang has been set to 'CliqueStartupTypeExplicit', then to create an ordered start amongst PorClique's StartsAfter can be used.
	// A forest of DAG's can be defined to model any start order dependencies. If there are more than one PodClique's defined and StartsAfter is not set for any of them,
	// then their startup order is random at best and must not be relied upon.
	// Validations:
	// 1. If a StarsAfter has been defined and one or more cycles are detected in DAG's then it will be flagged as validation error.
	// 2. If StartsAfter is defined and does not identify any PodClique then it will be flagged as a validation error.
	// +optional
	StartsAfter []string `json:"startsAfter,omitempty"`
	// HPAConfig is the horizontal pod autoscaler configuration for a PodClique.
	// +optional
	HPAConfig *autoscalingv2.HorizontalPodAutoscalerSpec `json:"hpaConfig,omitempty"`
}

// PodCliqueStatus defines the status of a PodClique.
type PodCliqueStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	// Replicas is the total number of non-terminated Pods targeted by this PodClique.
	Replicas int32 `json:"replicas,omitempty"`
	// ReadyReplicas is the number of ready Pods targeted by this PodClique.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// UpdatedReplicas is the number of Pods that have been updated and are at the desired revision of the PodClique.
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
	// Selector is the label selector that determines which pods are part of the PodClique.
	// PodClique is a unit of scale and this selector is used by HPA to scale the PodClique based on metrics captured for the pods that match this selector.
	Selector *string `json:"hpaPodSelector,omitempty"`
	// Conditions represents the latest available observations of the clique by its controller.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
