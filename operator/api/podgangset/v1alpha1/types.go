// /*
// Copyright 2024 The Grove Authors.
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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PodGangPhase represents the phase of a PodGang.
// +kubebuilder:validation:Enum={Pending,Starting,Running,Failed,Succeeded}
type PodGangPhase string

const (
	// PodGangPending indicates that the pods in a PodGang have not yet been taken up for scheduling.
	PodGangPending PodGangPhase = "Pending"
	// PodGangStarting indicates that the pods are bound to nodes by the scheduler and are starting.
	PodGangStarting PodGangPhase = "Starting"
	// PodGangRunning indicates that the all the pods in a PodGang are running.
	PodGangRunning PodGangPhase = "Running"
	// PodGangFailed indicates that one or more pods in a PodGang have failed.
	// This is a terminal state and is typically used for batch jobs.
	PodGangFailed PodGangPhase = "Failed"
	// PodGangSucceeded indicates that all the pods in a PodGang have succeeded.
	// This is a terminal state and is typically used for batch jobs.
	PodGangSucceeded PodGangPhase = "Succeeded"
)

// PodGangRestartPolicy describes how the PodGang should be restarted. PodGang is the unit of restart.
// If no restart policy is set then it defaults to Always.
// +kubebuilder:validation:Enum={Never,OnFailure,Always}
// +kubebuilder:default=Always
type PodGangRestartPolicy string

const (
	// GangRestartPolicyNever indicates that the PodGang should never be restarted.
	GangRestartPolicyNever PodGangRestartPolicy = "Never"
	// GangRestartPolicyOnFailure indicates that the PodGang should be restarted only when it fails.
	GangRestartPolicyOnFailure PodGangRestartPolicy = "OnFailure"
	// GangRestartPolicyAlways indicates that the PodGang should always be restarted.
	GangRestartPolicyAlways PodGangRestartPolicy = "Always"
)

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

// GangUpdateStrategyType defines the strategy to be used when updating a PodGang which is the unit of update.
// If no update strategy is set then it defaults to "Recreate".
// +kubebuilder:validation:Enum={RollingUpdate,Recreate}
// +kubebuilder:default=Recreate
type GangUpdateStrategyType string

const (
	// GangUpdateStrategyRolling indicates that the PodGang should be updated in a rolling fashion.
	// When rolling the availability is guaranteed, but it is possible that a most network optimal placement of pods within a PodGang is no longer possible.
	GangUpdateStrategyRolling GangUpdateStrategyType = "RollingUpdate"
	// GangUpdateStrategyRecreate indicates that the PodGang should be recreated instead of getting rolled.
	// Unless the resource requirements or the total number of Pods within the PodGang have not changed, the previous placement of Pods will be retained.
	GangUpdateStrategyRecreate GangUpdateStrategyType = "Recreate"
)

// CliqueStartupType defines the order in which each PodClique is started.
// +kubebuilder:validation:Enum={CliqueStartupTypeInOrder,CliqueStartupTypeExplicit}
// +kubebuilder:default=CliqueStartupTypeInOrder
type CliqueStartupType string

const (
	// CliqueStartupTypeInOrder defines that the cliques should be started in the order they are defined in the PodGang Cliques slice.
	// This is the default CliqueStartupType.
	CliqueStartupTypeInOrder CliqueStartupType = "CliqueStartupTypeInOrder"
	// CliqueStartupTypeExplicit defines that the cliques should be started after the cliques defined in PodClique.StartsAfter have started.
	CliqueStartupTypeExplicit CliqueStartupType = "CliqueStartupTypeExplicit"
)

// PodClique defines a set of pods that share the same PodSpec and serve as a single functional unit.
type PodClique struct {
	//Name is a unique clique name.
	Name string `json:"name"`
	// Template is the template of the pods in the clique.
	Template corev1.PodTemplateSpec `json:"template"`
	// Size is the number of pods in the clique. Once set this cannot be changed.
	// If not specified then it will be defaulted to 1.
	// +optional
	Size *int32 `json:"size,omitempty"`
	// StartsAfter provides you a way to explicitly define the startup dependencies amongst cliques.
	// If CliqueStartupType in PodGang has been set to 'CliqueStartupTypeExplicit', then to create an ordered start amongst PorClique's StartsAfter can be used.
	// A forest of DAG's can be defined to model any start order dependencies. If there are more than one PodClique's defined and StartsAfter is not set for any of them,
	// then their startup order is random at best and must not be relied upon.
	// Validations:
	// 1. If a StarsAfter has been defined and one or more cycles are detected in DAG's then it will be flagged as validation error.
	// 2. If StartsAfter is defined and does not identify any PodClique then it will be flagged as a validation error.
	// +optional
	StartsAfter []string `json:"startsAfter,omitempty"`
}

