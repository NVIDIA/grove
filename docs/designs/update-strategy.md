# Update Strategy - Grove Operator Design

## Overview

This document proposes additional configuration options for the update strategy of PodCliqueSet, PodCliqueScalingGroup and PodClique resources for the Grove operator.

## Motivation

Currently, Grove implements a single, non-configurable default update strategy across all resource levels:

**PodCliqueSet (PCS) Replica Updates:**

- Updates one replica at a time (sequential)
- Replica selection priority: unscheduled → unhealthy (minAvailableBreached) → ascending ordinal

**Within PCS Replica - PodCliqueScalingGroup (PCSG) Updates:**

- Updates one PCSG replica at a time (sequential)
- Replica selection: ascending ordinal (lowest index first)
- Entire PCSG replica is deleted and recreated (all member PodCliques together)

**Within PCS Replica - Standalone PodClique (PC) Updates:**

- Updates one pod at a time (sequential)
- Pod selection: oldest pod first (by creation timestamp)
- Individual pods are deleted and recreated

This default behavior provides safe, conservative updates but lacks user configurability. At the PCSG and standalone PC levels, the update corresponds to maxUnavailable 1 and maxSurge 0 where a singular old replica is deleted and new one is created.

## Use Cases

### Incompatible Version Updates

Consider a multinode disaggregated deployment with 2 PCS replicas, where each replica contains:

- **Frontend** standalone PC: 3 replicas
- **Prefill** PCSG: 2 replicas (prefill-leader PC: 1 replica, prefill-worker PC: 2 replicas)
- **Decode** PCSG: 2 replicas (decode-leader PC: 1 replica, decode-worker PC: 2 replicas)

**Default Update Behavior:**

1. PCS replica 0 is selected for update
2. Frontend PC updates one pod at a time (3 pods sequentially)
3. Prefill PCSG updates one replica at a time (2 replicas sequentially)
4. Decode PCSG updates one replica at a time (2 replicas sequentially)
5. PCS replica 1 repeats the same process

**Problem with Incompatible Versions:**

When the new frontend and worker versions introduce breaking API changes or protocol incompatibilities, the incremental update strategy creates a problematic coexistence period:

- During frontend pod updates, old and new frontend pods run concurrently, sending requests to a mix of old and new PCSG replicas
- During PCSG updates, new frontend pods communicate with old PCSG replicas (or vice versa)
- Communication between new prefill replicas and old decode replicas (or vice versa) can cause protocol mismatches
- This mixed-version state causes communication failures, incorrect behavior, or crashes

The default strategy assumes version compatibility during the update window. For incompatible updates, users need the ability to tear down the entire PCS replica atomically to prevent any cross-version communication within a replica.

### Compatible Configuration Update

Using the same multinode disaggregated deployment example, consider an update where the new versions are compatible between Frontend, Prefill, and Decode components. The update follows the same Default Update Behavior, but users may want to optimize the update process based on their capacity and SLA requirements.

**Scenario 1: Maintaining Full Capacity During Updates**

A user with excess cluster capacity wants to maintain full service capacity during updates. They would like to:

- Surge additional prefill and decode replicas before deleting old ones
- Keep existing replicas running to meet SLAs while new replicas are created
- Only delete old replicas after new ones are ready

However, the current default strategy (`maxUnavailable=1, maxSurge=0`) requires deleting an old replica before creating a new one. While service availability is maintained (when replica count > 1), this reduces capacity during the update window. With excess capacity available, users should be able to surge replicas to maintain full capacity throughout the update.

**Scenario 2: Faster Updates with Multiple Replicas**

A user with 4 replicas of prefill/decode each wants to speed up the update process. They would like to:

- Update 2 replicas simultaneously (`maxUnavailable=2`) instead of one at a time
- Reduce overall update time while still maintaining service availability
- Balance update speed with acceptable capacity reduction during the update window

The current default strategy only allows `maxUnavailable=1`, forcing sequential updates even when the deployment can tolerate multiple replicas being unavailable simultaneously.

## Goals

- **Enable application version incompatible updates** through PodCliqueSet-level replica recreation strategy, allowing entire PCS replicas to be torn down atomically to prevent cross-version communication issues

- **Enable surge capability** at the PCSG level for multinode deployments and PC level for singlenode inference, allowing new replicas to be created before old ones are deleted to maintain full capacity during updates

