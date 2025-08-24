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

// Common label keys to be placed on all resources managed by grove operator.
const (
	// LabelAppNameKey is a key of a label which sets the name of the resource.
	LabelAppNameKey = "app.kubernetes.io/name"
	// LabelManagedByKey is a key of a label which sets the operator which manages this resource.
	LabelManagedByKey = "app.kubernetes.io/managed-by"
	// LabelPartOfKey is a key of a label which sets the type of the resource.
	LabelPartOfKey = "app.kubernetes.io/part-of"
	// LabelManagedByValue is the value for LabelManagedByKey
	LabelManagedByValue = "grove-operator"
	// LabelComponentKey is a key for a label that sets the component type on resources provisioned for a PodGangSet.
	LabelComponentKey = "app.kubernetes.io/component"
	// LabelPodClique is a key for a label that sets the PodClique name.
	LabelPodClique = "grove.io/podclique"
	// LabelPodGang is a key for a label that sets the PodGang name.
	LabelPodGang = "grove.io/podgang"
	// LabelBasePodGang is a key for a label that sets the base PodGang name for scaled PodGangs.
	// This label is present on scaled PodGangs (beyond MinAvailable) and points to their base PodGang.
	LabelBasePodGang = "grove.io/base-podgang"
	// LabelPodGangSetReplicaIndex is a key for a label that sets the replica index of a PodGangSet.
	LabelPodGangSetReplicaIndex = "grove.io/podgangset-replica-index"
	// LabelPodCliqueScalingGroup is a key for a label that sets the PodCliqueScalingGroup name.
	LabelPodCliqueScalingGroup = "grove.io/podcliquescalinggroup"
	// LabelPodCliqueScalingGroupReplicaIndex is a key for a label that sets the replica index of a PCSG within PodGangSet.
	LabelPodCliqueScalingGroupReplicaIndex = "grove.io/podcliquescalinggroup-replica-index"
	// LabelComponentPGSPodCliqueValue is the value for LabelComponentKey for PodClique resources managed by a PodGangSet.
	LabelComponentPGSPodCliqueValue = "pgs-podclique"
	// LabelComponentPCSGPodCliqueValue is the value for LabelComponentKey for PodClique resources managed by a PodCliqueScalingGroup.
	LabelComponentPCSGPodCliqueValue = "pcsg-podclique"
)

// Constants for finalizers.
const (
	// FinalizerPodGangSet is the finalizer for PodGangSet that is added to `.metadata.finalizers[]` slice. This will be placed on all PodGangSet resources
	// during reconciliation. This finalizer is used to clean up resources that are created for a PodGangSet when it is deleted.
	FinalizerPodGangSet = "grove.io/podgangset.grove.io"
	// FinalizerPodClique is the finalizer for PodClique that is added to `.metadata.finalizers[]` slice. This will be placed on all PodClique resources
	// during reconciliation. This finalizer is used to clean up resources that are created for a PodClique when it is deleted.
	FinalizerPodClique = "grove.io/podclique.grove.io"
	// FinalizerPodCliqueScalingGroup is the finalizer for PodCliqueScalingGroup that is added to `.metadata.finalizers[]` slice.
	// This will be placed on all PodCliqueScalingGroup resources during reconciliation. This finalizer is used to clean up resources
	// that are created for a PodCliqueScalingGroup when it is deleted.
	FinalizerPodCliqueScalingGroup = "grove.io/podcliquescalinggroup.grove.io"
)

// Constants for events.
const (
	// EventReconciling is the event type which indicates that the reconcile operation has started.
	EventReconciling = "Reconciling"
	// EventReconciled is the event type which indicates that the reconcile operation has completed successfully.
	EventReconciled = "Reconciled"
	// EventReconcileError is the event type which indicates that the reconcile operation has failed.
	EventReconcileError = "ReconcileError"
	// EventDeleting is the event type which indicates that the delete operation has started.
	EventDeleting = "Deleting"
	// EventDeleted is the event type which indicates that the delete operation has completed successfully.
	EventDeleted = "Deleted"
	// EventDeleteError is the event type which indicates that the delete operation has failed.
	EventDeleteError = "DeleteError"
)

