# PodCliqueSet Rolling Update Interruption Behavior

This document explains what happens when a PodCliqueSet is patched (spec changes) while a rolling update is already in progress.

## Overview

When a new spec change is detected during an ongoing rolling update, the operator **completely resets and restarts** the rolling update. All progress from the previous update is discarded, and the update starts fresh with the new target generation hash.

## Behavior: Complete Reset and Restart

### Step 1: Generation Hash Change Detection

When `processGenerationHashChange` runs and detects a new generation hash:

```go
// operator/internal/controller/podcliqueset/reconcilespec.go:93-97
if newGenerationHash != *pcs.Status.CurrentGenerationHash {
    // trigger rolling update by setting or overriding pcs.Status.RollingUpdateProgress.
    if err := r.initRollingUpdateProgress(ctx, pcs, pcsObjectName, newGenerationHash); err != nil {
        return ctrlcommon.ReconcileWithErrors(...)
    }
}
```

### Step 2: Rolling Update Progress Reset

`initRollingUpdateProgress` **completely replaces** the existing `RollingUpdateProgress`:

```go
// operator/internal/controller/podcliqueset/reconcilespec.go:136-141
func (r *Reconciler) initRollingUpdateProgress(...) error {
    pcs.Status.RollingUpdateProgress = &grovecorev1alpha1.PodCliqueSetRollingUpdateProgress{
        UpdateStartedAt: metav1.Now(),  // NEW timestamp
    }
    pcs.Status.UpdatedReplicas = 0  // RESET to 0
    pcs.Status.CurrentGenerationHash = &newGenerationHash  // NEW hash
    // ...
}
```

**What gets reset:**
- ✅ `RollingUpdateProgress.UpdateStartedAt` → New timestamp
- ✅ `RollingUpdateProgress.UpdateEndedAt` → Set to `null`
- ✅ `RollingUpdateProgress.UpdatedPodCliques` → **Lost** (empty list)
- ✅ `RollingUpdateProgress.UpdatedPodCliqueScalingGroups` → **Lost** (empty list)
- ✅ `RollingUpdateProgress.CurrentlyUpdating` → **Cleared** (set to `null`)
- ✅ `UpdatedReplicas` → Reset to `0`
- ✅ `CurrentGenerationHash` → Updated to new hash

### Step 3: Component-Level Reset

Each component (PodClique and PodCliqueScalingGroup) controller detects the generation hash mismatch and resets its own rolling update progress.

#### PodClique Reset Logic

```go
// operator/internal/controller/podclique/reconcilespec.go:128-145
func shouldResetOrTriggerRollingUpdate(pcs, pclq) bool {
    // If PCLQ's RollingUpdateProgress.PodCliqueSetGenerationHash != PCS.CurrentGenerationHash
    // Then reset is needed
    inProgressPCLQUpdateNotStale := 
        IsPCLQUpdateInProgress(pclq) && 
        pclq.Status.RollingUpdateProgress.PodCliqueSetGenerationHash == *pcs.Status.CurrentGenerationHash
    
    // If update is for different hash, it's stale → reset needed
    if !inProgressPCLQUpdateNotStale {
        return true  // Reset required
    }
    return false
}
```

When reset is triggered:
- `PodClique.Status.UpdatedReplicas` → Reset to `0`
- `PodClique.Status.RollingUpdateProgress` → Recreated with new `PodCliqueSetGenerationHash`

#### PodCliqueScalingGroup Reset Logic

Similar logic applies to PCSGs:

```go
// operator/internal/controller/podcliquescalinggroup/reconcilespec.go:110-116
func shouldResetOrTriggerRollingUpdate(pcs, pcsg) bool {
    // If PCSG's RollingUpdateProgress.PodCliqueSetGenerationHash != PCS.CurrentGenerationHash
    // Then reset is needed
    if pcsg.Status.RollingUpdateProgress != nil && 
       pcsg.Status.RollingUpdateProgress.PodCliqueSetGenerationHash == *pcs.Status.CurrentGenerationHash {
        return false  // Already up-to-date
    }
    return true  // Reset required
}
```

### Step 4: Replica Re-evaluation

When `computePendingUpdateWork` runs, it re-evaluates all replicas against the **new** generation hash:

```go
// operator/internal/controller/podcliqueset/components/podcliquesetreplica/rollingupdate.go:253-267
func (pri *pcsReplicaInfo) computeUpdateProgress(pcs) {
    for _, pclq := range pri.pclqs {
        // Checks against NEW CurrentGenerationHash
        if isPCLQUpdateComplete(&pclq, *pcs.Status.CurrentGenerationHash) {
            progress.updatedPCLQFQNs = append(...)
        }
    }
    // ...
}
```

