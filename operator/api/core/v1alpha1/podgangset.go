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
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.hpaPodSelector
// +kubebuilder:resource:shortName={pgs}

// PodGangSet is a set of PodGangs defining specification on how to spread and manage a gang of pods and monitoring their status.
type PodGangSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec defines the specification of the PodGangSet.
	Spec PodGangSetSpec `json:"spec"`
	// Status defines the status of the PodGangSet.
	Status PodGangSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodGangSetList is a list of PodGangSet's.
type PodGangSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is a slice of PodGangSets.
	Items []PodGangSet `json:"items"`
}

// PodGangSetSpec defines the specification of a PodGangSet.
type PodGangSetSpec struct {
	// Replicas is the number of desired replicas of the PodGang.
	// +kubebuilder:default=0
	Replicas int32 `json:"replicas,omitempty"`
	// Template describes the template spec for PodGangs that will be created in the PodGangSet.
	Template PodGangSetTemplateSpec `json:"template"`
	// ReplicaSpreadConstraints defines the constraints for spreading each replica of PodGangSet across domains identified by a topology key.
	// +optional
	ReplicaSpreadConstraints []corev1.TopologySpreadConstraint `json:"replicaSpreadConstraints,omitempty"`
}

// PodGangSetStatus defines the status of a PodGangSet.
type PodGangSetStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	// LastOperation captures the last operation done by the respective reconciler on the PodGangSet.
	LastOperation *LastOperation `json:"lastOperation,omitempty"`
	// LastErrors captures the last errors observed by the controller when reconciling the PodGangSet.
	LastErrors []LastError `json:"lastErrors,omitempty"`
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

// PodGangSetTemplateSpec defines a template spec for a PodGang.
// A PodGang does not have a RestartPolicy field because the restart policy is predefined:
// If the number of pods in any of the cliques falls below the threshold, the entire PodGang will be restarted.
// The threshold is determined by either:
// - The value of "MinReplicas", if specified in the ScaleConfig of that clique, or
// - The "Replicas" value of that clique
type PodGangSetTemplateSpec struct {
	// Cliques is a slice of cliques that make up the PodGang. There should be at least one PodClique.
	Cliques []*PodCliqueTemplateSpec `json:"cliques"`
	// StartupType defines the type of startup dependency amongst the cliques within a PodGang.
	// If it is not defined then default of CliqueStartupTypeAnyOrder is used.
	// +kubebuilder:default=CliqueStartupTypeAnyOrder
	// +optional
	StartupType *CliqueStartupType `json:"cliqueStartupType,omitempty"`
	// PriorityClassName is the name of the PriorityClass to be used for the PodGangSet.
	// If specified, indicates the priority of the PodGangSet. "system-node-critical" and
	// "system-cluster-critical" are two special keywords which indicate the
	// highest priorities with the former being the highest priority. Any other
	// name must be defined by creating a PriorityClass object with that name.
	// If not specified, the pod priority will be default or zero if there is no default.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// HeadlessServiceConfig defines the config options for the headless service.
	// If present, create headless service for each PodGang.
	// +optional
	HeadlessServiceConfig *HeadlessServiceConfig `json:"headlessServiceConfig,omitempty"`
	// SchedulingPolicyConfig defines the scheduling policy configuration for the PodGang.
	// Defaulting only works for optional fields.
	// See https://github.com/kubernetes-sigs/controller-tools/issues/893#issuecomment-1991256368
	// +kubebuilder:default:={networkPackStrategy:BestEffort}
	SchedulingPolicyConfig *SchedulingPolicyConfig `json:"schedulingPolicyConfig,omitempty"`
	// PodCliqueScalingGroupConfigs is a list of scaling groups for the PodGangSet.
	PodCliqueScalingGroupConfigs []PodCliqueScalingGroupConfig `json:"podCliqueScalingGroups,omitempty"`
}

// PodCliqueTemplateSpec defines a template spec for a PodClique.
type PodCliqueTemplateSpec struct {
	// Name must be unique within a PodGangSet and is used to denote a role.
	// Once set it cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#names
	Name string `json:"name"`

	// Labels is a map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Specification of the desired behavior of a PodClique.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec PodCliqueSpec `json:"spec"`
}

// SchedulingPolicyConfig defines the scheduling policy configuration for the PodGang.
type SchedulingPolicyConfig struct {
	// NetworkPackGroupConfigs is a list of NetworkPackGroupConfig's that define how the pods in the PodGangSet are optimally packaged w.r.t cluster's network topology.
	// PodCliques that are not part of any NetworkPackGroupConfig are scheduled with best-effort network packing strategy.
	// Exercise caution when defining NetworkPackGroupConfig. Some of the downsides include:
	// 1. Scheduling may be delayed until optimal placement is available.
	// 2. Pods created due to scale-out or rolling upgrades is not guaranteed optimal placement.
	NetworkPackGroupConfigs []NetworkPackGroupConfig `json:"networkPackGroupConfigs,omitempty"`
	// TerminationDelay is the delay after which the gang termination will be triggered.
	// A gang is a candidate for termination if number of running pods fall below a threshold for any PodClique.
	// If a PodGang remains a candidate past TerminationDelay then it will be terminated. This allows additional time
	// to the kube-scheduler to re-schedule sufficient pods in the PodGang that will result in having the total number of
	// running pods go above the threshold.
	// +optional
	TerminationDelay *metav1.Duration `json:"terminationDelay,omitempty"`
}