// Constants for Grove environment variables
const (
	// EnvVarPGSName is the environment variable name for PodGangSet name
	EnvVarPGSName = "GROVE_PGS_NAME"
	// EnvVarPGSIndex is the environment variable name for PodGangSet replica index
	EnvVarPGSIndex = "GROVE_PGS_INDEX"
	// EnvVarPCLQName is the environment variable name for PodClique name
	EnvVarPCLQName = "GROVE_PCLQ_NAME"
	// EnvVarHeadlessService is the environment variable name for headless service address
	EnvVarHeadlessService = "GROVE_HEADLESS_SERVICE"
	// EnvVarPodIndex is the environment variable name for pod index within PodClique
	EnvVarPodIndex = "GROVE_PCLQ_POD_INDEX"
	// EnvVarPCSGName is the environment variable name for PodCliqueScalingGroup name
	EnvVarPCSGName = "GROVE_PCSG_NAME"
	// EnvVarPCSGIndex is the environment variable name for PodCliqueScalingGroup replica index
	EnvVarPCSGIndex = "GROVE_PCSG_INDEX"
	// EnvVarPCSGTemplateNumPods is the environment variable name for total number of pods in PCSG template
	EnvVarPCSGTemplateNumPods = "GROVE_PCSG_TEMPLATE_NUM_PODS"
)

// Constants for Condition Types
const (
	// ConditionTypeMinAvailableBreached indicates that the minimum number of ready pods in the PodClique are below the threshold defined in the PodCliqueSpec.MinAvailable threshold.
	ConditionTypeMinAvailableBreached = "MinAvailableBreached"
	// ConditionTypePodCliqueScheduled indicates that the PodClique has been successfully scheduled.
	// This condition is set to true when number of scheduled pods in the PodClique is greater than or equal to PodCliqueSpec.MinAvailable.
	ConditionTypePodCliqueScheduled = "PodCliqueScheduled"
)

// Constants for Condition Reasons
const (
	// ConditionReasonInsufficientReadyPods indicates that the number of ready pods in the PodClique is below the threshold defined in the PodCliqueSpec.MinAvailable threshold.
	ConditionReasonInsufficientReadyPods = "InsufficientReadyPods"
	// ConditionReasonSufficientReadyPods indicates that the number of ready pods in the PodClique is above the threshold defined in the PodCliqueSpec.MinAvailable threshold.
	ConditionReasonSufficientReadyPods = "SufficientReadyPods"
	// ConditionReasonInsufficientScheduledPods indicates that the number of scheduled pods in the PodClique is below the threshold defined in the PodCliqueSpec.MinAvailable threshold.
	ConditionReasonInsufficientScheduledPods = "InsufficientScheduledPods"
	// ConditionReasonSufficientScheduledPods indicates that the number of scheduled pods in the PodClique greater or equal to PodCliqueSpec.MinAvailable.
	ConditionReasonSufficientScheduledPods = "SufficientScheduledPods"
	// ConditionReasonInsufficientScheduledPCSGReplicas indicates that the number of scheduled replicas in the PodCliqueScalingGroup is below the PodCliqueScalingGroupSpec.MinAvailable.
	ConditionReasonInsufficientScheduledPCSGReplicas = "InsufficientScheduledPodCliqueScalingGroupReplicas"
	// ConditionReasonInsufficientAvailablePCSGReplicas indicates that the number of ready replicas in the PodCliqueScalingGroup is below the PodCliqueScalingGroupSpec.MinAvailable.
	ConditionReasonInsufficientAvailablePCSGReplicas = "InsufficientAvailablePodCliqueScalingGroupReplicas"
	// ConditionReasonSufficientAvailablePCSGReplicas indicates that the number of ready replicas in the PodCliqueScalingGroup is greater than or equal to the PodCliqueScalingGroupSpec.MinAvailable.
	ConditionReasonSufficientAvailablePCSGReplicas = "SufficientAvailablePodCliqueScalingGroupReplicas"
)
