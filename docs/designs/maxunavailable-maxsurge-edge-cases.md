# MaxUnavailable/MaxSurge Edge Cases: PodClique and PodCliqueScalingGroup

This document analyzes edge cases and potential stuck scenarios when adding `maxUnavailable` and `maxSurge` support to PodClique and PodCliqueScalingGroup levels.

## Standalone PodClique: MaxSurge Analysis

### Scenario Setup

**Configuration:**

- `replicas=3`
- `maxUnavailable=0`
- `maxSurge=1`
- `minAvailable=2` (PodClique level)

### Expected Behavior

1. **Initial state:** 3 pods (indices 0, 1, 2) in base PodGang
2. **Update starts:** Create surge pod (index 3)
3. **Surge pod added to PodGroup:** PodGroup now has 4 pod references
4. **Wait for surge pod ready:** Once surge pod is ready, can delete old pod
5. **Delete old pod:** Delete pod 0, recreate with new spec
6. **Continue:** Repeat for pods 1, 2
7. **Delete surge pod:** Once all original pods updated

### Edge Cases and Stuck Scenarios

#### 1. **Surge Pod Cannot Be Scheduled**

**Scenario:**

- Surge pod is created but cannot be scheduled (insufficient resources, topology constraints, node selectors)

**Impact:**

- Update is **stuck** - cannot proceed because `maxUnavailable=0` prevents deleting old pods
- PodGroup has 4 pod references but only 3 are scheduled
- Gang scheduling may block if `MinReplicas` (MinAvailable) requires all pods scheduled

**Detection:**

- Surge pod has `ScheduledReplicas=0`
- PodGroup has fewer scheduled pods than total pod references
- Update progress stalls

**Resolution:**

- User must manually delete surge pod or increase cluster capacity
- Consider timeout mechanism to abort surge and continue without it

#### 2. **Surge Pod Scheduled But Not Ready**

**Scenario:**

- Surge pod is scheduled but fails health checks, crash loops, or startup failures

**Impact:**

- Update is **stuck** - cannot delete old pods because surge pod isn't "available"
- With `maxUnavailable=0`, need surge pod to be ready before deleting old ones
- PodGroup `MinReplicas` is about **scheduled** pods, not ready pods, so gang scheduling may proceed
- But availability check (`ReadyReplicas >= MinAvailable`) may fail

**Detection:**

- Surge pod has `ScheduledReplicas=1` but `ReadyReplicas=0`
- PodClique `AvailableReplicas` doesn't increase
- Update progress stalls

**Resolution:**

- Fix application issues causing surge pod to fail
- Consider timeout to abort surge pod and continue without it
- May need to investigate pod logs/events

#### 3. **MinAvailable Constraint During Update**

**Scenario:**

- `minAvailable=3`, `replicas=3`, `maxSurge=1`
- During update: 3 old pods + 1 surge pod = 4 pods total
- Need at least 3 ready pods to maintain availability

**Impact:**

- If surge pod is ready, can delete one old pod (3 old + 1 new = 4, but only 3 need to be ready)
- If surge pod is not ready, cannot delete any old pods (would drop below minAvailable)
- Gang scheduling requires `MinReplicas` (3) scheduled pods, so all 4 pods must be scheduled together

**Consideration:**

- PodGroup `MinReplicas` is checked for **scheduled** pods
- PodClique availability is checked for **ready** pods
- Both constraints must be satisfied

#### 4. **Gang Scheduling Blocking**

**Scenario:**

- Base PodGang contains multiple PodCliques
- Surge pod is added to one PodClique's PodGroup
- Other PodCliques in the same base PodGang are also updating

**Impact:**

- All PodGroups in base PodGang must meet their `MinReplicas` before any pods are scheduled
- If another PodClique is also surging, both surge pods must be scheduled together
- This can create a "surge amplification" effect where multiple PodCliques surging simultaneously increases resource requirements

**Example:**

```
Base PodGang: "my-pcs-0"
  ├─ PodGroup: "my-pcs-0-frontend" (3 old + 1 surge = 4 pods)
  └─ PodGroup: "my-pcs-0-api" (3 old + 1 surge = 4 pods)

Gang scheduling requires:
  - Frontend: MinReplicas=2 scheduled
  - API: MinReplicas=2 scheduled
  - Total: 4 pods must be scheduled together (2 old + 2 surge minimum)
```

#### 5. **Index Management**

**Scenario:**

