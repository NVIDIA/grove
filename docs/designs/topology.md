# Topology-Aware Scheduling - Grove Operator Design

## Overview

This document defines the design for supporting topology-aware scheduling in the Grove operator.

**Motivation**: Topology-aware scheduling is critical for Grove's multi-node inference workloads because these
applications require:

- **Network Locality**: Proximity improves high-bandwidth communication between leaders and their respective workers
- **Coordinated Placement**: Related components (e.g., model shards) perform better when co-located within the same
  topology domain
- **Latency Optimization**: Minimizing network hops between interdependent inference components improves end-to-end
  performance

## Goals

- Provide flexible, cluster-agnostic topology hierarchy definition via TopologyDomain CRD
- Enable packing constraints for network locality across all Grove scalable resources
- Enforce singleton topology for cluster-wide consistency
- Immutable topology configuration ensuring scheduling consistency
- Hierarchical constraint validation (child stricter than parent)

## Non-Goals

- Spread constraints across topology domains (ReplicaSpreadDomain)
- Root domain constraints for entire resource (RootDomain)
- Ratio-based affinity groups between scaling groups (AffinityGroups with PackRatio)
- Dynamic topology reconfiguration after creation
- Automatic suggest topology according to workload characteristics

## Proposal

Grove implements topology-aware scheduling through a singleton TopologyDomain CRD,
operator configuration to enable/disable features, and user-specified TopologyConstraints in workloads.
The operator automatically generates preferred constraints (lower bound) for optimization
while allowing users to specify required constraints for strict placement (upper bound).

## Design Details

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Topology Architecture                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Admin Layer:                                                           │
│  ┌──────────────────────┐          ┌──────────────────────┐            │
│  │ TopologyDomain       │          │ Kueue Topology       │            │
│  │ "grove-topology"     │          │ "grove-topology"     │            │
│  │ (singleton)          │          │ (manual creation)    │            │
│  └──────────┬───────────┘          └───────────┬──────────┘            │
│             │                                   │                       │
│             │                                   │                       │
│  Operator Config: OperatorConfiguration.EnableTopology=true            │
│             │                                   │                       │
│             │ (validates against)               │ (referenced by)       │
├─────────────┼───────────────────────────────────┼───────────────────────┤
│             │                                   │                       │
│  User Layer:                                    │                       │
│             ▼                                   │                       │
│  ┌──────────────────┐              ┌────────────────────┐               │
│  │ PodCliqueSet     │─────────────▶│ Grove Operator     │               │
│  │ (packDomain)     │              │ (reconciles)       │               │
│  └──────────────────┘              └─────────┬──────────┘               │
│                                              │                          │
│                                              │ (translates)             │
│                                              ▼                          │
│                                    ┌────────────────────┐               │
│                                    │ PodGang            │───────▶ KAI   │
│                                    │ • Annotation:      │     Scheduler │
│                                    │   topology-name    │               │
│                                    │ • 3-level topology │               │
│                                    │   (required+       │               │
│                                    │    preferred)      │               │
│                                    └────────────────────┘               │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 1. TopologyDomain Infrastructure

#### TopologyDomain CR

TopologyDomain is a cluster-scoped CR that defines the topology hierarchy for scheduling. It maps friendly level names
to Kubernetes node labels and establishes ordering from broadest to narrowest scope.

*note: this CR is independent of Kueue Topology CRD, which must be manually created by admin to align with Grove's
TopologyDomain for KAI scheduler usage.* (also named "grove-topology")
**Characteristics:**

- **Cluster-scoped singleton**: Only one TopologyDomain allowed cluster-wide, user chooses name
- **Default name**: "grove-topology" used when topologyDomainName not specified in operator config
- **Immutable**: Once created, cannot be modified
- **List-ordered hierarchy**: Index 0 = broadest (e.g., region), last = narrowest (e.g., host)
- **Predefined ordering**: Region > Zone > DataCenter > Block > SubBlock > Rack > Host > Numa (broadest to narrowest)
- **Webhook-validated**: Webhook enforces singleton constraint (any name allowed)

**TopologyLevelName Definitions:**

- **Region**: Network local to a CSP region
- **Zone**: Network local to a CSP availability-zone within a region
- **DataCenter**: Network local to a data-center within a CSP availability-zone
- **Block**: Network local to a switching block unit within a data-center
- **SubBlock**: Sub-switching block unit within a larger block
- **Rack**: First-level network grouping of compute hosts (includes NVLink domains as logical racks)
- **Host**: Individual compute host
- **Numa**: NUMA node (processor and memory locality domain) within a compute host

**API Structure:**

