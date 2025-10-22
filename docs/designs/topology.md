# Topology-Aware Scheduling - Grove Operator Design

## Overview

This document defines the design for supporting topology-aware scheduling in the Grove operator.

**Motivation**: Topology-aware scheduling is critical for Grove's multinode inference workloads because these
applications require:

- **Network Locality**: Proximity improves high-bandwidth communication between leaders and their respective workers (
  prefill and decode, etc.)
- **Coordinated Placement**: Related components (e.g., model shards) perform better when co-located within the same
  topology domain
- **Latency Optimization**: Minimizing network hops between interdependent inference components improves end-to-end
  performance

**Design Approach**: This design introduces a flexible topology system with three main components:

1. **TopologyDomain CRD**: Admin-configured cluster topology hierarchy mapping friendly names (e.g., rack, zone, host)
   to node labels
2. **Operator Configuration**: Selects active topology via `--topology-domain-name` argument
3. **TopologyConstraint**: User-specified packing requirements in workloads (PodCliqueSet, PodCliqueScalingGroup,
   PodClique)

**Key Feature**: Grove attempts automatic out-of-box topology optimization by generating preferred (best-effort) packing
constraints at all levels, even without user configuration. This opportunistic packing may improve performance when
cluster resources allow, but users should specify required constraints when strict placement is critical for their
workload.

## Goals

- Provide flexible, cluster-agnostic topology hierarchy definition via TopologyDomain CRD
- Enable packing constraints for network locality across all Grove scalable resources
- Support multiple topology configurations for different environments
- Automatic Kueue Topology generation for KAI scheduler integration
- Immutable topology configuration ensuring scheduling consistency
- Hierarchical constraint validation (child stricter than parent)

## Non-Goals

- Spread constraints across topology domains (ReplicaSpreadDomain)
- Root domain constraints for entire resource (RootDomain)
- Ratio-based affinity groups between scaling groups (AffinityGroups with PackRatio)
- Dynamic topology reconfiguration after creation
- Per-workload topology domain selection
- Automatic topology inference from workload characteristics

## Proposal

### High-Level Approach

Grove implements topology-aware scheduling through three key components:

1. **TopologyDomain CRD**: Cluster-scoped resource defining topology hierarchy
    - Admin creates TopologyDomain with ordered list of topology levels
    - Each level maps friendly name (e.g., "rack", "zone", "host") to node label key (e.g., "
      topology.kubernetes.io/rack")
    - Multiple TopologyDomains supported for different environments

2. **Operator Configuration**: References TopologyDomain by name
    - Operator argument `--topology-domain-name=default` selects which TopologyDomain to use
    - All workload validation performed against configured TopologyDomain
    - Enables switching between topologies without changing workloads

3. **Workload API (TopologyConstraint)**: Users specify packing requirements
    - PodCliqueSet, PodCliqueScalingGroup, and PodClique each have TopologyConstraint field
    - Users reference level names from TopologyDomain (e.g., `packDomain: "rack"`)
    - No direct TopologyDomain reference needed in workloads

### Component Interactions

```
TopologyDomain CRD ──┐
  (admin creates)    │
                     │
Operator Config ─────┼──> Operator validates PackDomain
  (--topology-       │    against TopologyDomain.Spec.Levels
   domain-name)      │
                     │
PodCliqueSet ────────┘
  (packDomain: "rack")
```

### Automatic Optimization

**Out-of-Box Optimization:**

- Operator automatically generates **preferred** constraints using strictest topology level (e.g., "host")
- Applied at all three levels (PodGang, NetworkPackGroup, PodGroup) during translation to scheduler API
- Users get optimal packing without configuration

**User Control:**

- Users can specify **required** constraints via `packDomain` for strict placement requirements
- Required constraints validated and must be satisfied
- Preferred constraints enable best-effort optimization with graceful fallback

### Controller Responsibilities

The TopologyDomain controller manages:

- **Kueue Topology Generation**: Auto-creates Kueue Topology CRD for KAI scheduler integration
- **Deletion Protection**: Prevents deletion while PodCliqueSet resources reference it

## Out of Scope

The following features are explicitly out of scope for this design:

- **Spread Constraints**: ReplicaSpreadDomain for distributing replicas across domains for fault tolerance is not
  supported
- **Advanced Topology Constraints Per Replica**: RootDomain for constraining entire resource (all replicas) within a
  topology domain is not supported
- **Ratio Grouping Between Groups**: AffinityGroups with PackRatio for complex workload patterns (e.g., 2 Prefill + 1
  Decode ratios) is not supported
- **Workload-Based Auto Constraints**: Automatic constraint generation based on workload characteristics, patterns, and
  inference requirements

## Design Details

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Topology Architecture                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Admin Layer:                                                           │
│  ┌──────────────────┐              ┌────────────────────┐               │
│  │ TopologyDomain   │─────────────▶│ TopologyDomain     │               │
│  │ CR               │              │ Controller         │               │
│  │ (levels list)    │              └─────────┬──────────┘               │
│  └──────────────────┘                        │                          │
│         │                                    │                          │
│         │                                    ▼                          │
│         │                           ┌────────────────────┐              │
│         │                           │ Kueue Topology     │              │
│         │                           │ (auto-generated)   │              │
│         │                           └────────────────────┘              │
│         │                                                               │
│  Operator Config: --topology-domain-name=default                        │
│         │                                                               │
│         │ (validates against)                                           │
├─────────┼───────────────────────────────────────────────────────────────┤
│         │                                                               │
│  User Layer:                                                            │
│         ▼                                                               │
│  ┌──────────────────┐              ┌────────────────────┐               │
│  │ PodCliqueSet     │─────────────▶│ Grove Operator     │               │
│  │ (packDomain)     │              │ (reconciles)       │               │
│  └──────────────────┘              └─────────┬──────────┘               │
│                                              │                          │
│                                              │ (translates)             │
│                                              ▼                          │
│                                    ┌────────────────────┐               │
│                                    │ PodGang            │───────▶ KAI   │
│                                    │ • TopologyRef      │     Scheduler │
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

**Characteristics:**

- **Cluster-scoped**: Multiple TopologyDomains can exist
- **Operator-selected**: Operator references one by name via `--topology-domain-name` argument
- **Immutable**: Once created, cannot be modified
- **List-ordered hierarchy**: Index 0 = broadest (e.g., region), last = narrowest (e.g., host)

**API Structure:**

```go
// TopologyDomain defines the topology hierarchy for the cluster
// This resource is immutable after creation
// Multiple TopologyDomain resources can exist; Grove operator references one via --topology-domain-name argument
type TopologyDomain struct {
metav1.TypeMeta   `json:",inline"`
metav1.ObjectMeta `json:"metadata,omitempty"`

Spec TopologyDomainSpec `json:"spec,omitempty"`
}

type TopologyDomainSpec struct {
// Levels is an ordered list of topology levels from broadest to narrowest scope
// The order in this list defines the hierarchy (index 0 = highest level)
// This field is immutable
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="levels list is immutable"
// +kubebuilder:validation:MinItems=1
// +kubebuilder:validation:MaxItems=10
Levels []TopologyLevel `json:"levels"`
}

type TopologyLevel struct {
// Name is the level identifier used in TopologyConstraint references
// Must be a valid DNS label (lowercase alphanumeric with hyphens)
// Examples: "zone", "rack", "host"
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
Name string `json:"name"`

// TopologyKey is the node label key that identifies this topology domain
// Must be a valid Kubernetes label key (qualified name)
// Examples: "topology.kubernetes.io/zone", "kubernetes.io/hostname"
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=316
// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
TopologyKey string `json:"topologyKey"`

// Description provides human-readable information about this level
// +kubebuilder:validation:MaxLength=1024
// +optional
Description string `json:"description,omitempty"`
}
```

**Example TopologyDomain:**

```yaml
apiVersion: grove.run.ai/v1alpha1
kind: TopologyDomain
metadata:
  name: default
spec:
  levels:
    - name: region
      topologyKey: "topology.kubernetes.io/region"
      description: "Cloud provider region"
    - name: zone
      topologyKey: "topology.kubernetes.io/zone"
      description: "Availability zone within region"
    - name: datacenter
      topologyKey: "topology.kubernetes.io/datacenter"
      description: "Data center within zone"
    - name: block
      topologyKey: "topology.kubernetes.io/block"
      description: "Switching block within datacenter"
    - name: rack
      topologyKey: "topology.kubernetes.io/rack"
      description: "Network rack grouping"
    - name: host
      topologyKey: "kubernetes.io/hostname"
      description: "Individual compute host"
    - name: numa
      topologyKey: "topology.kubernetes.io/numa"
      description: "NUMA node within host"
```

**Creating TopologyDomain:**

Steps:

1. Install Grove: `helm install grove`
2. Customize example above with your cluster's actual `topologyKey` values
3. Create resource: `kubectl apply -f topologydomain.yaml`
4. Configure operator with `--topology-domain-name` matching the resource name
5. Create workloads with topology constraints

Notes:

- TopologyDomain becomes immutable after creation
- Multiple TopologyDomains can exist; operator uses the one specified in its argument
- Ensure node labels exist on cluster nodes before creating workloads
- List order defines hierarchy: index 0 = broadest, last = narrowest
- Example hierarchy: `region` (0) > `zone` (1) > `datacenter` (2) > `block` (3) > `rack` (4) > `host` (5) > `numa` (6)

**Validation:**

CRD-Level:

- At least one level required (minimum 1, maximum 10)
- Level `name` required (max 63 chars)
- Level `topologyKey` required (max 316 chars)
- Level `description` optional (max 1024 chars)
- Entire levels list immutable after creation

Webhook:

- Each level `name` must be unique within the `levels` array of a single TopologyDomain
- Each `topologyKey` must be unique within the `levels` array of a single TopologyDomain
- Cannot modify any field after creation
- Deletion protection via controller finalizer

**Node Label Responsibility:**

- Cluster administrators are responsible for ensuring that node labels specified in `topologyKey` fields exist on
  cluster nodes
- TopologyDomain creation succeeds even if labels don't exist yet (allows pre-configuration)
- Workloads may fail to schedule if referenced topology labels are missing from nodes
- Administrators should verify node labels match TopologyDomain configuration before creating workloads

#### TopologyDomain Controller

The TopologyDomain controller manages the TopologyDomain resource lifecycle with two primary responsibilities:

**1. Kueue Topology Generation**

Automatically generates Kueue Topology CRD from the TopologyDomain referenced by operator's `--topology-domain-name`
argument.

**Why Kueue Topology is Required:**

Grove uses its own TopologyDomain CRD for user-friendly admin/user API, but KAI scheduler specifically requires Kueue's
Topology CRD format for actual scheduling operations. The TopologyDomain controller bridges this gap by:

- Reading Grove's TopologyDomain (user-friendly with level names like "rack", "zone")
- Automatically generating Kueue Topology (KAI scheduler's required format with node labels only)
- Maintaining consistency between both representations
- Eliminating manual coordination for admins

This separation allows Grove to provide better UX while maintaining compatibility with KAI scheduler requirements.

Generation Process:

1. Controller watches TopologyDomain specified in operator argument
2. When TopologyDomain created, controller creates matching Kueue Topology
3. Kueue Topology name matches TopologyDomain name
4. Levels extracted from TopologyDomain.Spec.Levels using topologyKey field
5. Order preserved from TopologyDomain list

Example:

From TopologyDomain `default` with levels zone/rack/host, controller generates:

```yaml
apiVersion: kueue.x-k8s.io/v1alpha1
kind: Topology
metadata:
  name: default
  ownerReferences:
    - apiVersion: grove.run.ai/v1alpha1
      kind: TopologyDomain
      name: default
      controller: true
spec:
  levels:
    - nodeLabel: "topology.kubernetes.io/zone"
    - nodeLabel: "topology.kubernetes.io/rack"
    - nodeLabel: "kubernetes.io/hostname"
```

Key Points:

- Admin only creates TopologyDomain; Kueue Topology auto-generated
- Owner reference ensures Kueue Topology deleted with TopologyDomain
- Same name for both resources
- No manual coordination required

**Implementation Note:**

To avoid importing the entire Kueue package with all its dependencies, the operator will use Kubernetes unstructured API
to create and manage Kueue Topology CRDs. This approach is acceptable since the Kueue Topology CRD structure is simple (
just a list of node label keys).

**2. Deletion Protection**

Prevents TopologyDomain deletion while PodCliqueSet resources reference it using Kubernetes finalizer.

Deletion Workflow:

1. Admin runs `kubectl delete topologydomain default`
2. Kubernetes blocks deletion (finalizer `grove.run.ai/topology-protection` present)
3. Controller reconciles:
    - Detects deletion request (deletion timestamp set)
    - Scans cluster for any PodCliqueSet resources
    - If PodCliqueSet exists: Keeps finalizer, deletion blocked
    - If no PodCliqueSet exists: Removes finalizer, deletion proceeds
4. Once finalizer removed, Kubernetes deletes TopologyDomain

Why Only Check PodCliqueSet:

- Grove ownership hierarchy: PodCliqueSet owns PodCliqueScalingGroup and PodClique
- If no PodCliqueSet exists, other resources cannot exist

Key Points:

- Admin must delete all PodCliqueSet before deleting TopologyDomain
- Controller continuously reconciles
- Prevents orphaned workloads with invalid topology references

#### Operator Configuration

Operator references TopologyDomain by name via command-line argument:

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
            - --topology-domain-name=default  # References TopologyDomain by name
```

Configuration:

- `--topology-domain-name`: Specifies TopologyDomain resource name for validation
- Operator loads referenced TopologyDomain at startup
- All PodCliqueSet topology constraints validated against this TopologyDomain

**Runtime Behavior:**

When TopologyDomain is Missing or Deleted:

- **Startup**: If `--topology-domain-name` is configured but TopologyDomain doesn't exist at startup, operator fails to
  start
    - Operator requires TopologyDomain to exist for auto-optimization (preferred constraints generation)
    - This explicit failure prevents silent degradation of topology features
    - Admin must create TopologyDomain or remove `--topology-domain-name` argument before operator starts

- **During Runtime**: If TopologyDomain is deleted while operator is running:
    - Finalizer prevents deletion while any PodCliqueSet resources exist
    - If all PodCliqueSet resources are removed and TopologyDomain is deleted:
        - Operator blocks creation of ALL new workloads (topology and non-topology)
        - Admin must either create new TopologyDomain OR remove `--topology-domain-name` operator argument and restart
        - This explicit behavior prevents implicit edge cases and ensures topology configuration consistency

**Multiple Topologies:**

- Multiple TopologyDomain resources can exist (e.g., "aws-topology", "on-prem-topology")
- Operator argument selects which one to use
- Enables different topology configurations per environment

### 2. Operator API Changes (Grove CRDs)

#### TopologyConstraint Model

```go
type TopologyConstraint struct {
// PackDomain references a level name from TopologyDomain.Spec.Levels
// Defines required topology packing constraint for replicas
// Replicas packed together within specified topology level for network locality
PackDomain *string `json:"packDomain,omitempty"`
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

The validation webhook ensures topology configuration consistency:

**TopologyDomain Reference:**

- TopologyDomain specified in operator's `--topology-domain-name` must exist
- Referenced PackDomain name must exist in TopologyDomain.Spec.Levels
- All validation performed against operator-configured TopologyDomain

**Hierarchy Constraints:**

- Child resource PackDomain must be equal to or stricter than parent
- PodCliqueSet → PodCliqueScalingGroup → PodClique hierarchy
- Stricter = higher index (narrower scope) in TopologyDomain.Spec.Levels
- Example: If parent uses "zone" (index 1), child can use "zone", "rack" (index 4), or "host" (index 5)
- Validation applies on both CREATE and UPDATE operations
- During updates, hierarchy constraints are re-validated to ensure child remains equal or stricter than parent

### 3. Scheduler API Changes (Contract with KAI)

#### PodGang CRD Extensions

The Grove Operator translates topology configuration into Grove Scheduler API format, which serves as the contract with
KAI scheduler.

**PodGangSpec:**

```go
type PodGangSpec struct {
// PodGroups is a list of member pod groups in the PodGang
PodGroups []PodGroup `json:"podgroups"`

// TopologyRef references the Kueue Topology resource
// Points to Kueue Topology CRD auto-generated by TopologyDomain controller
// +optional
TopologyRef *string `json:"topologyRef,omitempty"`

// TopologyConstraint defines topology packing constraints for entire pod gang
// Translated from PodCliqueSet.TopologyConstraint
// Updated by operator on each reconciliation when PodCliqueSet topology constraints change
// +optional
TopologyConstraint *TopologyConstraint `json:"topologyConstraint,omitempty"`

// NetworkPackGroupConfigs defines groups of PodGroups for network optimization
// Enhanced with topology constraints for PCSG-level packing
// Updated by operator on each reconciliation when PCSG topology constraints change
// +optional
NetworkPackGroupConfigs []NetworkPackGroupConfig `json:"networkPackGroupConfigs,omitempty"`

// PriorityClassName is the name of the PriorityClass for the PodGang
PriorityClassName string `json:"priorityClassName,omitempty"`
}
```

**NetworkPackGroupConfig:**

```go
// NetworkPackGroupConfig indicates PodGroups should be optimally placed w.r.t cluster's network topology
type NetworkPackGroupConfig struct {
// PodGroupNames is the list of PodGroup names in the network pack group
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
// Required defines topology constraint that must be satisfied
// Populated from user's packDomain specification in operator API
// +optional
Required *PackConstraint `json:"required,omitempty"`

// Preferred defines best-effort topology constraint
// Auto-generated by operator using strictest level for optimization
// Scheduler can fallback to less strict levels if preferred cannot be satisfied
// +optional
Preferred *PackConstraint `json:"preferred,omitempty"`
}

type PackConstraint struct {
// PackDomain references a level name from TopologyDomain.Spec.Levels
PackDomain string `json:"packDomain"`
}
```

**Changes Summary:**

Fields Added:

- `PodGangSpec.TopologyRef *string` - References Kueue Topology CRD (optional pointer)
- `PodGangSpec.TopologyConstraint *TopologyConstraint` - PodGang-level packing from PodCliqueSet (optional pointer)
- `NetworkPackGroupConfig.TopologyConstraint *TopologyConstraint` - PCSG-level packing from PodCliqueScalingGroup (
  optional pointer)
- `PodGroup.TopologyConstraint *TopologyConstraint` - PodClique-level packing from PodClique (optional pointer)

Fields Removed:

- `PodGangSpec.SpreadConstraints` - Not implemented; spread will be part of TopologyConstraint in future

**Note:** All TopologyConstraint fields are pointers with omitempty, allowing workloads without topology constraints.

#### Translation Logic

The operator translates Grove operator API to Grove Scheduler API with three-level topology constraint hierarchy:

**TopologyRef Population:**

- Set to Kueue Topology resource name (matches TopologyDomain name from operator config)
- Example: operator config `--topology-domain-name=default` → `TopologyRef.Name="default"`
- KAI scheduler uses this to locate the Kueue Topology CRD

**Constraint Translation (Required and Preferred):**

The operator translates user's simple PackDomain into rich required/preferred structure in scheduler API:

**Required Constraints:**

- If user specifies `packDomain: "rack"` → becomes `TopologyConstraint.Required.PackDomain = "rack"`
- If user doesn't specify packDomain → `Required` is nil
- Applied at the appropriate level (PodGang, NetworkPackGroup, or PodGroup)

**Preferred Constraints (Auto-Generated):**

- Operator ALWAYS generates preferred constraint at all three levels
- Uses strictest/lowest level from TopologyDomain.Spec.Levels (e.g., "host")
- Enables out-of-box optimization even without user configuration
- Scheduler can fallback to less strict levels if preferred cannot be satisfied

**Three-Level Translation:**

1. **PodGang Level** (from PodCliqueSet):
    - `PodGangSpec.TopologyConstraint.Required` ← user's `PodCliqueSet.TopologyConstraint.PackDomain` (if set)
    - `PodGangSpec.TopologyConstraint.Preferred` ← auto-generated strictest level (e.g., "host")

2. **NetworkPackGroup Level** (from PodCliqueScalingGroup):
    - For each PCSG with TopologyConstraint, create NetworkPackGroupConfig
    - `NetworkPackGroupConfig.TopologyConstraint.Required` ← user's `PCSG.TopologyConstraint.PackDomain` (if set)
    - `NetworkPackGroupConfig.TopologyConstraint.Preferred` ← auto-generated strictest level

3. **PodGroup Level** (from PodClique):
    - `PodGroup.TopologyConstraint.Required` ← user's `PodClique.TopologyConstraint.PackDomain` (if set)
    - `PodGroup.TopologyConstraint.Preferred` ← auto-generated strictest level

**Example Translation:**

User creates PodCliqueSet:

```yaml
spec:
  template:
    topologyConstraint:
      packDomain: "rack"  # User specifies required constraint
```

Operator translates to PodGang:

```yaml
spec:
  topologyConstraint:
    required:
      packDomain: "rack"  # From user
    preferred:
      packDomain: "host"  # Auto-generated by operator
```

**Hierarchy Validation:**

- Child required constraints must be equal or stricter than parent required constraints
- Preferred constraints always use strictest level at all levels
- PodGang > NetworkPackGroup > PodGroup hierarchy maintained

**Mutable Topology Constraints:**

- Users can update topology constraints at any time (PodCliqueSet, PodCliqueScalingGroup, PodClique levels)
- Constraint changes only affect new or unscheduled pods
- Already scheduled pods retain their current placement and are not rescheduled
- Operator re-translates constraints to PodGang on each reconciliation triggered by updates
- Useful for adjusting placement requirements when workloads fail to schedule due to resource constraints

## Component Architecture

### Operator to Scheduler API Flow

When a PodCliqueSet is created or updated, the Grove Operator translates it into Grove Scheduler API (PodGang CRD):

**Step-by-Step Translation:**

1. **PodCliqueSet Created/Updated**:
    - User creates PodCliqueSet with optional `topologyConstraint.packDomain`
    - Validation webhook validates against TopologyDomain

2. **Operator Reconciles PodCliqueSet**:
    - Operator detects PodCliqueSet creation/update
    - Loads TopologyDomain specified in operator config (`--topology-domain-name`)
    - Prepares PodGang resource creation/update

3. **Build PodGang TopologyConstraint**:
    - **Required**: From user's `PodCliqueSet.topologyConstraint.packDomain` (if specified)
    - **Preferred**: Auto-generated using strictest/lowest level from TopologyDomain.Spec.Levels (e.g., "host")
    - Populates `PodGangSpec.TopologyConstraint`

4. **Build NetworkPackGroupConfigs**:
    - For each PodCliqueScalingGroup with TopologyConstraint in PodCliqueSet
    - Create NetworkPackGroupConfig entry with PodGroupNames from that PCSG
    - **Required**: From `PCSG.topologyConstraint.packDomain` (if specified)
    - **Preferred**: Auto-generated strictest level
    - Populates `PodGangSpec.NetworkPackGroupConfigs`

5. **Build PodGroups with TopologyConstraint**:
    - For each PodClique in PodCliqueSet, create corresponding PodGroup
    - **Required**: From `PodClique.topologyConstraint.packDomain` (if specified)
    - **Preferred**: Auto-generated strictest level
    - Populates `PodGroup.TopologyConstraint` for each PodGroup

6. **Set TopologyRef**:
    - References Kueue Topology by name (matches TopologyDomain name from operator config)
    - Example: `--topology-domain-name=default` → `TopologyRef.Name="default"`
    - KAI scheduler uses this to locate the Kueue Topology CRD

7. **Create/Update PodGang in Scheduler API**:
    - Operator calls Grove Scheduler API to create/update PodGang
    - PodGang now has complete topology information at three levels
    - KAI scheduler consumes PodGang and applies topology-aware scheduling

**Key Points:**

- Operator reconciliation performs translation
- Preferred constraints auto-generated at reconciliation time for out-of-box optimization
- Three-level hierarchy maintained: PodGang > NetworkPackGroup > PodGroup
- TopologyRef connects PodGang to KAI scheduler's required Kueue Topology
- All levels get both required (user-specified) and preferred (auto-generated) constraints

### Topology-Aware Scheduling Flow

High-level end-to-end flow:

1. **Admin Setup**: Create TopologyDomain, configure operator
2. **User Creates Workload**: PodCliqueSet with optional topology constraints
3. **Validation**: Webhooks validate against TopologyDomain
4. **Translation**: Operator builds PodGang with three-level constraints
5. **Scheduling**: KAI scheduler applies topology constraints with fallback

### Sequence Diagram

```
┌──────────────┐    ┌──────────────────┐     ┌─────────────────┐    ┌─────────────────┐
│ PodCliqueSet │    │  Grove Operator  │     │ Grove Scheduler │    │   Scheduler     │
│              │    │                  │     │      API        │    │                 │
└──────┬───────┘    └─────────┬────────┘     └────────┬────────┘    └────────┬────────┘
       │                      │                       │                      │
       │ CREATE/UPDATE        │                       │                      │
       ├─────────────────────▶│                       │                      │
       │                      │                       │                      │
       │                      │ 1. Validation webhook │                      │
       │                      │    validates against  │                      │
       │                      │    TopologyDomain     │                      │
       │                      │                       │                      │
       │                      │ 2. Translate to       │                      │
       │                      │    PodGang(s) spec    │                      │
       │                      │                       │                      │
       │                      │ CREATE/UPDATE PodGangs│                      │
       │                      ├─────────────────────▶ │                      │
       │                      │                       │                      │
       │                      │                       │ SCHEDULE Pods        │
       │                      │                       ├─────────────────────▶│
       │                      │                       │                      │
       │                      │                       │                      │ Apply topology
       │                      │                       │                      │ using Kueue
       │                      │                       │                      │ Topology CRD
       │                      │                       │                      │
```

## Implementation Notes

### Edge Cases

**Case 1: TopologyDomain Not Configured**

- If `--topology-domain-name` argument not provided to operator: topology features completely disabled
- PodCliqueSet workloads without `packDomain` function normally
- PodCliqueSet workloads with `packDomain` specified: validation webhook rejects creation (cannot validate without
  TopologyDomain)
- No auto-optimization (preferred constraints) applied

**Case 2: TopologyDomain Configured but Missing at Startup**

- If `--topology-domain-name` argument provided but TopologyDomain resource doesn't exist: operator fails to start
- Operator requires TopologyDomain to exist for auto-optimization
- Admin must either:
    - Create the referenced TopologyDomain resource, OR
    - Remove `--topology-domain-name` argument from operator configuration

**Case 3: TopologyDomain Deleted During Runtime**

- Finalizer prevents deletion while any PodCliqueSet resources exist
- If TopologyDomain deleted after all PodCliqueSet resources removed:
    - Operator blocks creation of ALL new workloads (topology and non-topology)
    - Existing workloads continue to function (already scheduled)
- Admin must either:
    - Create new TopologyDomain resource with same name, OR
    - Remove `--topology-domain-name` argument and restart operator

**Case 4: Topology Features Enabled/Disabled**

- **Enabled**: When `--topology-domain-name` provided and TopologyDomain exists
    - Auto-optimization active for all workloads (preferred constraints generated)
    - User-specified `packDomain` validated and enforced as required constraints
- **Disabled**: When `--topology-domain-name` argument not provided
    - Topology constraints in workload CRDs ignored during scheduling
    - Workloads schedule without topology awareness
- **Toggling**: Cannot enable/disable during runtime - requires operator restart with updated configuration

### Resolved Design Questions

This section documents key design decisions and their resolutions.

**Q: How will cluster admins map Grove topology constants to physical topology labels?**

**A: The `TopologyDomain` CRD provides the mapping mechanism. Admins create a TopologyDomain resource with
an ordered list of levels, where each level maps a friendly name (e.g., "rack", "zone", "host") to a node label key (
e.g., "
topology.kubernetes.io/rack"). This provides a clean, declarative API for topology configuration.

**Q: Should we allow changes to cluster topology levels and mappings after creation?**

**A: No (Immutable)** - TopologyDomain and all TopologyConstraint fields are immutable after creation. This
prevents unpredictable behavior with in-flight workloads and maintains scheduling consistency. To change topology
configuration:

1. Create a new TopologyDomain with updated configuration
2. Update operator's `--topology-domain-name` argument to reference new TopologyDomain
3. Drain or migrate existing workloads
4. Delete old TopologyDomain after all workloads are migrated

**Q: If topology constraints cannot be satisfied, should workloads remain pending or schedule anyway?**

**A: Remain Pending** - For gang-scheduled workloads with topology constraints:

- **Required Constraints** (user-specified `packDomain`): Must be satisfied; entire gang remains pending if unsatisfied
- **Preferred Constraints** (auto-generated): Best-effort optimization; scheduler can fall back to less strict levels
- This behavior ensures workload integrity for tightly-coupled distributed inference workloads where partial scheduling
  is ineffective
- Users relying on strict placement should use required constraints; users wanting flexibility should rely on preferred
  constraints

**Q: How will domain-level packing be realized in KAI scheduler?**

**A: Contract Defined** - The `PodGang` CRD serves as the API contract between Grove operator and KAI scheduler.
Expected scheduler behavior:

1. **Topology Resolution**: KAI Pod Grouper reads `PodGang.spec.topologyRef` to locate Kueue Topology CRD
2. **Constraint Processing**: For each topology constraint (PodGang, NetworkPackGroup, PodGroup level):
    - Process `required` constraints first (must satisfy)
    - Apply `preferred` constraints as optimization hints (best-effort)
3. **Domain Filtering**: Filter cluster nodes to find topology domains (e.g., single rack, single host) that satisfy:
    - Resource requests for all pods in the constraint scope
    - Required topology level specified in constraint
4. **Placement**: Schedule all pods in the constraint scope within the chosen topology domain
5. **Fallback**: For preferred constraints, fall back to less strict topology levels if preferred level cannot be
   satisfied
6. **Gang Semantics**: If required constraints cannot be satisfied, entire gang remains unscheduled (all-or-nothing)

This contract ensures Grove workloads receive topology-aware placement while maintaining scheduler independence.

## Security and RBAC

The topology system requires careful RBAC configuration to ensure proper separation of concerns between cluster
administrators and the operator.

### ClusterRole: Grove Operator

The Grove operator requires read access to TopologyDomain and full management of Kueue Topology:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: grove-operator-topology
rules:
  - apiGroups: [ "grove.run.ai" ]
    resources: [ "topologydomains" ]
    verbs: [ "get", "list", "watch" ]
  - apiGroups: [ "kueue.x-k8s.io" ]
    resources: [ "topologies" ]
    verbs: [ "create", "delete", "get", "list", "watch", "update", "patch" ]
```