- Pods use `GenerateName`, so no index collisions
- Surge pod gets a new generated name (not index-based)
- PodGroup references all pods by name

**Impact:**

- No index holes (unlike PCS replica level)
- PodGroup can have more pod references than `replicas` count
- Index tracker naturally handles the extra pod

**No Issues:**

- This is safe - pod names are unique and don't create ordering problems

## PodCliqueScalingGroup: MaxSurge Analysis

### Scenario Setup

**Configuration:**

- `replicas=3`
- `maxUnavailable=0`
- `maxSurge=1`
- `minAvailable=3` (PCSG level)

### Expected Behavior

1. **Initial state:** Replicas 0, 1, 2 in base PodGang
2. **Update starts:** Create surge replica (replica 3)
3. **Surge replica placement:** Replica 3 goes into scaled PodGang (since `replicas >= minAvailable`)
4. **Scaled PodGang gating:** Surge replica's scaled PodGang is gated until base PodGang is ready
5. **Wait for surge available:** Once surge replica is available, can delete old replica
6. **Delete old replica:** Delete replica 0, recreate with new spec
7. **Continue:** Repeat for replicas 1, 2
8. **Delete surge replica:** Once all original replicas updated

### Edge Cases and Stuck Scenarios

#### 1. **Surge Replica Cannot Be Scheduled (Scaled PodGang)**

**Scenario:**

- Surge replica (replica 3) is created but its scaled PodGang cannot be scheduled
- Scaled PodGang is gated by base PodGang readiness

**Impact:**

- Update is **stuck** - cannot proceed because `maxUnavailable=0` prevents deleting old replicas
- Scaled PodGang pods have scheduling gates that won't be removed until base PodGang is ready
- Even if base PodGang is ready, if scaled PodGang can't be scheduled, update is blocked

**Detection:**

- Surge replica's PodCliques have `ScheduledReplicas=0`
- Scaled PodGang exists but pods are not scheduled
- Update progress stalls

**Resolution:**

- User must manually delete surge replica or increase cluster capacity
- Consider timeout mechanism to abort surge

#### 2. **Base PodGang Update Blocks Surge Scaled PodGang**

**Scenario:**

- Base PodGang (replicas 0, 1, 2) is being updated
- Surge replica (replica 3) is in scaled PodGang
- Scaled PodGang is gated by base PodGang readiness

**Impact:**

- If base PodGang is updating, it may not be "ready" (all PodGroups meeting MinReplicas)
- Surge scaled PodGang cannot proceed even if it could be scheduled
- Creates a **circular dependency**: need surge ready to update base, but base must be ready for surge

**Example:**

```
Base PodGang updating:
  - Replica 0: old spec, updating
  - Replica 1: old spec, updating
  - Replica 2: old spec, updating

Surge Scaled PodGang:
  - Replica 3: new spec, gated by base PodGang

Problem: Base PodGang not ready (updating), so surge can't proceed
```

**Resolution:**

- May need to allow surge scaled PodGang to proceed even if base is updating (if base has sufficient ready replicas)
- Or require base PodGang update to complete before creating surge
- Consider relaxing gate requirement during updates

#### 3. **Surge Replica Scheduled But Not Ready**

**Scenario:**

- Surge replica's scaled PodGang is scheduled (gates removed, pods scheduled)
- But surge replica's PodCliques fail to become ready (health checks, crash loops)

**Impact:**

- Update is **stuck** - cannot delete old replicas because surge isn't "available"
- With `maxUnavailable=0`, need surge replica to be available before deleting old ones
- Scaled PodGang readiness is independent of base PodGang once gates are removed

**Detection:**

- Surge replica's PodCliques have `ScheduledReplicas > 0` but `ReadyReplicas < MinAvailable`
- PCSG `AvailableReplicas` doesn't increase
- Update progress stalls

**Resolution:**

- Fix application issues causing surge replica to fail
- Consider timeout to abort surge replica

#### 4. **MinAvailable Constraint During Update**

**Scenario:**

- `minAvailable=3`, `replicas=3`, `maxSurge=1`
- During update: 3 old replicas + 1 surge replica = 4 replicas total
- Need at least 3 available replicas to maintain PCSG availability

**Impact:**

- If surge replica is available, can delete one old replica (3 old + 1 new = 4, but only 3 need to be available)
- If surge replica is not available, cannot delete any old replicas (would drop below minAvailable)
- Base PodGang requires all replicas 0-2 to meet MinReplicas for gang scheduling

