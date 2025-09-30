# Topology-Aware Scheduling - Grove Operator Design (Phase 0)

## Overview

This document defines the Phase 0 (P0) design for implementing topology-aware scheduling in the Grove operator.

**Motivation for Grove**: Topology-aware scheduling is critical for Grove's multinode inference workloads because these
applications require:

- **Network Locality**: High-bandwidth communication between prefill and decode workers benefits from proximity
- **Coordinated Placement**: Related components (e.g., model shards) perform better when co-located within the same
  topology domain
- **Latency Optimization**: Minimizing network hops between interdependent inference components improves end-to-end
  performance

## KAI Scheduler Integration Context

Grove integrates with KAI scheduler, which currently provides the only production-ready implementation of topology-aware
scheduling in Kubernetes. This integration introduces several architectural constraints that shape Grove's design:

- **Kueue Topology CRD Dependency**: KAI relies on Kueue's topology Custom Resource Definition (CRD) to represent
  cluster topology hierarchy. Grove must use this representation in P0, though future phases will decouple for more
  flexible topology representation.

- **Two-Level Topology Hierarchy**: KAI scheduler currently supports exactly two levels - global and group-level
  constraints. Grove's design must align with this limitation until future scheduler capabilities expand.

- **Explicit Requirements**: KAI scheduler requires explicit topology grouping requirements from clients - it cannot
  infer topology needs, so Grove must provide structured topology data through the Grove Scheduler API.

## Requirements

This design addresses the following Phase 0 (P0) requirements for topology-aware scheduling:

### User Experience Requirements

- **Non-Power Users**: Should get automatic global pgs replica topology optimization with no configuration needed when
  admin configures default topology
- **Power Users**: Should be able to specify topology constraints for groups of PodCliques and entire PodCliques as
  global constrains using required constraints
  only (P0)
- **Configuration Consistency**: If topology is not configured, no topology features should be applied regardless of
  user specifications

### Technical Requirements (KAI Scheduler Constraints)

- **Two-Level Hierarchy**: Support global topology constraints plus group-level constraints
    * *required due to KAI scheduler's current two-level limitation*
- **Explicit Requirements**: Grove operator must provide explicit topology grouping requirements to KAI scheduler
  through Grove Scheduler API - *required because KAI cannot infer topology needs*
- **Kueue Topology CRD**: Use Kueue topology CRD for topology hierarchy representation - *required dependency because
  KAI scheduler uses Kueue topology CRD*

### Technical Requirements (Grove Design Choices)

- **Constraint Types**: Support both required and preferred topology constraints (P0: users specify required only,
  system auto-generates preferred)
- **Immutability**: All topology configuration must be immutable after resource creation
- **Admin Configuration**: Admin should be able to configure default topology for the grove operator
- **Automatic Population**: Mutation webhook should autopopulate admin defaults if not provided by user
- **Automatic Global Constraints**: Grove operator should automatically add topology constraints to group all PodCliques
  closely together as mush as possible, using lowest available topology level as preferred constraint
  **Group Constraints**: Grove operator should allow power users to specify additional topology constraints:
    - for specific PodClique groups
    - entire PodClique's in a PodGangSet replica
- **Constraint Consistency**: Validate topology constraint consistency and uniqueness rules when topology is configured
- **Conditional Validation**: Ensure topology constraints are only specified when topology name is configured
- **Topology Deletion Prevention**: Prevent deletion of topology CRD when any PodGangSet references it using finalizers
    - topology CRD is immutable after creation so no need to handle updates

## Out of Scope (Future Phases)

The following features are explicitly out of scope for Phase 0 and will be considered in future phases:

- **Status Handling**: Status reporting, error conditions, and reconcile error handling will be designed and implemented
  in later phases
- **Workload-Based Auto Constraints**: Automatic constraint generation based on workload characteristics, patterns, and
  inference requirements
- **Enhanced Error Handling**: Advanced topology constraint conflict resolution, recovery mechanisms, and graceful
  degradation

## Architecture Context

### Topology CRD Overview

**What Topology CRD Defines**: The Kueue topology CRD represents the physical and logical hierarchy of Kubernetes
cluster infrastructure. It defines how nodes are organized across different topology domains such as racks, zones, and
regions, providing schedulers with explicit knowledge of the cluster's network and hardware layout.

**Topology Level Hierarchy**: Topology levels are organized from highest (most broad) to lowset (most granular),
for example:

- **Region Level** (`topology.kubernetes.io/region`) - Geographic or datacenter-level groupings
- **Zone Level** (`topology.kubernetes.io/zone`) - Availability zones within regions
- **Rack Level** (`topology.kubernetes.io/rack`) - Groups of nodes sharing network infrastructure
- **Node Level** (`kubernetes.io/hostname`) - Individual machines/nodes

#### example:

```yaml
apiVersion: kueue.x-k8s.io/v1alpha1
kind: Topology
metadata:
  name: "default"
spec:
  levels:
    - nodeLabel: "cloud.provider.com/topology-block"
    - nodeLabel: "cloud.provider.com/topology-rack"
    - nodeLabel: "kubernetes.io/hostname"
```

for more details see:

- [Kueue Topology concepts](https://kueue.sigs.k8s.io/docs/concepts/topology_aware_scheduling/)
- [kueue topology CRD](https://github.com/kubernetes-sigs/kueue/blob/main/apis/kueue/v1alpha1/topology_types.go)

## Component Architecture

### Non-Power User Flow (Automatic)

```
┌─────────────────┐    ┌──────────────────┐    ┌───────────────────┐
│ User Creates    │    │ Mutation Webhook │    │ Grove Operator    │
│ PodGangSet      │───▶│ Auto-populates   │───▶│ Detects Lowest    │
│ (no topology)   │    │ Admin Default    │    │ Topology Level    │
└─────────────────┘    │ Topology Name    │    └─────────┬─────────┘
                       └──────────────────┘              │
                                                         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                Grove Scheduler API (PodGang)                            │
│ • GroupingConstraints: Global topology (preferred, lowest level)        │
│ • SpreadConstraints: Kubernetes-native spreading                        │
│ • NetworkPackGroupConfigs: Basic network optimization                   │
└─────────────────────────────────────────────────────────────────────────┘
```

### Power User Flow (Explicit)

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│ User Creates    │    │ Validation       │    │ Grove Operator      │
│ PodGangSet with │───▶│ Webhook Checks   │───▶│ Processes Global    │
│ Group Topology  │    │ Constraints      │    │ + User Group        │
│ Constraints     │    │                  │    │ Constraints         │
└─────────────────┘    └──────────────────┘    └─────────┬───────────┘
                                                         │
                                                         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                Grove Scheduler API (PodGang)                            │
│ • GroupingConstraints: Global topology (automatic, lowest level)        │
│ • NetworkPackGroupConfigs: User-defined group-level constraints         │
│ • SpreadConstraints: Kubernetes-native spreading                        │
└─────────────────────────────────────────────────────────────────────────┘
```

### Topology-Aware Scheduling Flow

The complete flow follows these high-level steps:

1. **Auto-Population**: Mutation webhook auto-populates admin topology defaults when user doesn't specify topology
   configuration
2. **Validation**: System validates topology CRD exists and constraints are consistent
3. **Translation**: Operator translates topology constraints from user API to scheduler API format:
    - User-specified required constraints pass through directly
    - System auto-generates preferred constraints using lowest topology level
    - PodClique template names map to created PodClique's and are added as PodGroup names in PodGang with topology
      constraints
4. **Scheduling**: scheduler applies topology constraints using topology CR
5. **Finalizer Management**: Manage topology CRD finalizers to prevent deletion (see detailed sequence diagram below)

#### Main Sequence Diagram

```
┌──────────────┐    ┌──────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ PodGangSet   │    │  Grove Operator  │    │ Grove Scheduler │    │  KAI Scheduler  │
│              │    │                  │    │      API        │    │                 │
└──────┬───────┘    └─────────┬────────┘    └────────┬────────┘    └────────┬────────┘
       │                      │                      │                      │
       │ CREATE/UPDATE        │                      │                      │
       ├─────────────────────▶│                      │                      │
       │                      │                      │                      │
       │                      │ 1. Mutation webhook  │                      │
       │                      │    auto-populates    │                      │
       │                      │    admin topology    │                      │
       │                      │                      │                      │
       │                      │ 2. Validate topology │                      │
       │                      │    CRD exists        │                      │
       │                      │                      │                      │
       │                      │ 3. Manage finalizers │                      │
       │                      │    (see detailed     │                      │
       │                      │     diagram below)   │                      │
       │                      │                      │                      │
       │                      │ 4. Translate topology                       │
       │                      │    constraints:      │                      │
       │                      │    - User required   │                      │
       │                      │    - Auto preferred  │                      │
       │                      │    - PodClique→Group │                      │
       │                      │                      │                      │
       │                      │ CREATE/UPDATE PodGang│                      │
       │                      ├─────────────────────▶│                      │
       │                      │                      │                      │
       │                      │                      │ SCHEDULE Pods        │
       │                      │                      ├─────────────────────▶│
       │                      │                      │                      │
       │                      │                      │                      │ 5. Apply topology
       │                      │                      │                      │    constraints
       │                      │                      │                      │    using Kueue
       │                      │                      │                      │    topology CRD
       │                      │                      │                      │
```

## API Design

### Grove Operator API Changes

#### PodGangSet CRD Extensions

##### SchedulingPolicyConfig Changes

```go
type SchedulingPolicyConfig struct {
// TopologyName specifies the topology CRD name for topology-aware scheduling
// When nil, no topology features are applied (fallback to standard scheduling)
// This field is immutable after resource creation
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="topology name is immutable"
TopologyName *string `json:"topologyName,omitempty"`
// GroupingConstraints provides explicit topology grouping requirements for all PodCliques in the PodGangSet replica
// This field is immutable after resource creation
// +kubebuilder:validation:XValidation:rule="self==oldSelf",message="topology grouping constraints are immutable"
GroupingConstraints *TopologyConstraints `json:"groupingConstraints,omitempty"`

// NetworkPackGroupConfigs defines groups of PodCliques for network optimization
// Now supports optional topology constraints when topology is configured
NetworkPackGroupConfigs []NetworkPackGroupConfig `json:"networkPackGroupConfigs,omitempty"`
}
```

##### Enhanced NetworkPackGroupConfig

```go
// NetworkPackGroupConfig indicates that PodCliques should be optimally placed w.r.t network topology
// Now supports optional topology constraints for group-level topology requirements
type NetworkPackGroupConfig struct {
// CliqueNames is the list of PodClique names that are part of the network pack group
CliqueNames []string `json:"cliqueNames"`

// TopologyConstraint specifies optional topology requirements for this group
// When specified, all PodCliques in this group must satisfy the topology constraint
// Only available when TopologyName is configured
TopologyConstraints *TopologyConstraints `json:"topologyConstraint,omitempty"`
}


type TopologyConstraints struct {
// Required defines topology placement requirements (user-specified)
Required *TopologyConstraint `json:"required,omitempty"`
}

// TopologyConstraint defines topology placement requirements for a group of PodCliques
type TopologyConstraint struct {
// TopologyLevelKey is the topology domain key that must match the levels defined
// in the topology CRD (e.g., "kubernetes.io/zone", "kubernetes.io/hostname")
TopologyLevelKey string `json:"topologyKey"`
}
```

**Note**: Each component will have a maximum of two constraints - one preferred and one required, so a single
TopologyConstraint field is sufficient per group.

### Grove Scheduler API Changes

The Grove Operator translates user-defined topology configuration into Grove Scheduler API format for consumption by KAI
scheduler.

#### Enhanced PodGang Structure

```go
type PodGangSpec struct {
// PodGroups is a list of member pod groups in the PodGang
PodGroups []PodGroup `json:"podgroups"`

// NetworkPackGroupConfigs with topology constraints for group-level requirements
NetworkPackGroupConfigs []NetworkPackGroupConfig `json:"networkPackGroupConfigs,omitempty"`

// GroupingConstraints provides explicit topology grouping requirements for entire pod gang
// This gives the scheduler explicit pod gang level topology grouping constraints
GroupingConstraints *TopologyConstraints `json:"topologyGrouping,omitempty"`

// SpreadConstraints handles Kubernetes-native topology spreading (different purpose)
SpreadConstraints []corev1.TopologySpreadConstraint `json:"spreadConstraints,omitempty"`

// PriorityClassName is the name of the PriorityClass for the PodGang
PriorityClassName string `json:"priorityClassName,omitempty"`
}
```

#### Enhanced NetworkPackGroupConfig

```go
// NetworkPackGroupConfig indicates that PodGroups should be optimally placed w.r.t network topology
// Now supports direct topology constraints for group-level requirements
type NetworkPackGroupConfig struct {
// PodGroupNames is the list of PodGroup names that are part of the network pack group
PodGroupNames []string `json:"podGroupNames"`

// TopologyConstraints specifies topology requirements for this group
// When specified, all PodGroups in this group must satisfy the topology constraint
TopologyConstraints *TopologyConstraints `json:"topologyConstraints,omitempty"`
}
```

#### TopologyConstraints Structure

```go

// TopologyConstraints defines global topology distribution requirements
type TopologyConstraints struct {
// Required topology constraint that must be satisfied (user-specified)
Required *TopologyConstraint `json:"required,omitempty"`

// Preferred topology constraint for best-effort distribution (system auto-generated)
Preferred *TopologyConstraint `json:"preferred,omitempty"`
}

// TopologyConstraint defines topology placement requirements
type TopologyConstraint struct {
// TopologyLevelKey is the topology domain key that must match the levels defined
// in the Kueue topology CRD (e.g., "kubernetes.io/zone", "kubernetes.io/hostname")
TopologyLevelKey string `json:"topologyKey"`
}
```

## Topology Deletion Prevention

### Why Topology Deletion Prevention is Critical

Topology CRDs define the cluster's physical layout that Grove uses for optimal pod placement. Accidental deletion of a
topology CRD while PodGangSets reference it would cause:

- **Scheduling Failures**: New pods cannot be scheduled as the topology information is missing
- **Operational Confusion**: Administrators may not understand why scheduling suddenly fails

### Deletion Prevention Flow

Grove uses a stateless approach to prevent topology CRD deletion when PodGangSets reference them:

- **Finalizer Addition**: When any PodGangSet references a topology CRD, Grove adds a finalizer to the topology resource
- **Deletion Blocking**: When topology deletion is attempted, Grove scans all PodGangSets in the cluster to check for
  references, and blocks deletion
- **Cleanup Process**: Finalizers are removed when no PodGangSets reference the topology CRD (determined by cluster
  scan)

### Implementation Pattern

```yaml
# Topology with Grove finalizer
apiVersion: kueue.x-k8s.io/v1alpha1
kind: Topology
metadata:
  name: "default"
  finalizers:
    - "grove.ai/topology-protection"
```

### Topology Deletion Prevention Sequence Diagram

```
┌──────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Admin/User   │    │  Grove Operator  │    │ Topology CRD    │
│              │    │                  │    │                 │
└──────┬───────┘    └─────────┬────────┘    └─────────┬───────┘
       │                      │                        │
       │ DELETE topology      │                        │
       │ "default"            │                        │
       ├─────────────────────▶│                        │
       │                      │                        │
       │                      │ Deletion webhook       │
       │                      │ triggered              │
       │                      │                        │
       │                      │ Scan all PodGangSets   │
       │                      │ in cluster for refs    │
       │                      │ to "default"           │
       │                      │                        │
       │                      │ Found references?      │
       │                      │                        │
       │                      │ YES: Keep finalizer    │
       │                      │ Deletion blocked       │
       │                      │                        │
       │ Deletion blocked     │                        │
       │ (finalizer present)  │                        │
       │◀─────────────────────┤                        │
       │                      │                        │
       │                      │ NO: Remove finalizer   │
       │                      │                        │
       │                      ├───────────────────────▶│
       │                      │                        │
       │ Topology deleted     │                        │
       │◀─────────────────────┤                        │
       │                      │                        │
```

## Topology Configuration

### Admin Configuration

Administrators configure the default topology name in the operator deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grove-operator
spec:
  template:
    spec:
      containers:
        - name: operator
          args:
            - --default-topology-name=default-topology  # Kueue topology CRD name
```

## Validation Rules

### Validation Logic

The validation webhook ensures topology configuration consistency through the following checks:

### Uniqueness Constraints

- Each `CliqueNames` entry can only appear in one `NetworkPackGroupConfig`
- `TopologyName` must filled when any `TopologyConstraint` is specified either globally or in a group

### Immutability Constraints

- `TopologyName` field is immutable after resource creation
- All `TopologyConstraint` fields are immutable after resource creation
- `NetworkPackGroupConfigs` structure is immutable after resource creation

## Mutation Webhook Rules

### Admin Default Auto-Population

The mutation webhook handles automatic topology configuration:

- Automatically populates topology name from admin default when user hasn't specified one
- Only applies to NEW PodGangSet resources
- Preserves user-specified topology names

### Admin Default Change Handling

When admin changes the default topology name in operator deployment:

- Existing PodGangSet resources are NOT updated
- Only NEW PodGangSet resources receive the new default
- Prevents unexpected modifications to running workloads

