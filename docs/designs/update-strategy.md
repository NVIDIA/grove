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

## Non-Goals

## Architecture

### Three-Level Update Control

```
PodCliqueSet (Top Level)
├─ UpdateStrategy (controls PCS replica updates)
│  ├─ RollingUpdate: one replica at a time
│  ├─ ReplicaRecreate: tear down entire replica
│  └─ OnDelete: manual updates only
│
├─ PCS Replica 0
│  ├─ Standalone PodCliques (concurrent updates)
│  │  ├─ frontend: UpdateStrategy (controls pod updates)
│  │  └─ backend: UpdateStrategy (controls pod updates)
│  │
│  └─ PodCliqueScalingGroups (concurrent updates)
│     ├─ prefill: UpdateStrategy (controls PCSG replica updates)
│     └─ decode: UpdateStrategy (controls PCSG replica updates)
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
    OnDelete         // Manual updates only
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

**PCS Replica Level:**

- Updates one PCS replica at a time (by default)
- Each replica must complete before the next starts
- Priority: unscheduled → unhealthy → ascending ordinal

**Within Each PCS Replica:**

- **Standalone PodCliques**: Update concurrently (all at once)
- **PCSGs**: Update concurrently with each other
- Each uses its own `updateStrategy` configuration

**Within Each PodClique:**

- Pods update one at a time (by default)
- Can configure `maxSurge=1` for zero-downtime updates

**Within Each PCSG:**

- PCSG replicas update sequentially (by default)
- Each PCSG replica = all member PodCliques deleted/recreated together

### ReplicaRecreate

**Behavior:**

- Deletes **entire PCS replica** (all PCSGs + standalone PCs)
- All components within the replica are deleted simultaneously
- Recreates all components together
- **Bypasses** individual component update strategies

**Use Cases:**

- Need to clear all state at once
- Faster updates when downtime is acceptable
- Coordinated recreation of interdependent components

### OnDelete

**Behavior:**

- No automatic updates
- Replicas only update when manually deleted
- Once deleted, recreates with new spec
- Component strategies apply during recreation

**Use Cases:**

- Manual rollout control
- Testing/staging environments
- Stateful workloads requiring careful migration

## MaxSurge Considerations

### Pod-Level MaxSurge (Safe)

**Works well** because:

- Pod indices can exceed replica count temporarily
- Pod names use `GenerateName` (no collisions)
- Index holes are filled naturally by index tracker
- Example: `replicas=10`, during surge can have pod with index 11

### PCS Replica-Level MaxSurge (Complex)

**Challenges:**

- Creates temporary replicas with indices `[replicas, replicas+maxSurge-1]`
- DNS names become non-sequential during updates
- Example: `replicas=3`, surge creates replica 3, indices become [1, 2, 3] after deleting 0
- **Index holes occur** during rolling updates
- Applications must tolerate non-sequential replica indices

**Gang Scheduling:**

- ✅ Each PodGang is independent (no cross-replica dependencies)
- ✅ Surge replica can be gang-scheduled if cluster has capacity
- ❌ But naming/DNS challenges remain

**Recommendation:** Start with `maxUnavailable` only, add `maxSurge` support later if needed.

## Use Case Examples

### High-Availability Inference Service

```yaml
spec:
  replicas: 3
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 0
  template:
    cliques:
      - name: frontend
        updateStrategy:
          maxUnavailable: 0
          maxSurge: 1 # Zero-downtime frontend updates
```

### Batch Processing (Fast Updates)

```yaml
spec:
  replicas: 5
  updateStrategy:
    type: ReplicaRecreate # Tear down entire replica at once
  template:
    cliques:
      - name: worker
        # Component strategies ignored with ReplicaRecreate
```

### Database Cluster (Manual Control)

```yaml
spec:
  replicas: 3
  updateStrategy:
    type: OnDelete # Only update when manually deleted
  template:
    cliques:
      - name: postgres
        updateStrategy:
          maxUnavailable: 0
          maxSurge: 1 # When triggered, do zero-downtime
```

### Multi-Tier Service (Mixed Strategies)

```yaml
spec:
  replicas: 2
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  template:
    cliques:
      - name: api
        updateStrategy:
          maxUnavailable: 0
          maxSurge: 1 # API needs zero downtime
      - name: cache
        updateStrategy:
          maxUnavailable: 2 # Cache can update faster
    podCliqueScalingGroups:
      - name: workers
        updateStrategy:
          maxUnavailable: 3 # Can update multiple worker replicas
```

## Implementation Phases

### Phase 1: PCS Replica-Level Strategies

- Implement `RollingUpdate`, `ReplicaRecreate`, `OnDelete`
- Support `maxUnavailable` only (no `maxSurge` yet)
- Default behavior: one replica at a time

### Phase 2: Component-Level Strategies

- Add `updateStrategy` to `PodCliqueTemplateSpec`
- Add `updateStrategy` to `PodCliqueScalingGroupConfig`
- Support `maxUnavailable` and `maxSurge` for components

### Phase 3: Advanced Features (Optional)

- PCS replica-level `maxSurge` support
- Partition-based rolling updates
- Canary deployments

## Migration Path

**Existing behavior (no breaking changes):**

- No `updateStrategy` specified → defaults to current behavior
- One PCS replica at a time, one pod at a time
- Equivalent to: `maxUnavailable=1, maxSurge=0` at all levels

**Opt-in improvements:**

- Users explicitly add `updateStrategy` fields to gain new capabilities
- Can migrate incrementally (add to PCS first, then components later)
