# Gang Scheduling Semantics: Standalone PodClique vs PodCliqueScalingGroup

## Overview

Grove implements a **two-tier gang scheduling system** that handles standalone PodCliques and PodCliqueScalingGroups differently:

1. **Base PodGang**: Created by PodCliqueSet controller for each PCS replica
2. **Scaled PodGangs**: Created by PodCliqueSet controller for PCSG replicas above `minAvailable`

## Standalone PodClique Gang Creation

For standalone PodCliques (not part of a PodCliqueScalingGroup):

### Structure

- Each PCS replica has **one base PodGang** (e.g., `my-pcs-0`)
- The base PodGang contains **all standalone PodCliques** from that PCS replica
- Each PodClique becomes a **PodGroup** within the PodGang

### Example

```
PCS Replica 0:
  Base PodGang: "my-pcs-0"
    ├─ PodGroup: "my-pcs-0-frontend" (standalone PC)
    └─ PodGroup: "my-pcs-0-api" (standalone PC)
```

### Gang Scheduling Behavior

- All pods from all standalone PodCliques in the base PodGang are **scheduled together as one gang**
- If any PodGroup can't meet its `MinReplicas`, the entire gang waits
- Scheduling gates are removed once all PodGroups have at least `MinReplicas` scheduled

## PodCliqueScalingGroup Gang Creation

For PodCliqueScalingGroups, gang creation is split into two tiers based on `minAvailable`:

### Base PodGang (Replicas 0 to minAvailable-1)

#### Structure

- Replicas **0 through `minAvailable-1`** are included in the **base PodGang**
- Each PCSG replica contributes **one PodClique per `CliqueName`** to the base PodGang
- All these PodCliques form **PodGroups** within the same base PodGang

#### Example

With `minAvailable=3`, `replicas=5`:

```
PCS Replica 0:
  Base PodGang: "my-pcs-0"
    ├─ PodGroup: "my-pcs-0-prefill-0-leader" (PCSG replica 0)
    ├─ PodGroup: "my-pcs-0-prefill-0-worker" (PCSG replica 0)
    ├─ PodGroup: "my-pcs-0-prefill-1-leader" (PCSG replica 1)
    ├─ PodGroup: "my-pcs-0-prefill-1-worker" (PCSG replica 1)
    ├─ PodGroup: "my-pcs-0-prefill-2-leader" (PCSG replica 2)
    └─ PodGroup: "my-pcs-0-prefill-2-worker" (PCSG replica 2)
```

#### Gang Scheduling Behavior

- All pods from replicas 0-2 are **scheduled together as one gang**
- Scheduling gates are removed once all PodGroups meet their `MinReplicas`
- This ensures the minimum viable cluster is established atomically

### Scaled PodGangs (Replicas minAvailable and above)

#### Structure

- Each PCSG replica **>= `minAvailable`** gets its **own scaled PodGang**
- Scaled PodGang naming: `{pcsg-fqn}-{podGangIndex}` (0-based index)
- Each scaled PodGang contains **one PodClique per `CliqueName`** from that PCSG replica

#### Example

With `minAvailable=3`, `replicas=5`:

```
PCS Replica 0:
  Scaled PodGang: "my-pcs-0-prefill-0" (PCSG replica 3)
    ├─ PodGroup: "my-pcs-0-prefill-3-leader"
    └─ PodGroup: "my-pcs-0-prefill-3-worker"

  Scaled PodGang: "my-pcs-0-prefill-1" (PCSG replica 4)
    ├─ PodGroup: "my-pcs-0-prefill-4-leader"
    └─ PodGroup: "my-pcs-0-prefill-4-worker"
```

#### Gang Scheduling Behavior

- Each scaled PodGang is **scheduled independently**
- Scaled PodGang pods have **scheduling gates** that are removed only **after the base PodGang is ready**
- This ensures the base cluster is operational before scale-out replicas are scheduled

## Key Differences

| Aspect           | Standalone PC               | PCSG Base (0 to minAvailable-1)                | PCSG Scaled (minAvailable+)                                        |
| ---------------- | --------------------------- | ---------------------------------------------- | ------------------------------------------------------------------ |
| **PodGang**      | Base PodGang only           | Base PodGang                                   | Individual scaled PodGang per replica                              |
| **Gang Size**    | All standalone PCs together | All PCSG replicas 0 to minAvailable-1 together | One PCSG replica per gang                                          |
| **Gate Removal** | When base PodGang ready     | When base PodGang ready                        | After base PodGang is ready                                        |
| **Controller**   | PodCliqueSet                | PodCliqueSet                                   | PodCliqueSet (computed, but PodCliques created by PCSG controller) |

## PodGroup Structure

Each PodGroup in a PodGang represents:

- **Name**: The PodClique FQN (fully qualified name)
- **PodReferences**: List of pod names belonging to that PodClique
- **MinReplicas**: The `MinAvailable` value from the PodClique spec
- **TopologyConstraint**: Optional topology constraints for that PodClique

## Summary

The gang scheduling semantics work as follows:

1. **Standalone PodCliques**: All included in the base PodGang, scheduled together
2. **PCSG Base**: Replicas 0 to `minAvailable-1` form one base PodGang, scheduled together
3. **PCSG Scaled**: Each replica >= `minAvailable` gets its own scaled PodGang, scheduled independently but gated until base is ready

This design ensures:

- **Minimum viable cluster (base) is gang-scheduled** for reliability
- **Scale-out replicas can be scheduled independently** for efficiency
- **Base cluster must be ready** before scale-out replicas are scheduled

## Implementation Details

### Base PodGang Creation

Base PodGangs are created by the PodCliqueSet controller's PodGang component:

- For each PCS replica, one base PodGang is created
- Standalone PodCliques are directly included
- PCSG replicas 0 to `minAvailable-1` are included via `buildPCSGPodCliqueInfosForBasePodGang()`

### Scaled PodGang Creation

Scaled PodGangs are computed by the PodCliqueSet controller:

- Computed via `getExpectedPodGangsForPCSG()` for each PCS replica
- Created for PCSG replicas starting from `minAvailable`
- PodCliques for scaled replicas are created by the PodCliqueScalingGroup controller
- PodGang resources reference these PodCliques

### PodGroup Construction

PodGroups are constructed from PodCliques:

- Each PodClique becomes one PodGroup
- Pod references are collected from pods labeled with the PodClique's PodGang label
- `MinReplicas` comes from the PodClique's `MinAvailable` spec
- Topology constraints are propagated from PodClique to PodGroup
