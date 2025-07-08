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

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodCliqueScalingGroupList is a slice of PodCliqueScalingGroup's.
type PodCliqueScalingGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is a slice of PodCliqueScalingGroup.
	Items []PodCliqueScalingGroup `json:"items"`
}

// PodCliqueScalingGroupSpec is the specification of the PodCliqueScalingGroup.
type PodCliqueScalingGroupSpec struct {
	// Replicas is the desired number of replicas for the PodCliqueScalingGroup.
	// If not specified, it defaults to 1.
	Replicas int32 `json:"replicas"`
	// CliqueNames is the list of PodClique names that are configured in the
	// matching PodCliqueScalingGroup in PodGangSet.Spec.Template.PodCliqueScalingGroupConfigs.
	CliqueNames []string `json:"cliqueNames"`
}

// PodCliqueScalingGroupStatus is the status of the PodCliqueScalingGroup.
type PodCliqueScalingGroupStatus struct {
	// Replicas is the observed number of replicas for the PodCliqueScalingGroup.
	Replicas int32 `json:"replicas,omitempty"`
	// Selector is the selector used to identify the pods that belong to this scaling group.
	Selector *string `json:"selector,omitempty"`
	// ObservedGeneration is the most recent generation observed by the controller.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	// LastOperation captures the last operation done by the respective reconciler on the PodClique.
	LastOperation *LastOperation `json:"lastOperation,omitempty"`
	// LastErrors captures the last errors observed by the controller when reconciling the PodClique.
	LastErrors []LastError `json:"lastErrors,omitempty"`
}

// SetLastErrors sets the last errors observed by the controller when reconciling the PodCliqueScalingGroup.
func (pcsg *PodCliqueScalingGroup) SetLastErrors(lastErrs ...LastError) {
	pcsg.Status.LastErrors = lastErrs
}

// SetLastOperation sets the last operation done by the respective reconciler on the PodCliqueScalingGroup.
func (pcsg *PodCliqueScalingGroup) SetLastOperation(lastOp *LastOperation) {
	pcsg.Status.LastOperation = lastOp
}
