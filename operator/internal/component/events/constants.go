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
	// ReasonPodCreateSuccessful is an event reason which represents a successful creation of a Pod.
	ReasonPodCreateSuccessful = "PodCreateSuccessful"
	// ReasonPodCreateFailed is an event reason which represents that the creation of a Pod failed.
	ReasonPodCreateFailed = "PodCreateFailed"
	// ReasonPodDeleteSuccessful is an event reason which represents a successful deletion of a Pod.
	ReasonPodDeleteSuccessful = "PodDeleteSuccessful"
	// ReasonPodDeleteFailed is an event reason which represents that the deletion of a Pod failed.
	ReasonPodDeleteFailed = "PodDeleteFailed"
)

// constants for PodClique lifecycle events
const (
	// ReasonPodCliqueCreateSuccessful is an event reason which represents a successful creation of a PodClique.
	ReasonPodCliqueCreateSuccessful = "PodCliqueCreateSuccessful"
	// ReasonPodCliqueCreateOrUpdateSuccessful is an event reason which represents a successful creation or updation of a PodClique.
	ReasonPodCliqueCreateOrUpdateSuccessful = "PodCliqueCreateOrUpdateSuccessful"
	// ReasonPodCliqueCreateFailed is an event reason which represents that the creation of a PodClique failed.
	ReasonPodCliqueCreateFailed = "PodCliqueCreateFailed"
	// ReasonPodCliqueCreateOrUpdateFailed is an event reason which represents that the creation or updation of a PodClique failed.
	ReasonPodCliqueCreateOrUpdateFailed = "PodCliqueCreateOrUpdateFailed"
	// ReasonPodCliqueDeleteSuccessful is an event reason which represents a successful deletion of a PodClique.
	ReasonPodCliqueDeleteSuccessful = "PodCliqueDeleteSuccessful"
	// ReasonPodCliqueDeleteFailed is an event reason which represents that the deletion of a PodClique failed.
	ReasonPodCliqueDeleteFailed = "PodCliqueDeleteFailed"
)

// constants for PodCliqueScalingGroup lifecycle events
const (
	// ReasonPodCliqueScalingGroupCreateSuccessful is an event which represents a successful creation of PodCliqueScalingGroup.
	ReasonPodCliqueScalingGroupCreateSuccessful = "PodCliqueScalingGroupCreateSuccessful"
	// ReasonPodCliqueScalingGroupCreateOrUpdateFailed is an event which represents that create or update of PodCliqueScalingGroup failed.
	ReasonPodCliqueScalingGroupCreateOrUpdateFailed = "PodCliqueScalingGroupCreateOrUpdateFailed"
	// ReasonPodCliqueScalingGroupDeleteSuccessful is an event reason which represents a successful deletion of PodCliqueScalingGroup.
	ReasonPodCliqueScalingGroupDeleteSuccessful = "PodCliqueScalingGroupDeleteSuccessful"
	// ReasonPodCliqueScalingGroupDeleteFailed is an event which represents that the deletion of PodCliqueScalingGroup failed.
	ReasonPodCliqueScalingGroupDeleteFailed = "PodCliqueScalingGroupDeleteFailed"
	// ReasonPodCliqueScalingGroupReplicaDeleteSuccessful is an event reason which represents a successful deletion of a PodCliqueScalingGroup replica.
	ReasonPodCliqueScalingGroupReplicaDeleteSuccessful = "PodCliqueScalingGroupReplicaDeleteSuccessful"
	// ReasonPodCliqueScalingGroupReplicaDeleteFailed is an event which represents that the deletion of a replica of PodCliqueScalingGroup failed.
	ReasonPodCliqueScalingGroupReplicaDeleteFailed = "PodCliqueScalingGroupReplicaDeleteFailed"
)

// constants for PodGang lifecycle events
const (
	// ReasonPodGangCreateOrUpdateSuccessful is an event reason which represents a successful creation or updating of a PodGang.
	ReasonPodGangCreateOrUpdateSuccessful = "PodGangCreateOrUpdateSuccessful"
	// ReasonPodGangCreateOrUpdateFailed is an event reason which represents that the creation or updating of PodGang failed.
	ReasonPodGangCreateOrUpdateFailed = "PodGangCreateOrUpdateFailed"
	// ReasonPodGangDeleteSuccessful is an event reason which represents a successful deletion of a PodGang.
	ReasonPodGangDeleteSuccessful = "PodGangDeleteSuccessful"
	// ReasonPodGangDeleteFailed is an event reason which represents that the deletion of a PodGang failed.
	ReasonPodGangDeleteFailed = "PodGangDeleteFailed"
)

// constants for PodCliqueSet lifecycle events
const (
	// ReasonPodCliqueSetReplicaDeleteSuccessful is an event reason which represents a successful deletion of a PodCliqueSet replica.
	ReasonPodCliqueSetReplicaDeleteSuccessful = "PodCliqueSetReplicaDeleteSuccessful"
	// ReasonPodCliqueSetReplicaDeleteFailed is an event reason which represents that the deletion of a PodCliqueSet replica failed.
	ReasonPodCliqueSetReplicaDeleteFailed = "PodCliqueSetReplicaDeleteFailed"
)
