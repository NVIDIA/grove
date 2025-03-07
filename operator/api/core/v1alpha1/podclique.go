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
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec defines the specification of a PodClique.
	Spec PodCliqueSpec `json:"spec"`
	// Status defines the status of a PodClique.
	Status PodCliqueStatus `json:"status,omitempty"`
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
	// Spec is the spec of the pods in the clique.
	PodSpec corev1.PodSpec `json:"podSpec"`
	// Replicas is the number of replicas of the pods in the clique.
	Replicas int32 `json:"replicas"`
	// StartsAfter provides you a way to explicitly define the startup dependencies amongst cliques.
	// If CliqueStartupType in PodGang has been set to 'CliqueStartupTypeExplicit', then to create an ordered start amongst PorClique's StartsAfter can be used.
	// A forest of DAG's can be defined to model any start order dependencies. If there are more than one PodClique's defined and StartsAfter is not set for any of them,
	// then their startup order is random at best and must not be relied upon.
	// Validations:
	// 1. If a StarsAfter has been defined and one or more cycles are detected in DAG's then it will be flagged as validation error.
	// 2. If StartsAfter is defined and does not identify any PodClique then it will be flagged as a validation error.
	// +optional
	StartsAfter []string `json:"startsAfter,omitempty"`
	// ScaleConfig is the horizontal pod autoscaler configuration for a PodClique.
	// +optional
	ScaleConfig *AutoScalingConfig `json:"autoScalingConfig,omitempty"`
}

// AutoScalingConfig defines the configuration for the horizontal pod autoscaler for a PodClique.
type AutoScalingConfig struct {
	// minReplicas is the lower limit for the number of replicas to which the autoscaler
	// can scale down. It defaults to 1 pod. minReplicas is not allowed to be 0.
	// Scaling is active as long as at least one metric value is available.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// maxReplicas is the upper limit for the number of replicas to which the autoscaler can scale up.
	// It cannot be less that minReplicas.
	MaxReplicas int32 `json:"maxReplicas"`
	// Metrics contains the specifications for which to use to calculate the
	// desired replica count (the maximum replica count across all metrics will
	// be used).  The desired replica count is calculated multiplying the
	// ratio between the target value and the current value by the current
	// number of pods.  Ergo, metrics used must decrease as the pod count is
	// increased, and vice versa.  See the individual metric source types for
	// more information about how each type of metric must respond.
	// If not set, the default metric will be set to 80% average CPU utilization.
	// +listType=atomic
	// +optional
	Metrics []autoscalingv2.MetricSpec `json:"metrics,omitempty" protobuf:"bytes,4,rep,name=metrics"`
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