- **Enable faster rollouts** by allowing multiple replicas to be recreated simultaneously at the PCSG level for multinode deployments and PC level for singlenode inference, configurable via `maxUnavailable` settings

## Non-Goals

- Partition-based rolling updates or canary deployments - These are advanced features that can be considered in future phases

- Automatic strategy selection based on version compatibility detection - Users must explicitly configure update strategies

- Automatic selection of surge based on capacity - Users are expected to be aware of their cluster capacity and should explicitly configure surge settings. Grove does not have visibility into available cluster capacity at update time and cannot automatically determine if surge is feasible

## Proposal

This proposal expands Grove's default RollingUpdate strategy to support replica recreation at the PodCliqueSet level for version incompatible upgrades, as well as `maxUnavailable` and `maxSurge` configuration at the PCS level. Additionally, it introduces `maxUnavailable` and `maxSurge` concepts at the standalone PodClique and PodCliqueScalingGroup levels to enable faster rollouts and capacity maintenance during compatible upgrades.

## Architecture

### Three-Level Update Control

```
PodCliqueSet (Top Level)
├─ UpdateStrategy (controls PCS replica updates)
│  ├─ RollingUpdate: one replica at a time
│  └─ ReplicaRecreate: tear down entire replica
│
├─ PCS Replica 0
│  ├─ Standalone PodCliques (concurrent updates)
│  │  └─ frontend: UpdateStrategy (controls pod updates)
│  │     └─ maxUnavailable/maxSurge
│  │
│  └─ PodCliqueScalingGroups (concurrent updates)
│     ├─ prefill: UpdateStrategy (controls PCSG replica updates)
│     │  └─ maxUnavailable/maxSurge
│     └─ decode: UpdateStrategy (controls PCSG replica updates)
│        └─ maxUnavailable/maxSurge
│
└─ PCS Replica 1, 2, ... (sequential per PCS strategy)
```

## API Structure

### 1. PodCliqueSetSpec.UpdateStrategy

Controls **how PCS replicas are updated** (the top-most level).

```go
type PodCliqueSetUpdateStrategy struct {
    Type          PodCliqueSetUpdateStrategyType
    RollingUpdate *PodCliqueSetRollingUpdateStrategy
}

type PodCliqueSetUpdateStrategyType string
const (
    RollingUpdate    // Update replicas sequentially
    ReplicaRecreate  // Delete/recreate entire replica
)

type PodCliqueSetRollingUpdateStrategy struct {
    MaxUnavailable *int32  // How many PCS replicas can be down
    MaxSurge       *int32  // How many extra PCS replicas to create
}
```

**Defaults:** `RollingUpdate` with `maxUnavailable=1, maxSurge=0`

### 2. PodCliqueTemplateSpec.UpdateStrategy

Controls **how pods update within a standalone PodClique**.

```go
type ComponentUpdateStrategy struct {
    MaxUnavailable *int32  // How many pods can be down
    MaxSurge       *int32  // How many extra pods to create
}
```

**Defaults:** `maxUnavailable=1, maxSurge=0`
**Scope:** Only applies to standalone PodCliques (not in a PCSG)

### 3. PodCliqueScalingGroupConfig.UpdateStrategy

Controls **how PCSG replicas update within a scaling group**.

```go
type ComponentUpdateStrategy struct {
    MaxUnavailable *int32  // How many PCSG replicas can be down
    MaxSurge       *int32  // How many extra PCSG replicas to create
}
```

**Defaults:** `maxUnavailable=1, maxSurge=0`
**Scope:** Controls the update of PCSG replicas (groups of PodCliques)

## Update Behavior

### RollingUpdate (Default)