// CliqueStatus defines the status of a clique.
type CliqueStatus struct {
	// Name is the name of the clique.
	Name string `json:"name"`
	// Conditions represents the latest available observations of the clique by its controller.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// PodGangStatus defines the status of a PodGang.
type PodGangStatus struct {
	// Name is the name of the PodGang.
	Name string `json:"name"`
	// Phase is the current phase of the PodGang.
	Phase PodGangPhase `json:"phase"`
	// Conditions represents the latest available observations of the PodGang by its controller.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// CliqueStatutes represents the status of each clique in the PodGang.
	CliqueStatutes []CliqueStatus `json:"cliqueStatuses,omitempty"`
}

// RollingUpdateConfiguration is the configuration to control the desired behavior of a rolling update of a PodGang.
type RollingUpdateConfiguration struct {
	// The maximum number of podgangs that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of total podgangs at the start of update (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// By default, a fixed value of 1 is used.
	// Example: when this is set to 30%, the old RC can be scaled down by 30%
	// immediately when the rolling update starts. Once new podgangs are ready, old RC
	// can be scaled down further, followed by scaling up the new RC, ensuring
	// that at least 70% of original number of podgangs are available at all times
	// during the update.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// The maximum number of podgangs that can be scheduled above the original number of
	// podgangs.
	// Value can be an absolute number (ex: 5) or a percentage of total podgangs at
	// the start of the update (ex: 10%). This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// By default, a value of 1 is used.
	// Example: when this is set to 30%, the new RC can be scaled up by 30%
	// immediately when the rolling update starts. Once old podgangs have been killed,
	// new RC can be scaled up further, ensuring that total number of podgangs running
	// at any time during the update is at most 130% of original podgangs.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
}

// GangUpdateStrategy defines the strategy to be used when updating a PodGang.
type GangUpdateStrategy struct {
	// Type is the type of update strategy.
	Type GangUpdateStrategyType `json:"type"`
	// RollingUpdateConfig is the configuration to control the desired behavior of a rolling update of a PodGang.
	// +optional
	RollingUpdateConfig *RollingUpdateConfiguration `json:"rollingUpdateConfig,omitempty"`
}

// PodGangTemplateSpec defines a template spec for a PodGang.
type PodGangTemplateSpec struct {
	// Cliques is a slice of cliques that make up the PodGang. There should be at least one PodClique.
	Cliques []PodClique `json:"cliques"`
	// StartupType defines the type of startup dependency amongst the cliques within a PodGang.
	// +optional
	StartupType *CliqueStartupType `json:"cliqueStartupType,omitempty"`
	// RestartPolicy defines the restart policy for the PodGang.
	// +optional
	RestartPolicy *PodGangRestartPolicy `json:"restartPolicy,omitempty"`
	// NetworkPackStrategy defines the strategy for packing pods on nodes while minimizing network switch hops.
	// +optional
	NetworkPackStrategy *NetworkPackStrategy `json:"networkPackStrategy,omitempty"`
}

// PodGangSetSpec defines the specification of a PodGangSet.
type PodGangSetSpec struct {
	// Template describes the template spec for PodGangs that will be created in the PodGangSet.
	Template PodGangTemplateSpec `json:"template"`
	// Replicas is the number of desired replicas of the PodGang.
	// +kubebuilder:default=0
	Replicas int32 `json:"replicas,omitempty"`
	// UpdateStrategy defines the strategy to be used when updating the PodGangs.
	// +optional
	UpdateStrategy *GangUpdateStrategy `json:"updateStrategy,omitempty"`
	// GangSpreadConstraints defines the constraints for spreading PodGang's across domains identified by a topology.
	// +optional
	GangSpreadConstraints []corev1.TopologySpreadConstraint `json:"gangSpreadConstraints,omitempty"`
}

// PodGangSetStatus defines the status of a PodGangSet.
type PodGangSetStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	// Replicas is the total number of non-terminated PodGangs targeted by this PodGangSet.
	Replicas int32 `json:"replicas,omitempty"`
	// ReadyReplicas is the number of ready PodGangs targeted by this PodGangSet.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// UpdatedReplicas is the number of PodGangs that have been updated and are at the desired revision of the PodGangSet.
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
	// Selector is the label selector that determines which pods are part of the PodGang.
	// PodGang is a unit of scale and this selector is used by HPA to scale the PodGang based on metrics captured for the pods that match this selector.
	Selector *string `json:"hpaPodSelector,omitempty"`
	// PodGangStatuses captures the status for all the PodGang's that are part of the PodGangSet.
	PodGangStatutes []PodGangStatus `json:"podGangStatuses,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:resource:shortName={podgangset}

// PodGangSet is a set of PodGangs defining specification on how to spread and manage PodGangs and monitoring their status.
type PodGangSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:",inline"`
	// Spec defines the specification of the PodGangSet.
	Spec PodGangSetSpec `json:"spec"`
	// Status defines the status of the PodGangSet.
	Status PodGangSetStatus `json:"status"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodGangSetList is a list of PodGangSets.
type PodGangSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is a slice of PodGangSets.
	Items []PodGangSet `json:"items"`
}
