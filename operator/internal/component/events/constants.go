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

package events

// constants used for pod lifecycle events
const (
	// ReasonPodCreationSuccessful is an event reason which represents a successful creation of a Pod.
	ReasonPodCreationSuccessful = "PodCreationSuccessful"
	// ReasonPodCreationFailed is an event reason which represents that the creation of a Pod failed.
	ReasonPodCreationFailed = "PodCreationFailed"
	// ReasonPodDeletionSuccessful is an event reason which represents a successful deletion of a Pod.
	ReasonPodDeletionSuccessful = "PodDeletionSuccessful"
	// ReasonPodDeletionFailed is an event reason which represents that the deletion of a Pod failed.
	ReasonPodDeletionFailed = "PodDeletionFailed"
)

// constants for PodClique lifecycle events
const (
	// ReasonPodCliqueCreationSuccessful is an event reason which represents a successful creation of a PodClique.
	ReasonPodCliqueCreationSuccessful = "PodCliqueCreationSuccessful"
	// ReasonPodCliqueCreationFailed is an event reason which represents that the creation of a PodClique failed.
	ReasonPodCliqueCreationFailed = "PodCliqueCreationFailed"
	// ReasonPodCliqueDeletionSuccessful is an event reason which represents a successful deletion of a PodClique.
	ReasonPodCliqueDeletionSuccessful = "PodCliqueDeletionSuccessful"
	// ReasonPodCliqueDeletionFailed is an event reason which represents that the deletion of a PodClique failed.
	ReasonPodCliqueDeletionFailed = "PodCliqueDeletionFailed"
)

// constants for PodCliqueScalingGroup lifecycle events
const (
	// ReasonPodCliqueScalingGroupCreationSuccessful is an event which represents a successful creation of PodCliqueScalingGroup.
	ReasonPodCliqueScalingGroupCreationSuccessful = "PodCliqueScalingGroupCreationSuccessful"
	// ReasonPodCliqueScalingGroupCreationFailed is an event which represents that creation of PodCliqueScalingGroup failed.
	ReasonPodCliqueScalingGroupCreationFailed = "PodCliqueScalingGroupCreationFailed"
	// ReasonPodCliqueScalingGroupDeletionSuccessful is an event reason which represents a successful deletion of PodCliqueScalingGroup.
	ReasonPodCliqueScalingGroupDeletionSuccessful = "PodCliqueScalingGroupDeletionSuccessful"
	// ReasonPodCliqueScalingGroupDeletionFailed is an event which represents that the deletion of PodCliqueScalingGroup failed.
	ReasonPodCliqueScalingGroupDeletionFailed = "PodCliqueScalingGroupDeletionFailed"
	// ReasonPodCliqueScalingGroupReplicaDeletionSuccessful is an event reason which represents a successful deletion of a PodCliqueScalingGroup replica.
	ReasonPodCliqueScalingGroupReplicaDeletionSuccessful = "PodCliqueScalingGroupReplicaDeletionSuccessful"
	// ReasonPodCliqueScalingGroupReplicaDeletionFailed is an event which represents that the deletion of a replica of PodCliqueScalingGroup failed.
	ReasonPodCliqueScalingGroupReplicaDeletionFailed = "PodCliqueScalingGroupReplicaDeletionFailed"
)

// constants for PodGang lifecycle events
const (
	// ReasonPodGangCreationOrUpdationSuccessful is an event reason which represents a successful creation or updating of a PodGang.
	ReasonPodGangCreationOrUpdationSuccessful = "PodGangCreationOrUpdateSuccessful"
	// ReasonPodGangCreationOrUpdationFailed is an event reason which represents that the creation or updating of PodGang failed.
	ReasonPodGangCreationOrUpdationFailed = "PodGangCreationOrUpdateFailed"
	// ReasonPodGangDeletionSuccessful is an event reason which represents a successful deletion of a PodGang.
	ReasonPodGangDeletionSuccessful = "PodGangDeletionSuccessful"
	// ReasonPodGangDeletionFailed is an event reason which represents that the deletion of a PodGang failed.
	ReasonPodGangDeletionFailed = "PodGangDeletionFailed"
)

// constants for PodGangSet lifecycle events
const (
	// ReasonPodGangSetReplicaDeletionSuccessful is an event reason which represents a successful deletion of a PodGangSet replica.
	ReasonPodGangSetReplicaDeletionSuccessful = "PodGangSetReplicaDeletionSuccessful"
	// ReasonPodGangSetReplicaDeletionFailed is an event reason which represents that the deletion of a PodGangSet replica failed.
	ReasonPodGangSetReplicaDeletionFailed = "PodGangSetReplicaDeletionFailed"
)
