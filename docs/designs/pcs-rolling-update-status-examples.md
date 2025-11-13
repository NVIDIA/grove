# PodCliqueSet Rolling Update Status Examples

This document shows what the PodCliqueSet status looks like during different phases of a rolling update.

## Status Structure Overview

The key fields in `PodCliqueSetStatus` for tracking rolling updates are:

- `CurrentGenerationHash`: The target generation hash for the update
- `Replicas`: Total number of PCS replicas (from spec)
- `AvailableReplicas`: Number of replicas that are available
- `UpdatedReplicas`: Number of replicas that have been updated to the target generation hash
- `RollingUpdateProgress`: Detailed progress tracking (only present during updates)

## State 1: No Rolling Update (Steady State)

When there's no rolling update in progress, `RollingUpdateProgress` is `nil` or has `UpdateEndedAt` set.

```yaml
status:
  observedGeneration: 42
  replicas: 3
  availableReplicas: 3
  updatedReplicas: 3
  currentGenerationHash: "abc123def456"
  # RollingUpdateProgress is either nil or completed:
  rollingUpdateProgress:
    updateStartedAt: "2025-01-15T10:00:00Z"
    updateEndedAt: "2025-01-15T10:15:00Z" # Update completed
    updatedPodCliques:
      - "my-pcs-0-frontend"
      - "my-pcs-1-frontend"
      - "my-pcs-2-frontend"
    updatedPodCliqueScalingGroups:
      - "my-pcs-0-prefill"
      - "my-pcs-0-decode"
      - "my-pcs-1-prefill"
      - "my-pcs-1-decode"
      - "my-pcs-2-prefill"
      - "my-pcs-2-decode"
    # CurrentlyUpdating is nil when update is complete
    currentlyUpdating: null
```

**Key indicators:**

- `UpdatedReplicas == Replicas` (all replicas updated)
- `UpdateEndedAt` is set (update completed)
- `CurrentlyUpdating` is `null`

## State 2: Rolling Update Just Started

When a rolling update is triggered (generation hash changes), the status is initialized:

```yaml
status:
  observedGeneration: 43
  replicas: 3
  availableReplicas: 3
  updatedReplicas: 0 # Reset to 0 when update starts
  currentGenerationHash: "xyz789ghi012" # New generation hash
  rollingUpdateProgress:
    updateStartedAt: "2025-01-15T11:00:00Z"
    updateEndedAt: null # null means update is in progress
    updatedPodCliques: [] # Empty initially
    updatedPodCliqueScalingGroups: [] # Empty initially
    currentlyUpdating: null # No replica selected yet
```

**Key indicators:**

- `UpdatedReplicas` is reset to `0`
- `CurrentGenerationHash` has changed
- `UpdateEndedAt` is `null` (update in progress)
- `CurrentlyUpdating` is `null` (first replica not yet selected)

## State 3: Rolling Update In Progress - First Replica Selected

Once the controller selects the first replica to update:

```yaml
status:
  observedGeneration: 43
  replicas: 3
  availableReplicas: 3 # Still 3, replica 0 is updating but not yet deleted
  updatedReplicas: 0 # No replicas fully updated yet
  currentGenerationHash: "xyz789ghi012"
  rollingUpdateProgress:
    updateStartedAt: "2025-01-15T11:00:00Z"
    updateEndedAt: null
    updatedPodCliques: [] # Still empty, update in progress
    updatedPodCliqueScalingGroups: [] # Still empty
    currentlyUpdating:
      replicaIndex: 0 # Replica 0 is being updated
      updateStartedAt: "2025-01-15T11:00:05Z"
```

**Key indicators:**

- `CurrentlyUpdating` is set with `replicaIndex: 0`
- `UpdatedPodCliques` and `UpdatedPodCliqueScalingGroups` are still empty
- `UpdatedReplicas` is still `0`

## State 4: Rolling Update In Progress - First Replica Partially Updated

As components within replica 0 get updated, they're tracked:

```yaml
status:
  observedGeneration: 43
  replicas: 3
  availableReplicas: 2 # Replica 0 may be temporarily unavailable during update
  updatedReplicas: 0 # Replica 0 not fully updated yet
  currentGenerationHash: "xyz789ghi012"
  rollingUpdateProgress:
    updateStartedAt: "2025-01-15T11:00:00Z"
    updateEndedAt: null
    updatedPodCliques:
      - "my-pcs-0-frontend" # Frontend updated
    updatedPodCliqueScalingGroups:
      - "my-pcs-0-prefill" # Prefill PCSG updated
      # decode PCSG still updating
    currentlyUpdating:
      replicaIndex: 0
      updateStartedAt: "2025-01-15T11:00:05Z"
```

**Key indicators:**

- Some components appear in `UpdatedPodCliques` or `UpdatedPodCliqueScalingGroups`
- `CurrentlyUpdating` still points to replica 0
- `UpdatedReplicas` is still `0` (replica not fully updated)

