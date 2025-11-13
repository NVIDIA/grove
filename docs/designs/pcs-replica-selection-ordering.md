# PodCliqueSet Replica Selection and Ordering for Rolling Updates

This document explains how PCS replicas are selected and ordered for rolling updates.

## Overview

The replica selection process follows a **three-tier priority system** to determine which replica should be updated next. The selection prioritizes:

1. **Unscheduled replicas** (highest priority)
2. **Unhealthy replicas** (MinAvailable breached, but not yet terminated)
3. **Healthy replicas** (lowest priority, ascending ordinal order)

## Selection Process Flow

### Step 1: Filter Replicas That Need Updates

Before ordering, the controller filters replicas to only consider those that need updating:

```go
// operator/internal/controller/podcliqueset/components/podcliquesetreplica/rollingupdate.go:75-96
func computePendingUpdateWork(...) {
    for _, replicaInfo := range replicaInfos {
        replicaInfo.computeUpdateProgress(pcs)

        // Skip the currently updating replica (tracked separately)
        if pcs.Status.RollingUpdateProgress.CurrentlyUpdating != nil &&
           pcs.Status.RollingUpdateProgress.CurrentlyUpdating.ReplicaIndex == replicaInfo.replicaIndex {
            pendingWork.currentlyUpdatingReplicaInfo = &replicaInfo
            continue
        }

        // Only include replicas that are NOT fully updated
        if !replicaInfo.updateProgress.done {
            pendingWork.pendingUpdateReplicaInfos = append(...)
        }
    }
}
```

**What gets filtered:**

- ✅ **Currently updating replica**: Tracked separately, not in pending list
- ✅ **Fully updated replicas**: Excluded (already at target generation hash)
- ✅ **Replicas marked for termination**: Excluded (handled by deletion flow)
- ✅ **Replicas needing update**: Included in `pendingUpdateReplicaInfos`

### Step 2: Calculate Scheduled Pods Count

For each replica, the controller calculates how many pods are scheduled:

```go
// operator/internal/controller/podcliqueset/components/podcliquesetreplica/rollingupdate.go:270-286
func (pri *pcsReplicaInfo) getNumScheduledPods(pcs *PodCliqueSet) int {
    noScheduled := 0

    // Count scheduled pods from standalone PodCliques
    for _, pclq := range pri.pclqs {
        noScheduled += int(pclq.Status.ScheduledReplicas)
    }

    // Count scheduled pods from PodCliqueScalingGroups
    for _, pcsg := range pri.pcsgs {
        for _, cliqueName := range pcsg.Spec.CliqueNames {
            pclqTemplateSpec := findTemplateSpec(...)
            // For PCSG: ScheduledReplicas * MinAvailable per clique
            noScheduled += int(pcsg.Status.ScheduledReplicas * *pclqTemplateSpec.Spec.MinAvailable)
        }
    }

    return noScheduled
}
```

**Note:** For PCSGs, the calculation multiplies `ScheduledReplicas` by `MinAvailable` because each PCSG replica contains multiple PodCliques (one per `CliqueName`), and we count the minimum required pods per clique.

### Step 3: Order Replicas by Priority

The `orderPCSReplicaInfo` function implements a three-tier priority system:

```go
// operator/internal/controller/podcliqueset/components/podcliquesetreplica/rollingupdate.go:195-223
func orderPCSReplicaInfo(pcs, minAvailableBreachedPCSReplicaIndices) func(a, b) int {
    return func(a, b pcsReplicaInfo) int {
        scheduledPodsInA, scheduledPodsInB := a.getNumScheduledPods(pcs), b.getNumScheduledPods(pcs)

        // PRIORITY 1: Unscheduled replicas (highest priority)
        if scheduledPodsInA == 0 && scheduledPodsInB != 0 {
            return -1  // a comes before b
        } else if scheduledPodsInA != 0 && scheduledPodsInB == 0 {
            return 1   // b comes before a
        }

        // PRIORITY 2: MinAvailable breached (but termination delay not expired)
        minAvailableBreachedForA := slices.Contains(minAvailableBreachedPCSReplicaIndices, a.replicaIndex)
        minAvailableBreachedForB := slices.Contains(minAvailableBreachedPCSReplicaIndices, b.replicaIndex)
        if minAvailableBreachedForA && !minAvailableBreachedForB {
            return -1  // a comes before b
        } else if !minAvailableBreachedForA && minAvailableBreachedForB {
            return 1   // b comes before a
        }

        // PRIORITY 3: Healthy replicas (ascending ordinal order)
        if a.replicaIndex < b.replicaIndex {
            return -1  // a comes before b
        } else {
            return 1   // b comes before a
        }
    }
}
```

### Step 4: Select Next Replica

After sorting, the first replica in the list is selected:

```go
// operator/internal/controller/podcliqueset/components/podcliquesetreplica/rollingupdate.go:243-250
func (w *pendingUpdateWork) getNextReplicaToUpdate(...) *int {
    slices.SortFunc(w.pendingUpdateReplicaInfos, orderPCSReplicaInfo(...))
    if len(w.pendingUpdateReplicaInfos) > 0 {
        return &w.pendingUpdateReplicaInfos[0].replicaIndex  // First in sorted list
    }
    return nil  // No replicas need updating
}
```

## Priority Tiers Explained

### Priority 1: Unscheduled Replicas (Highest)

**Condition:** `getNumScheduledPods() == 0`

**Rationale:**

- Replicas with no scheduled pods are likely in a bad state (gang scheduling failed, pods pending, etc.)
- Updating them first minimizes impact since they're not serving traffic
- These replicas are essentially "dead weight" and should be refreshed