```go
// TopologyLevelName represents a predefined topology level in the hierarchy
type TopologyLevelName string

const (
TopologyLevelRegion     TopologyLevelName = "region"
TopologyLevelZone       TopologyLevelName = "zone"
TopologyLevelDataCenter TopologyLevelName = "datacenter"
TopologyLevelBlock      TopologyLevelName = "block"
TopologyLevelSubBlock   TopologyLevelName = "subblock"
TopologyLevelRack       TopologyLevelName = "rack"
TopologyLevelHost       TopologyLevelName = "host"
TopologyLevelNuma       TopologyLevelName = "numa"
)

// Topology ordering (broadest to narrowest):
// Region > Zone > DataCenter > Block > SubBlock > Rack > Host > Numa

// TopologyDomain defines the topology hierarchy for the cluster
// This resource is immutable after creation
// Only one TopologyDomain can exist cluster-wide with enforced name "grove-topology"
type TopologyDomain struct {
metav1.TypeMeta   `json:",inline"`
metav1.ObjectMeta `json:"metadata,omitempty"`

Spec TopologyDomainSpec `json:"spec,omitempty"`
}

type TopologyDomainSpec struct {
// Levels is an ordered list of topology levels from broadest to narrowest scope
// The order in this list defines the hierarchy (index 0 = highest level)
// This field is immutable after creation
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="levels list is immutable"
// +kubebuilder:validation:MinItems=1
// +kubebuilder:validation:MaxItems=8
Levels []TopologyLevel `json:"levels"`
}

type TopologyLevel struct {
// Name is the predefined level identifier used in TopologyConstraint references
// Must be one of: region, zone, datacenter, block, subblock, rack, host, numa
// +kubebuilder:validation:Required
// +kubebuilder:validation:Enum=region;zone;datacenter;block;subblock;rack;host;numa
Name TopologyLevelName `json:"name"`

// TopologyKey is the node label key that identifies this topology domain
// Must be a valid Kubernetes label key (qualified name)
// Examples: "topology.kubernetes.io/zone", "kubernetes.io/hostname"
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=316
// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
TopologyKey string `json:"topologyKey"`
}
```

**Example TopologyDomain:**

```yaml
apiVersion: grove.io/v1alpha1
kind: TopologyDomain
metadata:
   name: my-cluster-topology  # User chooses name
spec:
  levels:
    - name: region
      topologyKey: "topology.kubernetes.io/region"
    - name: zone
      topologyKey: "topology.kubernetes.io/zone"
    - name: datacenter
      topologyKey: "topology.kubernetes.io/datacenter"
    - name: block
      topologyKey: "topology.kubernetes.io/block"
    - name: subblock
      topologyKey: "topology.kubernetes.io/subblock"
    - name: rack
      topologyKey: "topology.kubernetes.io/rack"
    - name: host
      topologyKey: "kubernetes.io/hostname"
    - name: numa
      topologyKey: "topology.kubernetes.io/numa"
```

**Creating TopologyDomain:**

1. Customize example above with your cluster's actual `topologyKey` values
2. Choose a name for your topology:
   - Use custom name (e.g., "my-cluster-topology") OR
   - Use default name "grove-topology" (no config needed)
3. Create resource: `kubectl apply -f topologydomain.yaml`
4. If using custom name: configure operator with topology name in OperatorConfiguration
5. Manually create Kueue Topology with same name and aligned levels for KAI scheduler

**Validation:**

- Only one TopologyDomain allowed cluster-wide (webhook enforces singleton, any name allowed)
- Level names must be from predefined set: region, zone, datacenter, block, subblock, rack, host, numa (enum validation)
- Each level `name` and `topologyKey` must be unique
- Mutation webhook automatically reorders levels to match predefined ordering (Region > Zone > DataCenter > Block >
  SubBlock > Rack > Host > Numa)
- Admins can skip intermediate levels (e.g., define only region, rack, host)
- Immutable after creation (webhook blocks updates)
- Deletion protection via controller finalizer (blocks deletion while PodCliqueSet resources exist)

#### TopologyDomain Controller

The TopologyDomain controller manages the TopologyDomain resource lifecycle:

**Deletion Protection**

Prevents TopologyDomain deletion while any PodCliqueSet resources exist using Kubernetes finalizer.

Deletion Workflow:

1. Admin runs `kubectl delete topologydomain <name>`
2. Kubernetes blocks deletion (finalizer `grove.io/topologydomain` present)
3. Controller reconciles:
    - Detects deletion request (deletion timestamp set)
   - Scans cluster for any PodCliqueSet resources
   - If any PodCliqueSet exists: Keeps finalizer, deletion blocked
   - If no PodCliqueSet exists: Removes finalizer, deletion proceeds