## State 5: Rolling Update In Progress - First Replica Complete, Second Selected

Once replica 0 is fully updated, the controller moves to replica 1:

```yaml
status:
  observedGeneration: 43
  replicas: 3
  availableReplicas: 3 # Replica 0 is back to available
  updatedReplicas: 1 # Replica 0 is now updated
  currentGenerationHash: "xyz789ghi012"
  rollingUpdateProgress:
    updateStartedAt: "2025-01-15T11:00:00Z"
    updateEndedAt: null
    updatedPodCliques:
      - "my-pcs-0-frontend" # All components from replica 0
    updatedPodCliqueScalingGroups:
      - "my-pcs-0-prefill"
      - "my-pcs-0-decode"
    currentlyUpdating:
      replicaIndex: 1 # Now updating replica 1
      updateStartedAt: "2025-01-15T11:05:00Z"
```

**Key indicators:**

- `UpdatedReplicas` increased to `1`
- `CurrentlyUpdating.replicaIndex` changed to `1`
- All components from replica 0 are in the updated lists

## State 6: Rolling Update In Progress - Multiple Replicas Updated

As more replicas complete:

```yaml
status:
  observedGeneration: 43
  replicas: 3
  availableReplicas: 3
  updatedReplicas: 2 # Replicas 0 and 1 are updated
  currentGenerationHash: "xyz789ghi012"
  rollingUpdateProgress:
    updateStartedAt: "2025-01-15T11:00:00Z"
    updateEndedAt: null
    updatedPodCliques:
      - "my-pcs-0-frontend"
      - "my-pcs-1-frontend"
    updatedPodCliqueScalingGroups:
      - "my-pcs-0-prefill"
      - "my-pcs-0-decode"
      - "my-pcs-1-prefill"
      - "my-pcs-1-decode"
    currentlyUpdating:
      replicaIndex: 2 # Now updating replica 2 (last one)
      updateStartedAt: "2025-01-15T11:10:00Z"
```

**Key indicators:**

- `UpdatedReplicas` is `2` (out of 3)
- `CurrentlyUpdating.replicaIndex` is `2` (last replica)
- Components from replicas 0 and 1 are in updated lists

## State 7: Rolling Update Completed

When all replicas are updated, the update is marked complete:

```yaml
status:
  observedGeneration: 43
  replicas: 3
  availableReplicas: 3
  updatedReplicas: 3 # All replicas updated
  currentGenerationHash: "xyz789ghi012"
  rollingUpdateProgress:
    updateStartedAt: "2025-01-15T11:00:00Z"
    updateEndedAt: "2025-01-15T11:15:00Z" # Update completed!
    updatedPodCliques:
      - "my-pcs-0-frontend"
      - "my-pcs-1-frontend"
      - "my-pcs-2-frontend"
    updatedPodCliqueScalingGroups:
      - "my-pcs-0-prefill"
      - "my-pcs-0-decode"
      - "my-pcs-1-prefill"
      - "my-pcs-1-decode"
      - "my-pcs-2-prefill"
      - "my-pcs-2-decode"
    currentlyUpdating: null # No replica being updated
```

**Key indicators:**

- `UpdatedReplicas == Replicas` (all updated)
- `UpdateEndedAt` is set (not null)
- `CurrentlyUpdating` is `null`
- All components are in updated lists

## Determining Update State Programmatically

To check if a rolling update is in progress:

```go
func isRollingUpdateInProgress(pcs *PodCliqueSet) bool {
    return pcs.Status.RollingUpdateProgress != nil &&
           pcs.Status.RollingUpdateProgress.UpdateEndedAt == nil
}
```

To check which replica is currently being updated:

```go
func getCurrentlyUpdatingReplica(pcs *PodCliqueSet) *int32 {
    if pcs.Status.RollingUpdateProgress != nil &&
       pcs.Status.RollingUpdateProgress.CurrentlyUpdating != nil {
        return &pcs.Status.RollingUpdateProgress.CurrentlyUpdating.ReplicaIndex
    }
    return nil
}
```

To check update progress:

```go
func getUpdateProgress(pcs *PodCliqueSet) (updated, total int32) {
    total = pcs.Status.Replicas
    updated = pcs.Status.UpdatedReplicas
    return updated, total
}
```

## Notes

1. **Component-level tracking**: The `UpdatedPodCliques` and `UpdatedPodCliqueScalingGroups` lists track individual components that have been updated, not just replicas. This allows fine-grained progress tracking.

2. **Replica completion**: A replica is considered "updated" (counted in `UpdatedReplicas`) only when ALL its components (standalone PCLQs and PCSGs) are updated to the target generation hash.

3. **Cleanup on scale-in**: If a replica is deleted during a rolling update (scale-in), its components are removed from the updated lists to keep them accurate.

4. **Update initialization**: When a new rolling update starts, `UpdatedReplicas` is reset to `0` and the updated component lists are cleared (or will be cleared as the update progresses).
