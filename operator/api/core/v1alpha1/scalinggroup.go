package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:resource:shortName={pcsg}

// PodCliqueScalingGroup is the schema to define scaling groups that is used to scale a group of PodClique's.
// An instance of this custom resource will be created for every pod clique scaling group defined as part of PodGangSet.
type PodCliqueScalingGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the specification of the PodCliqueScalingGroup.
	Spec PodCliqueScalingGroupSpec `json:"spec"`
	// Status is the status of the PodCliqueScalingGroup.
	Status PodCliqueScalingGroupStatus `json:"status,omitempty"`
}

// PodCliqueScalingGroupSpec is the specification of the PodCliqueScalingGroup.
type PodCliqueScalingGroupSpec struct {
	// Replicas is the desired number of replicas for the PodCliqueScalingGroup.
	Replicas int32 `json:"replicas"`
}

// PodCliqueScalingGroupStatus is the status of the PodCliqueScalingGroup.
type PodCliqueScalingGroupStatus struct {
	// Replicas is the observed number of replicas for the PodCliqueScalingGroup.
	Replicas int32 `json:"replicas"`
	// Selector is the selector used to identify the pods that belong to this scaling group.
	Selector *string `json:"selector"`
}