4. Once finalizer removed, Kubernetes deletes TopologyDomain

Key Points:

- Admin must delete all PodCliqueSet resources before deleting TopologyDomain
- Controller checks if any PodCliqueSet exists (no need to check specific references)
- Since topology is singleton, any PodCliqueSet potentially uses it
- Controller continuously reconciles deletion requests
- Prevents orphaned workloads with invalid topology configuration

#### Operator Configuration

Operator enables/disables topology features via OperatorConfiguration manifest:

```yaml
apiVersion: grove.io/v1alpha1
kind: OperatorConfiguration
metadata:
  name: grove-operator-config
spec:
   topology:
      enabled: true
      topologyDomainName: "my-cluster-topology"  # Optional, defaults to "grove-topology"
```

**Startup Behavior:**

- If `topology.enabled: true`:
   - `topologyDomainName` not specified → defaults to "grove-topology"
   - Operator looks for TopologyDomain with configured name (defaults to "grove-topology")
   - If TopologyDomain with that name doesn't exist → operator fails to start
- If `topology.enabled: false`: topology features disabled
- Admin must create TopologyDomain with matching name OR disable topology

**Admin Responsibilities:**

- Manually create Kueue Topology with same name as Grove TopologyDomain for KAI scheduler
- Ensure topology levels align between Grove TopologyDomain and Kueue Topology

### 2. Operator API Changes (Grove CRDs)

#### TopologyConstraint Model

```go
type TopologyConstraint struct {
// PackLevel specifies the topology level name for grouping replicas
// Controls placement constraint for EACH individual replica instance
// Must be one of: region, zone, datacenter, block, subblock, rack, host, numa
// Example: "rack" means each replica independently placed within one rack
// Note: Does NOT constrain all replicas to the same rack together
// Different replicas can be in different topology domains
// +kubebuilder:validation:Enum=region;zone;datacenter;block;subblock;rack;host;numa
PackLevel *TopologyLevelName `json:"packLevel,omitempty"`
}
```

#### Fields Removed from Current API

**From PodCliqueSetSpec:**

- `ReplicaSpreadConstraints []corev1.TopologySpreadConstraint` - Removed (spread not supported)

**From PodCliqueSetTemplateSpec:**

- `SchedulingPolicyConfig *SchedulingPolicyConfig` - Removed (replaced by TopologyConstraint)

**Types Removed:**

- `SchedulingPolicyConfig` struct - Removed entirely
- `NetworkPackGroupConfig` struct - Removed entirely

#### PodCliqueSet CRD Extensions

```go
type PodCliqueSetTemplateSpec struct {
// ... existing fields ...

// TopologyConstraint defines topology placement requirements for PodCliqueSet
// +optional
TopologyConstraint *TopologyConstraint `json:"topologyConstraint,omitempty"`
}
```

#### PodCliqueScalingGroup CRD Extensions

```go
type PodCliqueScalingGroupConfig struct {
// ... existing fields ...

// TopologyConstraint defines topology placement requirements for PodCliqueScalingGroup
// Must be equal to or stricter than parent PodCliqueSet constraints
// +optional
TopologyConstraint *TopologyConstraint `json:"topologyConstraint,omitempty"`
}
```

#### PodClique CRD Extensions

```go
type PodCliqueTemplateSpec struct {
// ... existing fields ...

// TopologyConstraint defines topology placement requirements for PodClique
// Must be equal to or stricter than parent resource constraints
// +optional
TopologyConstraint *TopologyConstraint `json:"topologyConstraint,omitempty"`
}
```

#### Validation Webhook

**Hierarchy Constraints:**

- Child PackLevel must be equal to or stricter than parent (stricter = higher index in levels list)
- PodCliqueSet → PodCliqueScalingGroup → PodClique hierarchy
- Referenced PackLevel name must exist in TopologyDomain.Spec.Levels
- Validation applies on both CREATE and UPDATE operations

### 3. Scheduler API Changes (Contract with KAI)

#### PodGang CRD Extensions

The Grove Operator translates topology configuration into Grove Scheduler API format, which serves as the contract with
KAI scheduler.

**PodGangSpec:**