**Example:**

- Replica 0: 0 scheduled pods → **Selected first**
- Replica 1: 5 scheduled pods → Not selected
- Replica 2: 3 scheduled pods → Not selected

### Priority 2: MinAvailable Breached (Medium)

**Condition:** Replica has components with `MinAvailableBreached=True` but `TerminationDelay` has not expired

**Rationale:**

- These replicas are unhealthy but not yet terminated
- Updating them may restore health
- Replicas where `TerminationDelay` has expired are handled separately (deleted before rolling update starts)

**Note:** Replicas with expired `TerminationDelay` are excluded from the update flow entirely - they're deleted via the gang termination mechanism before rolling updates begin.

**Example:**

- Replica 0: Healthy → Not selected
- Replica 1: MinAvailable breached (delay not expired) → **Selected**
- Replica 2: Healthy → Not selected

### Priority 3: Healthy Replicas (Lowest)

**Condition:** All replicas are healthy and have scheduled pods

**Ordering:** Ascending ordinal (0, 1, 2, 3, ...)

**Rationale:**

- For healthy replicas, use predictable, deterministic ordering
- Ascending order provides consistency and predictability
- Lower indices are updated first, which is intuitive

**Example:**

- Replica 0: Healthy → **Selected first**
- Replica 1: Healthy → Selected second
- Replica 2: Healthy → Selected third

## Complete Example Scenarios

### Scenario 1: Mixed States

**Initial State:**

- Replica 0: 0 scheduled pods (unscheduled)
- Replica 1: 5 scheduled pods, MinAvailable breached
- Replica 2: 3 scheduled pods, healthy

**Selection Order:**

1. **Replica 0** (Priority 1: unscheduled)
2. **Replica 1** (Priority 2: MinAvailable breached)
3. **Replica 2** (Priority 3: healthy, ascending ordinal)

### Scenario 2: All Healthy

**Initial State:**

- Replica 0: 5 scheduled pods, healthy
- Replica 1: 5 scheduled pods, healthy
- Replica 2: 5 scheduled pods, healthy

**Selection Order:**

1. **Replica 0** (Priority 3: lowest index)
2. **Replica 1** (Priority 3: middle index)
3. **Replica 2** (Priority 3: highest index)

### Scenario 3: Multiple Unscheduled

**Initial State:**

- Replica 0: 0 scheduled pods
- Replica 1: 0 scheduled pods
- Replica 2: 5 scheduled pods, healthy

**Selection Order:**

1. **Replica 0** (Priority 1: unscheduled, then ascending ordinal)
2. **Replica 1** (Priority 1: unscheduled, then ascending ordinal)
3. **Replica 2** (Priority 3: healthy)

**Note:** When multiple replicas have the same priority, they're sub-sorted by ascending ordinal.

### Scenario 4: With Currently Updating Replica

**Initial State:**

- Replica 0: Currently updating (in `RollingUpdateProgress.CurrentlyUpdating`)
- Replica 1: 5 scheduled pods, healthy
- Replica 2: 3 scheduled pods, healthy

**Selection:**

- Replica 0: **Excluded** from pending list (tracked separately)
- **Replica 1** selected next (Priority 3: healthy, ascending ordinal)
- Replica 2: Will be selected after Replica 1 completes

## Update Progress Tracking

A replica is considered "done" (fully updated) when:

```go
// operator/internal/controller/podcliqueset/components/podcliquesetreplica/rollingupdate.go:252-268
func (pri *pcsReplicaInfo) computeUpdateProgress(pcs) {
    // Check all standalone PodCliques are updated
    for _, pclq := range pri.pclqs {
        if isPCLQUpdateComplete(&pclq, currentGenerationHash) {
            progress.updatedPCLQFQNs = append(...)
        }
    }

    // Check all PodCliqueScalingGroups are updated
    for _, pcsg := range pri.pcsgs {
        if IsPCSGUpdateComplete(&pcsg, currentGenerationHash) {
            progress.updatedPCSGFQNs = append(...)
        }
    }

    // Replica is done when ALL components are updated
    progress.done =
        len(updatedPCLQFQNs) == expectedStandalonePCLQs &&
        len(updatedPCSGFQNs) == expectedPCSGs
}
```

A replica is only removed from the pending list when **all** its components (standalone PCLQs and PCSGs) are updated to the target generation hash.

## Key Design Decisions

### 1. **Unscheduled First**

- Prioritizes problematic replicas that aren't serving traffic
- Minimizes impact on service availability
- Helps recover from scheduling failures

### 2. **Unhealthy Before Healthy**

- Attempts to restore unhealthy replicas through updates
- May resolve issues that caused MinAvailable breaches
- Prevents unhealthy replicas from lingering

### 3. **Deterministic Ordering**

- Ascending ordinal provides predictable behavior
- Makes debugging and monitoring easier
- Ensures consistent update patterns

### 4. **Exclude Terminating Replicas**

- Replicas with expired `TerminationDelay` are deleted, not updated
- Prevents wasted effort updating replicas that will be terminated
- Clean separation between termination and update flows

## Code References

- **Replica filtering**: `computePendingUpdateWork()` (rollingupdate.go:75-96)
- **Scheduled pods calculation**: `getNumScheduledPods()` (rollingupdate.go:270-286)
- **Priority ordering**: `orderPCSReplicaInfo()` (rollingupdate.go:195-223)
- **Replica selection**: `getNextReplicaToUpdate()` (rollingupdate.go:243-250)
- **Update progress**: `computeUpdateProgress()` (rollingupdate.go:252-268)