The default RollingUpdate behavior is described in the [Motivation](#motivation) section. When using the RollingUpdate strategy, `maxUnavailable` and `maxSurge` settings at the PodCliqueSet level have no effect - PCS replicas are always updated one at a time sequentially.

### ReplicaRecreate

**Behavior:**

- Deletes **entire PCS replica** (all PCSGs + standalone PCs) atomically
- All components within the replica are deleted simultaneously
- Recreates all components together with the new configuration
- **Bypasses** individual component update strategies (PC and PCSG `maxUnavailable`/`maxSurge` settings are ignored)

**MaxUnavailable and MaxSurge:**

When using the ReplicaRecreate strategy, `maxUnavailable` and `maxSurge` at the PodCliqueSet level control how many PCS replicas can be recreated simultaneously:

- `maxUnavailable`: Maximum number of PCS replicas that can be down (deleted) at once during the update
- `maxSurge`: Maximum number of extra PCS replicas that can be created above the desired replica count during the update

For example, with `replicas=2` and `maxUnavailable=1, maxSurge=0`:

- One PCS replica is deleted and recreated at a time
- The other replica remains available during the update

With `replicas=2` and `maxUnavailable=2, maxSurge=0`:

- Both PCS replicas can be deleted and recreated simultaneously
- This provides faster updates but results in complete service unavailability during the recreation window

**Use Cases:**

- Version incompatible updates where old and new versions cannot coexist
- Need to clear all state at once within a replica
- Coordinated recreation of interdependent components to prevent cross-version communication issues

## MaxSurge Considerations

### PodClique MaxSurge

**Indexing Strategy:**

PodClique uses an index tracker that extracts pod indices from hostnames and fills holes automatically. When surge pods are created:

1. **Surge pods get indices above replica count**: With `replicas=3` and `maxSurge=1`, surge pod gets index 3 (or higher if holes exist)
2. **Index tracker fills holes**: When old pods are deleted, their indices become available. The tracker fills holes from lowest to highest (starting from 0)
3. **No holes at end of update**: As old pods are deleted and recreated, new pods fill the lowest available indices, ensuring sequential indices `[0, replicas-1]` at completion

**Example with `replicas=3`, `maxSurge=1`, `maxUnavailable=0`:**

1. **Initial:** Pods with indices 0, 1, 2 (old spec)
2. **Create surge:** Pod with index 3 (surge, new spec) - now have [0, 1, 2, 3]
3. **Delete pod 0:** Index 0 becomes available
4. **Recreate pod 0:** New pod fills index 0 (new spec) - now have [0, 1, 2, 3]
5. **Delete pod 1:** Index 1 becomes available
6. **Recreate pod 1:** New pod fills index 1 (new spec) - now have [0, 1, 2, 3]
7. **Delete pod 2:** Index 2 becomes available
8. **Recreate pod 2:** New pod fills index 2 (new spec) - now have [0, 1, 2, 3]
9. **Delete surge pod 3:** Final state [0, 1, 2] - no holes

**Gang Scheduling Impact:**

- Surge pods are added to the same PodGroup as existing pods
- PodGroup's `PodReferences` list includes all pods (old + surge)
- Gang scheduling requires the PodGroup to meet `MinReplicas` (from PodClique's `MinAvailable`)
- All pods in the PodGroup (including surge) must be scheduled together as part of the gang
- If surge pod cannot be scheduled, the entire gang is blocked

**PodGang/PodGroup Construction:**

- PodGroup contains pod references from the PodClique
- During surge, PodGroup temporarily has more pod references than `replicas` count
- PodGroup's `MinReplicas` is set to PodClique's `MinAvailable` (not affected by surge)
- Gang scheduling ensures at least `MinReplicas` pods are scheduled together

**Stuck Scenarios:**

- **Surge pod cannot be scheduled**: Gang scheduling blocks until surge pod can be scheduled, update stuck
- **Surge pod scheduled but not ready**: Update cannot proceed if `maxUnavailable=0` requires surge pod to be ready before deleting old pods

### PodCliqueScalingGroup MaxSurge

**Indexing Strategy:**

PodCliqueScalingGroup replicas use replica indices (0, 1, 2, ...). When surge replicas are created:

1. **Surge replicas get indices above replica count**: With `replicas=3` and `maxSurge=1`, surge replica gets index 3
2. **Replica placement depends on minAvailable**:
   - If `replicas <= minAvailable`: All replicas (including surge) go into base PodGang
   - If `replicas > minAvailable`: Surge replica goes into scaled PodGang
3. **No holes at end of update**: Original replica indices `[0, replicas-1]` are maintained, surge replicas at `[replicas, replicas+maxSurge-1]` are deleted after update completes

**Example with `replicas=3`, `minAvailable=3`, `maxSurge=1`, `maxUnavailable=0`:**

1. **Initial:** Replicas 0, 1, 2 in base PodGang (old spec)
2. **Create surge:** Replica 3 in scaled PodGang (surge, new spec) - replicas 0, 1, 2 in base PodGang; replica 3 in scaled PodGang
3. **Wait for surge available:** Replica 3 becomes available (scaled PodGang gated by base PodGang readiness)
4. **Delete and recreate replica 0:** Replica 0 (new spec) in base PodGang
5. **Wait for replica 0 available:** Replica 0 becomes available
6. **Delete and recreate replica 1:** Replica 1 (new spec) in base PodGang
7. **Wait for replica 1 available:** Replica 1 becomes available
8. **Delete and recreate replica 2:** Replica 2 (new spec) in base PodGang
9. **Wait for replica 2 available:** Replica 2 becomes available
10. **Delete surge replica 3:** Final state replicas [0, 1, 2] - no holes

**Example with `replicas=3`, `minAvailable=2`, `maxSurge=1`:**

1. **Initial:** Replicas 0, 1 in base PodGang; Replica 2 in scaled PodGang (old spec)
2. **Create surge:** Replica 3 in scaled PodGang (surge, new spec)
3. **Update proceeds:** Replicas 0, 1, 2 updated, then surge replica 3 deleted

**Gang Scheduling Impact:**

- **Base PodGang (replicas 0 to minAvailable-1)**: All PodGroups in base PodGang must meet `MinReplicas` for gang scheduling to proceed.
- **Scaled PodGangs (replicas >= minAvailable)**: Surge replicas (always at indices >= replicas, which is >= minAvailable) get their own scaled PodGang. Scaled PodGangs are gated by base PodGang readiness - gates are removed only after base PodGang is ready.
- **Gang scheduling constraints**: Each PodGroup (one per PodClique in the PCSG replica) must meet its `MinReplicas` for the gang to be scheduled.

**PodGang/PodGroup Construction:**

- **Base PodGang**: Contains PodGroups for replicas 0 to `minAvailable-1`.
- **Scaled PodGangs**: Each replica >= `minAvailable` gets its own scaled PodGang. Surge replicas (always at indices >= replicas >= minAvailable) create new scaled PodGangs.
- **PodGroup per PodClique**: Each PodClique in a PCSG replica becomes a PodGroup. Surge replica creates PodGroups for all its PodCliques.

**Stuck Scenarios:**

- **Surge replica cannot be scheduled**: Surge replica is always in a scaled PodGang. If scaled PodGang is blocked and base PodGang is updating, creates circular dependency
- **Base PodGang update blocks surge scaled PodGang**: Surge replica in scaled PodGang is gated by base PodGang readiness. If base is updating, surge cannot proceed.
- **Surge replica scheduled but not ready**: Update cannot proceed if `maxUnavailable=0` requires surge replica to be available before deleting old replicas.
- **Gang scheduling amplification**: Surge replica contains multiple PodCliques (one per `CliqueName`), all must be scheduled together in the gang, increasing resource requirements.

### PCS Replica-Level MaxSurge with ReplicaRecreate

**Behavior:**

With ReplicaRecreate, surge replicas are created at new indices above the desired replica count to avoid index holes. The update process:

1. Creates surge replicas at indices `[replicas, replicas+maxSurge-1]`
2. Recreates original indices `[0, replicas-1]` with the updated spec
3. Deletes surge replicas once original indices are recreated

**Example:**

With `replicas=3`, `maxSurge=1`, and `maxUnavailable=0`:

1. **Initial state:** Replicas 0, 1, 2 (old spec)
2. **Create surge replica:** Replicas 0, 1, 2 (old), 3 (surge, new spec)
3. **Wait for surge available:** Replica 3 becomes available
4. **Delete and recreate replica 0:** Replicas 0 (new), 1, 2 (old), 3 (surge, new)
5. **Wait for replica 0 available:** Replica 0 becomes available
6. **Delete and recreate replica 1:** Replicas 0, 1 (new), 2 (old), 3 (surge, new)
7. **Wait for replica 1 available:** Replica 1 becomes available
8. **Delete and recreate replica 2:** Replicas 0, 1, 2 (new), 3 (surge, new)
9. **Wait for replica 2 available:** Replica 2 becomes available
10. **Delete surge replica 3:** Replicas 0, 1, 2 (new spec) - no index holes

This approach maintains sequential indices throughout the update, avoiding DNS naming issues and ensuring applications always see consistent replica indices. With `maxUnavailable=0`, a surge replica must be available before deleting any original replica to maintain full capacity.

**Stuck Scenarios with ReplicaRecreate and MaxSurge:**

When using ReplicaRecreate with `maxSurge > 0`, the update can get stuck if surge replicas fail to become available. This can happen in several scenarios:

1. **Surge replica is unscheduled**: The surge replica's pods cannot be scheduled due to insufficient cluster resources, topology constraints that cannot be satisfied, node selectors/affinity mismatches, or resource quotas exceeded.

2. **Surge replica has MinAvailable breached**: The surge replica's pods are scheduled but fail to become ready due to crash loops, health check failures, application startup failures, or dependency issues.

3. **Existing replicas are unhealthy**: Even if surge replica is healthy, if existing replicas are unscheduled or have MinAvailable breached, the update may be blocked by `maxUnavailable` constraints.

Users are responsible for identifying when a rolling update with `maxSurge` during ReplicaRecreate is stuck (e.g., update progress stalls, surge replica remains unscheduled or has MinAvailable breached) and manually intervening to unblock the update, such as by reducing `maxSurge` to 0 or deleting the stuck surge replica.

## Use Case Examples

### Single Node Aggregated (Version Compatibility Assumed)

Single node deployment with version compatibility between components. Frontend uses default update strategy (sequential), while agg worker uses surge to maintain capacity.

```yaml
spec:
  replicas: 1
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 0
  template:
    cliques:
      - name: frontend
        replicas: 2
        # Default: maxUnavailable=1, maxSurge=0 (sequential pod updates)
      - name: agg-worker
        replicas: 3
        updateStrategy:
          maxUnavailable: 0
          maxSurge: 1 # Maintain full capacity during updates
```

### Multi-Node Disaggregated (No Version Compatibility, Excess Capacity)

Multi-node deployment with incompatible versions but excess cluster capacity. Uses ReplicaRecreate with surge to maintain availability.

```yaml
spec:
  replicas: 2
  updateStrategy:
    type: ReplicaRecreate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1 # Create surge replica before deleting old ones
  template:
    podCliqueScalingGroups:
      - name: prefill
      - name: decode
```

### Multi-Node Disaggregated (No Version Compatibility, No Excess Capacity)

Multi-node deployment with incompatible versions and limited cluster capacity. Uses ReplicaRecreate without surge, accepting temporary capacity reduction.

```yaml
spec:
  replicas: 2
  updateStrategy:
    type: ReplicaRecreate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 0 # No surge, delete before create
  template:
    podCliqueScalingGroups:
      - name: prefill
      - name: decode
```

### Multi-Node Aggregated (Version Compatibility Assumed)

Multi-node deployment with version compatibility. Prefill uses sequential updates, while decode uses surge to maintain capacity.

```yaml
spec:
  replicas: 2
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 0
  template:
    podCliqueScalingGroups:
      - name: prefill
        replicas: 3
        updateStrategy:
          maxUnavailable: 1
          maxSurge: 0 # Sequential PCSG replica updates
      - name: decode
        replicas: 2
        updateStrategy:
          maxUnavailable: 0
          maxSurge: 1 # Maintain full capacity during updates
```

## Implementation Phases

### Phase 1: PCS Replica-Level Recreate

- Default is `RollingUpdate` (no PC or PCSG level configurability)
- Can specify `ReplicaRecreate` - no configurability of `maxUnavailable` or `maxSurge`, will just recreate PCS replicas one at a time

### Phase 2: PCS Replica-Level Recreate Configurability

- Can specify `maxUnavailable` and `maxSurge` for PCS replicas

### Phase 3: Component-Level Strategies

- Add `updateStrategy` to `PodCliqueTemplateSpec`
- Add `updateStrategy` to `PodCliqueScalingGroupConfig`
- Support `maxUnavailable` and `maxSurge` for components

## Migration Path

**Existing behavior (no breaking changes):**

- No `updateStrategy` specified → defaults to current behavior
- One PCS replica at a time, one pod at a time
- Equivalent to: `maxUnavailable=1, maxSurge=0` at all levels

**Opt-in improvements:**

- Users explicitly add `updateStrategy` fields to gain new capabilities
- Can migrate incrementally (add to PCS first, then components later)
