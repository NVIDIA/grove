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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName={td}

// TopologyDomain defines the topology hierarchy for the cluster.
// This resource is immutable after creation.
// Only one TopologyDomain can exist cluster-wide (enforced by webhook).
type TopologyDomain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec defines the topology hierarchy specification.
	Spec TopologyDomainSpec `json:"spec"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TopologyDomainList is a list of TopologyDomain resources.
type TopologyDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TopologyDomain `json:"items"`
}

// TopologyDomainSpec defines the topology hierarchy specification.
type TopologyDomainSpec struct {
	// Levels is an ordered list of topology levels from broadest to narrowest scope.
	// The order in this list defines the hierarchy (index 0 = broadest level).
	// This field is immutable after creation.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="levels list is immutable"
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=7
	Levels []TopologyLevel `json:"levels"`
}

// TopologyLevelName represents a predefined topology level in the hierarchy.
// Topology ordering (broadest to narrowest):
// Region > Zone > DataCenter > Block > Rack > Host > Numa
type TopologyLevelName string

const (
	// TopologyLevelRegion represents the region level in the topology hierarchy.
	TopologyLevelRegion TopologyLevelName = "region"
	// TopologyLevelZone represents the zone level in the topology hierarchy.
	TopologyLevelZone TopologyLevelName = "zone"
	// TopologyLevelDataCenter represents the datacenter level in the topology hierarchy.
	TopologyLevelDataCenter TopologyLevelName = "datacenter"
	// TopologyLevelBlock represents the block level in the topology hierarchy.
	TopologyLevelBlock TopologyLevelName = "block"
	// TopologyLevelRack represents the rack level in the topology hierarchy.
	TopologyLevelRack TopologyLevelName = "rack"
	// TopologyLevelHost represents the host level in the topology hierarchy.
	TopologyLevelHost TopologyLevelName = "host"
	// TopologyLevelNuma represents the numa level in the topology hierarchy.
	TopologyLevelNuma TopologyLevelName = "numa"
)

// TopologyLevel defines a single level in the topology hierarchy.
type TopologyLevel struct {
	// Name is the predefined level identifier used in TopologyConstraint references.
	// Must be one of: region, zone, datacenter, block, rack, host, numa
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=region;zone;datacenter;block;rack;host;numa
	Name TopologyLevelName `json:"name"`

	// TopologyKey is the node label key that identifies this topology domain.
	// Must be a valid Kubernetes label key (qualified name).
	// Examples: "topology.kubernetes.io/zone", "kubernetes.io/hostname"
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=316
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
	TopologyKey string `json:"topologyKey"`
}