**Consideration:**

- PCSG availability is checked at replica level (all PodCliques in replica must be available)
- Base PodGang gang scheduling requires all PodGroups to meet MinReplicas
- Both constraints must be satisfied

#### 5. **Surge Replica Placement with minAvailable < replicas**

**Scenario:**

- `minAvailable=2`, `replicas=3`, `maxSurge=1`
- Base PodGang: replicas 0, 1
- Scaled PodGangs: replica 2 (existing), replica 3 (surge)

**Impact:**

- Surge replica (replica 3) goes into scaled PodGang
- Both scaled PodGangs (replica 2 and 3) are gated by base PodGang
- If base PodGang is updating, both scaled PodGangs are blocked
- Surge replica must wait for base PodGang to be ready, then its own scaled PodGang to be scheduled and ready

**Complexity:**

- Multiple scaled PodGangs can be affected by base PodGang updates
- Surge scaled PodGang has additional dependency chain: base ready → scaled scheduled → scaled ready

#### 6. **Gang Scheduling Amplification**

**Scenario:**

- Multiple PodCliques in PCSG (e.g., leader + worker)
- Surge replica creates surge PodCliques for all CliqueNames
- All surge PodCliques must be scheduled together in scaled PodGang

**Impact:**

- Scaled PodGang requires all PodGroups (one per CliqueName) to meet MinReplicas
- If one PodClique can't schedule, entire scaled PodGang is blocked
- This is expected gang scheduling behavior, but can cause stuck updates if surge PodClique can't be scheduled

**Example:**

```
Surge Scaled PodGang: "my-pcs-0-prefill-0" (replica 3)
  ├─ PodGroup: "my-pcs-0-prefill-3-leader" (MinReplicas=1)
  └─ PodGroup: "my-pcs-0-prefill-3-worker" (MinReplicas=2)

If leader can't schedule → entire gang blocked
If worker can't schedule → entire gang blocked
```

## Common Stuck Patterns

### Pattern 1: Surge Resource Unavailable

**Symptoms:**

- Surge pod/replica created but not scheduled or not ready
- Update progress stalls
- `maxUnavailable=0` prevents deleting old resources

**Resolution Options:**

1. Manual intervention: Delete surge resource
2. Timeout mechanism: Abort surge after timeout
3. Capacity increase: Add cluster resources
4. Fix application: Resolve health check/startup issues

### Pattern 2: Gang Scheduling Blocking

**Symptoms:**

- Surge resource cannot be scheduled due to gang constraints
- Other PodGroups in same gang also need resources
- Gang requires all PodGroups to meet MinReplicas

**Resolution Options:**

1. Increase cluster capacity to satisfy entire gang
2. Reduce MinAvailable to lower gang requirements
3. Temporarily disable surge (set maxSurge=0)

### Pattern 3: Dependency Chain Blocking

**Symptoms:**

- Scaled PodGang surge replica gated by base PodGang
- Base PodGang is updating, not "ready"
- Circular dependency: need surge to update base, need base ready for surge

**Resolution Options:**

1. Complete base PodGang update first, then create surge
2. Relax gate requirements during updates
3. Allow surge if base has sufficient ready replicas (even if updating)

## Recommendations

### For Standalone PodClique MaxSurge

1. **Timeout mechanism:** Implement timeout to abort surge pods that can't be scheduled/ready
2. **Gang scheduling awareness:** Consider gang constraints when creating surge pods
3. **Readiness vs Scheduling:** Distinguish between scheduled (for gang) and ready (for availability)
4. **MinAvailable interaction:** Ensure surge pods count toward availability before deleting old ones

### For PodCliqueScalingGroup MaxSurge

1. **Base PodGang dependency:** Handle case where base PodGang is updating when surge is created
2. **Scaled PodGang gating:** Consider relaxing gate requirements during updates if base has sufficient ready replicas
3. **Timeout mechanism:** Implement timeout to abort surge replicas that can't be scheduled/ready
4. **Gang scheduling amplification:** Account for multiple PodCliques in surge replica requiring gang scheduling

### General Recommendations

1. **User responsibility:** Users must monitor update progress and intervene if stuck
2. **Start conservative:** Begin with `maxSurge=0` and only enable when confident in cluster capacity
3. **Capacity planning:** Ensure cluster has sufficient resources before enabling surge
4. **Monitoring:** Track surge resource status (scheduled, ready) to detect stuck scenarios early