// NetworkPackGroupConfig indicates that all the Pods belonging to the constituent PodClique's should be optimally placed w.r.t cluster's network topology.
// If a constituent PodClique belongs to a PodCliqueScalingGroup then ensure that all constituent PodClique's of that PodCliqueScalingGroup are also part of the NetworkPackGroupConfig.
type NetworkPackGroupConfig struct {
	// CliqueNames is the list of PodClique names that are part of the network pack group.
	CliqueNames []string `json:"cliqueNames"`
}

// PodCliqueScalingGroupConfig is a group of PodClique's that are scaled together.
// Each member PodClique.Replicas will be computed as a product of PodCliqueScalingGroupConfig.Replicas and PodCliqueTemplateSpec.Spec.Replicas.
// NOTE: If a PodCliqueScalingGroupConfig is defined, then for the member PodClique's, individual AutoScalingConfig cannot be defined.
type PodCliqueScalingGroupConfig struct {
	// Name is the name of the PodCliqueScalingGroupConfig. This should be unique within the PodGangSet.
	// It allows consumers to give a semantic name to a group of PodCliques that needs to be scaled together.
	Name string `json:"name"`
	// CliqueNames is the list of names of the PodClique's that are part of the scaling group.
	CliqueNames []string `json:"cliqueNames"`
	// ScaleConfig is the horizontal pod autoscaler configuration for the pod clique scaling group.
	// +optional
	ScaleConfig *AutoScalingConfig `json:"scaleConfig,omitempty"`
}

// HeadlessServiceConfig defines the config options for the headless service.
type HeadlessServiceConfig struct {
	// PublishNotReadyAddresses if set to true will publish the DNS records of pods even if the pods are not ready.
	PublishNotReadyAddresses bool `json:"publishNotReadyAddresses"`
}

// CliqueStartupType defines the order in which each PodClique is started.
// +kubebuilder:validation:Enum={CliqueStartupTypeAnyOrder,CliqueStartupTypeInOrder,CliqueStartupTypeExplicit}
type CliqueStartupType string

const (
	// CliqueStartupTypeAnyOrder defines that the cliques can be started in any order. This allows for concurrent starts of cliques.
	// This is the default CliqueStartupType.
	CliqueStartupTypeAnyOrder CliqueStartupType = "CliqueStartupTypeAnyOrder"
	// CliqueStartupTypeInOrder defines that the cliques should be started in the order they are defined in the PodGang Cliques slice.
	CliqueStartupTypeInOrder CliqueStartupType = "CliqueStartupTypeInOrder"
	// CliqueStartupTypeExplicit defines that the cliques should be started after the cliques defined in PodClique.StartsAfter have started.
	CliqueStartupTypeExplicit CliqueStartupType = "CliqueStartupTypeExplicit"
)

// PodGangStatus defines the status of a PodGang.
type PodGangStatus struct {
	// Name is the name of the PodGang.
	Name string `json:"name"`
	// Phase is the current phase of the PodGang.
	Phase PodGangPhase `json:"phase"`
	// Conditions represents the latest available observations of the PodGang by its controller.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

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

// LastOperationType is a string alias for the type of the last operation.
type LastOperationType string

const (
	// LastOperationTypeReconcile indicates that the last operation was a reconcile operation.
	LastOperationTypeReconcile LastOperationType = "Reconcile"
	// LastOperationTypeDelete indicates that the last operation was a delete operation.
	LastOperationTypeDelete LastOperationType = "Delete"
)

// LastOperationState is a string alias for the state of the last operation.
type LastOperationState string

const (
	// LastOperationStateProcessing indicates that the last operation is in progress.
	LastOperationStateProcessing LastOperationState = "Processing"
	// LastOperationStateSucceeded indicates that the last operation succeeded.
	LastOperationStateSucceeded LastOperationState = "Succeeded"
	// LastOperationStateError indicates that the last operation completed with errors and will be retried.
	LastOperationStateError LastOperationState = "Error"
)

// LastOperation captures the last operation done by the respective reconciler on the PodGangSet.
type LastOperation struct {
	// Type is the type of the last operation.
	Type LastOperationType `json:"type"`
	// State is the state of the last operation.
	State LastOperationState `json:"state"`
	// Description is a human-readable description of the last operation.
	Description string `json:"description"`
	// LastUpdateTime is the time at which the last operation was updated.
	LastUpdateTime metav1.Time `json:"lastTransitionTime"`
}

// ErrorCode is a custom error code that uniquely identifies an error.
type ErrorCode string

// LastError captures the last error observed by the controller when reconciling an object.
type LastError struct {
	// Code is the error code that uniquely identifies the error.
	Code ErrorCode `json:"code"`
	// Description is a human-readable description of the error.
	Description string `json:"description"`
	// ObservedAt is the time at which the error was observed.
	ObservedAt metav1.Time `json:"observedAt"`
}

// SetLastErrors sets the last errors observed by the controller when reconciling the PodGangSet.
func (pgs *PodGangSet) SetLastErrors(lastErrs ...LastError) {
	pgs.Status.LastErrors = lastErrs
}

// SetLastOperation sets the last operation done by the respective reconciler on the PodGangSet.
func (pgs *PodGangSet) SetLastOperation(operation *LastOperation) {
	pgs.Status.LastOperation = operation
}