**Result:**
- Replicas that were updated to the **old** generation hash are now considered **outdated**
- They're added back to the `pendingUpdateReplicaInfos` list
- The update starts from the beginning

## Example Scenario

### Initial State
- PCS has 3 replicas
- Rolling update in progress to generation hash `hash-v1`
- Replica 0: ✅ Updated to `hash-v1`
- Replica 1: 🔄 Currently updating to `hash-v1`
- Replica 2: ⏳ Pending update to `hash-v1`

**Status:**
```yaml
status:
  currentGenerationHash: "hash-v1"
  updatedReplicas: 1
  rollingUpdateProgress:
    updateStartedAt: "2025-01-15T10:00:00Z"
    updateEndedAt: null
    updatedPodCliques:
      - "my-pcs-0-frontend"
    updatedPodCliqueScalingGroups:
      - "my-pcs-0-prefill"
      - "my-pcs-0-decode"
    currentlyUpdating:
      replicaIndex: 1
```

### New Patch Applied

User patches the PCS spec (e.g., changes image tag), generating new hash `hash-v2`.

### After Reset

**Status immediately after reset:**
```yaml
status:
  currentGenerationHash: "hash-v2"  # NEW hash
  updatedReplicas: 0  # RESET to 0
  rollingUpdateProgress:
    updateStartedAt: "2025-01-15T10:05:00Z"  # NEW timestamp
    updateEndedAt: null
    updatedPodCliques: []  # EMPTY - all progress lost
    updatedPodCliqueScalingGroups: []  # EMPTY - all progress lost
    currentlyUpdating: null  # CLEARED
```

### Replica Re-evaluation

All replicas are re-evaluated:
- Replica 0: Now considered **outdated** (has `hash-v1`, needs `hash-v2`)
- Replica 1: Now considered **outdated** (has `hash-v1`, needs `hash-v2`)
- Replica 2: Already outdated (needs `hash-v2`)

**All 3 replicas** are now in the pending update list and will be updated again.

### New Update Sequence

The update restarts from the beginning:
1. Replica 0 selected again (priority: unscheduled → unhealthy → ascending ordinal)
2. Replica 0 updated to `hash-v2`
3. Replica 1 updated to `hash-v2`
4. Replica 2 updated to `hash-v2`

## Key Implications

### 1. **No Incremental Progress**
- Progress from the interrupted update is **completely lost**
- Even replicas that were already updated need to be updated again
- No "resume from where we left off" behavior

### 2. **Potential for Churn**
If multiple patches are applied in quick succession:
- Each patch triggers a complete reset
- Replicas may be updated multiple times unnecessarily
- Example: Replica 0 updated to `hash-v1`, then immediately reset and updated to `hash-v2`, then reset again to `hash-v3`

### 3. **Component-Level Consistency**
- All components (PCLQs and PCSGs) reset their progress independently
- They check their `RollingUpdateProgress.PodCliqueSetGenerationHash` against PCS's `CurrentGenerationHash`
- Mismatch triggers reset

### 4. **Update Priority Re-evaluation**
- Replica selection priority is re-evaluated from scratch
- Unscheduled replicas still get highest priority
- MinAvailable-breached replicas get second priority
- Then ascending ordinal order

## Code Flow Summary

```
1. User patches PCS spec
   ↓
2. processGenerationHashChange() detects new hash
   ↓
3. initRollingUpdateProgress() resets PCS status:
   - New RollingUpdateProgress (empty lists)
   - UpdatedReplicas = 0
   - New CurrentGenerationHash
   ↓
4. Component controllers (PCLQ/PCSG) detect hash mismatch
   ↓
5. Component controllers reset their RollingUpdateProgress
   ↓
6. computePendingUpdateWork() re-evaluates all replicas
   ↓
7. All replicas (even previously updated ones) marked as outdated
   ↓
8. Update restarts from beginning with new target hash
```

## Design Rationale

This "reset and restart" approach ensures:

1. **Consistency**: All replicas eventually converge to the same target (the latest spec)
2. **Simplicity**: No need to track multiple generation hashes or partial updates
3. **Correctness**: Avoids complex state management of "which replica is at which version"

However, it can be inefficient if:
- Updates are interrupted frequently
- Updates take a long time
- Users apply multiple patches in quick succession

## Future Considerations

Potential improvements could include:
- **Batching**: Wait for current update to complete before starting new one
- **Incremental updates**: Track which replicas are at which version and only update what's needed
- **Update queue**: Queue pending updates instead of resetting

For now, the current behavior prioritizes correctness and simplicity over efficiency in edge cases.