```go
type PodGangSpec struct {
// PodGroups is a list of member pod groups in the PodGang
PodGroups []PodGroup `json:"podgroups"`

// TopologyConstraint defines topology packing constraints for entire pod gang
// Translated from PodCliqueSet.TopologyConstraint
// Updated by operator on each reconciliation when PodCliqueSet topology constraints change
// +optional
TopologyConstraint *TopologyConstraint `json:"topologyConstraint,omitempty"`

// TopologyConstraintGroupConfigs defines groups of PodGroups for topology-aware placement
// Enhanced with topology constraints for PCSG-level packing
// Updated by operator on each reconciliation when PCSG topology constraints change
// +optional
TopologyConstraintGroupConfigs []TopologyConstraintGroupConfig `json:"topologyConstraintGroupConfigs,omitempty"`

// PriorityClassName is the name of the PriorityClass for the PodGang
PriorityClassName string `json:"priorityClassName,omitempty"`
}
```

**PodGang Metadata:**

The operator adds topology information to PodGang metadata via annotation:

```go
// Annotation added to PodGang
metadata:
annotations:
grove.run.ai/topology-name: "<user-configured-name>"
```

This annotation allows the scheduler to locate the Kueue Topology resource without requiring a spec field, providing
flexibility for future API changes.

**TopologyConstraintGroupConfig:**

```go
// TopologyConstraintGroupConfig defines topology constraints for a group of PodGroups
type TopologyConstraintGroupConfig struct {
// PodGroupNames is the list of PodGroup names in the topology constraint group
PodGroupNames []string `json:"podGroupNames"`

// TopologyConstraint defines topology packing constraints for this group
// Enables PCSG-level topology constraints
// Updated by operator when PodCliqueScalingGroup topology constraints change
// +optional
TopologyConstraint *TopologyConstraint `json:"topologyConstraint,omitempty"`
}
```

**PodGroup:**

```go
type PodGroup struct {
// Name is the name of the PodGroup
Name string `json:"name"`

// PodReferences is a list of references to the Pods in this group
PodReferences []NamespacedName `json:"podReferences"`

// MinReplicas is the number of replicas that needs to be gang scheduled
MinReplicas int32 `json:"minReplicas"`

// TopologyConstraint defines topology packing constraints for this PodGroup
// Enables PodClique-level topology constraints
// Updated by operator when PodClique topology constraints change
// +optional
TopologyConstraint *TopologyConstraint `json:"topologyConstraint,omitempty"`
}
```

**Supporting Types:**

```go
type TopologyConstraint struct {
// PackConstraint defines topology packing constraint with required and preferred levels
// Operator translates user's level name to corresponding topologyKeys
// +optional
PackConstraint *TopologyPackConstraint `json:"packConstraint,omitempty"`
}

type TopologyPackConstraint struct {
// Required defines topology constraint that must be satisfied
// Holds topologyKey (not level name) translated from user's packLevel specification
// Example: "topology.kubernetes.io/rack"
// +optional
Required *string `json:"required,omitempty"`

// Preferred defines best-effort topology constraint
// Auto-generated by operator using strictest level topologyKey for optimization
// Scheduler can fallback to less strict levels if preferred cannot be satisfied
// Example: "kubernetes.io/hostname"
// +optional
Preferred *string `json:"preferred,omitempty"`
}
```

**Changes Summary:**

Fields Added:

- `PodGangSpec.TopologyConstraint *TopologyConstraint` - PodGang-level packing from PodCliqueSet (optional pointer)
- `TopologyConstraintGroupConfig.TopologyConstraint *TopologyConstraint` - PCSG-level packing from
  PodCliqueScalingGroup (optional pointer)
- `PodGroup.TopologyConstraint *TopologyConstraint` - PodClique-level packing from PodClique (optional pointer)

Annotations Added:

- `grove.run.ai/topology-name: "<user-configured-name>"` - Annotation on PodGang metadata referencing topology name

Fields Removed:

- `PodGangSpec.SpreadConstraints` - Not implemented; spread will be part of TopologyConstraint in future

**Note:** All TopologyConstraint fields are pointers with omitempty, allowing workloads without topology constraints.

#### Translation Logic

The operator translates Grove operator API to Grove Scheduler API with three-level topology constraint hierarchy:

**Topology Annotation:**

- Operator adds annotation `grove.run.ai/topology-name: "<topology-name>"` to PodGang metadata
- Annotation value matches the TopologyDomain name from operator configuration
- KAI scheduler uses this annotation to locate the corresponding Kueue Topology CRD
- Annotation approach provides API flexibility for future changes without breaking spec

**Constraint Translation (Required and Preferred):**

The operator translates user's level names to topologyKeys and builds required/preferred structure:

**Required Constraints:**

- User specifies level name: `packLevel: "rack"`
- Operator looks up topologyKey from TopologyDomain: `"topology.kubernetes.io/rack"`
- Writes to PodGang: `TopologyConstraint.PackConstraint.Required = "topology.kubernetes.io/rack"`
- If user doesn't specify packLevel → `PackConstraint.Required` is nil

**Preferred Constraints (Auto-Generated):**

- Operator ALWAYS generates preferred constraint at all three levels
- Uses topologyKey of strictest level (e.g., `"kubernetes.io/hostname"` for "host" level)
- Writes to PodGang: `TopologyConstraint.PackConstraint.Preferred = "kubernetes.io/hostname"`
- Enables out-of-box optimization even without user configuration
- Scheduler can fallback to less strict levels if preferred cannot be satisfied

**Three-Level Translation:**

1. **PodGang Level** (from PodCliqueSet):
    - `PodGangSpec.TopologyConstraint.PackConstraint.Required` ← topologyKey looked up from user's level name (if set)
    - `PodGangSpec.TopologyConstraint.PackConstraint.Preferred` ← topologyKey of strictest level (e.g.,
     `"kubernetes.io/hostname"`)

2. **TopologyConstraintGroup Level** (from PodCliqueScalingGroup):
   - For each PCSG with TopologyConstraint, create TopologyConstraintGroupConfig
   - `TopologyConstraintGroupConfig.TopologyConstraint.PackConstraint.Required` ← topologyKey looked up from PCSG level
     name (if set)
   - `TopologyConstraintGroupConfig.TopologyConstraint.PackConstraint.Preferred` ← topologyKey of strictest level

3. **PodGroup Level** (from PodClique):
    - `PodGroup.TopologyConstraint.PackConstraint.Required` ← topologyKey looked up from PodClique level name (if set)
    - `PodGroup.TopologyConstraint.PackConstraint.Preferred` ← topologyKey of strictest level

**Example Translation:**

User creates PodCliqueSet with 3 replicas:

```yaml
spec:
   replicas: 3
  template:
    topologyConstraint:
       packLevel: "rack"  # User specifies level NAME (per-replica constraint)
```

Operator translates to PodGang:

```yaml
spec:
  topologyConstraint:
    packConstraint:
      required: "topology.kubernetes.io/rack"  # Operator looks up topologyKEY
      preferred: "kubernetes.io/hostname"  # Auto-generated topologyKEY of strictest level
```

**Per-Replica Behavior:**

- Replica 0: all pods constrained to one rack (e.g., rack-a)
- Replica 1: all pods constrained to one rack (e.g., rack-b)
- Replica 2: all pods constrained to one rack (e.g., rack-a)
- Different replicas can be in different racks (NOT all forced to same rack)

**Hierarchy Validation:**

- Child required constraints must be equal or stricter than parent required constraints
- Preferred constraints always use strictest level at all levels
- PodGang > NetworkPackGroup > PodGroup hierarchy maintained

**Mutable Topology Constraints:**

- Users can update topology constraints at any time
- Changes only affect new or unscheduled pods (already scheduled pods retain placement)
- Operator re-translates constraints to PodGang on each reconciliation

## Component Architecture

### Operator to Scheduler API Flow

When a PodCliqueSet is created or updated, the Grove Operator translates it into Grove Scheduler API (PodGang CRD):

**Translation Steps:**

1. User creates PodCliqueSet with optional `topologyConstraint.packLevel` (level name, e.g., "rack")
2. Operator loads TopologyDomain "grove-topology" and builds PodGang:
    - Looks up topologyKey for each user-specified level name (e.g., "rack" → "topology.kubernetes.io/rack")
   - **PodGang level**: PackConstraint with Required (topologyKey from PCS) + Preferred (strictest topologyKey)
   - **NetworkPackGroup level**: PackConstraint with Required (topologyKey from PCSG) + Preferred (strictest
     topologyKey)
   - **PodGroup level**: PackConstraint with Required (topologyKey from PodClique) + Preferred (strictest topologyKey)
    - Adds annotation `grove.run.ai/topology-name: "grove-topology"` to PodGang metadata
3. KAI scheduler reads annotation, uses packConstraints to apply three-level topology constraints

### End-to-End Flow

1. **Admin Setup**: Create TopologyDomain "grove-topology", configure operator with `EnableTopology: true`, create
   aligned Kueue Topology
2. **User Creates Workload**: PodCliqueSet with optional topology constraints
3. **Validation**: Webhook validates against TopologyDomain
4. **Translation**: Operator builds PodGang with three-level constraints (required + preferred)
5. **Scheduling**: KAI scheduler reads annotation, applies topology constraints with fallback

## Security and RBAC

Grove operator requires read access to TopologyDomain and permission to manage finalizers:

```yaml
rules:
   - apiGroups: [ "grove.io" ]
    resources: [ "topologydomains", "topologydomains/finalizers" ]
    verbs: [ "get", "list", "watch", "update" ]
```
