# Grove Core Concepts

This document provides an overview of Grove's fundamental concepts and custom resources that enable orchestration of complex AI workloads with coordinated scheduling, scaling, and lifecycle management.

## Overview

Grove introduces a hierarchical set of custom resources designed specifically for AI workloads that require coordinated multi-pod execution, gang scheduling, and network-aware placement. These resources abstract the complexity of distributed AI applications into declarative specifications.

## Resource Hierarchy

Grove's resources follow a clear hierarchical structure where higher-level resources create and manage lower-level resources:

```
PodCliqueSet (Top-level orchestration)
├── PodClique (Homogeneous pod groups)
├── PodCliqueScalingGroup (Coordinated scaling)
└── PodGang (Gang scheduling primitive)
    └── Pods (Actual workloads)
```

### Resource Ownership Model

- **User-Defined**: `PodCliqueSet` - Primary interface for defining AI workloads
- **Operator-Managed**: `PodClique`, `PodCliqueScalingGroup`, `PodGang` - Created automatically
- **Kubernetes-Native**: `Pod`, `HorizontalPodAutoscaler`, `Service` - Generated resources

## Core Resource Types

| Resource | Purpose | Scope | Documentation |
|----------|---------|-------|---------------|
| [`PodCliqueSet`](./podcliqueset.md) | Top-level orchestration resource | Entire AI system | Multi-replica deployment, gang scheduling, service discovery |
| [`PodClique`](./podclique.md) | Homogeneous pod group with specific role | Component within system | Autoscaling, startup dependencies, role-based organization |
| [`PodCliqueScalingGroup`](./podclique-scaling-group.md) | Coordinated scaling of multiple cliques | Cross-component scaling | Proportional scaling, unified HPA management |
| [`PodGang`](./podgang-scheduling.md) | Scheduler-facing gang scheduling primitive | Scheduling coordination | All-or-nothing scheduling, network-aware placement |

## Key Orchestration Concepts

### 1. Gang Scheduling
Grove ensures that groups of pods are scheduled atomically - either all pods in a gang are successfully placed, or none are scheduled until sufficient resources become available. This prevents resource deadlocks in distributed AI workloads.

### 2. Startup Dependencies  
PodCliques can specify explicit startup ordering through dependency declarations, enabling complex initialization sequences where certain components must be ready before others start.

### 3. Coordinated Scaling
Multiple PodCliques can be grouped for proportional scaling, ensuring that related components (like workers and parameter servers) scale together while maintaining proper ratios.

### 4. Network Topology Awareness
Grove's scheduler considers network bandwidth and node proximity when placing pods, optimizing for high-bandwidth communication patterns common in AI workloads.

## Example: AI Training Workload

Here's how Grove resources work together for a distributed training job:

```yaml
apiVersion: grove.io/v1alpha1
kind: PodCliqueSet
metadata:
  name: distributed-training
spec:
  replicas: 2  # Create 2 complete training setups
  template:
    cliqueStartupType: CliqueStartupTypeExplicit
    cliques:
    - name: parameter-server
      spec:
        roleName: parameter-server
        replicas: 1
        minAvailable: 1
        podSpec:
          containers:
          - name: ps
            image: training/ps:latest
    - name: coordinator  
      spec:
        roleName: coordinator
        replicas: 1
        minAvailable: 1
        startsAfter: ["parameter-server"]  # Wait for PS
        podSpec:
          containers:
          - name: coord
            image: training/coord:latest
    - name: worker
      spec:
        roleName: worker
        replicas: 4
        minAvailable: 3  # Need at least 3 workers
        startsAfter: ["coordinator"]  # Wait for coordinator
        podSpec:
          containers:
          - name: worker
            image: training/worker:latest
    
    podCliqueScalingGroups:
    - name: training-workers
      cliqueNames: ["worker", "parameter-server"] 
      replicas: 2  # Scale workers and PS together
      minAvailable: 1
```

This creates:
- **2 PodGangs** (one per replica) for gang scheduling
- **6 PodCliques** (parameter-server-0/1, coordinator-0/1, worker-0/1)
- **1 PodCliqueScalingGroup** for coordinated worker/PS scaling
- **Dependencies** ensuring proper startup order
- **Gang scheduling** for atomic pod placement

## Resource Lifecycle

1. **Creation**: User creates PodCliqueSet
2. **Generation**: Grove Operator creates PodClique, PodCliqueScalingGroup, and PodGang resources
3. **Scheduling**: Grove Scheduler performs gang scheduling with network awareness
4. **Startup**: Init containers handle dependency coordination
5. **Operation**: HPA manages scaling, operator maintains desired state
6. **Updates**: Rolling updates coordinate changes across all resources

## Status and Observability

All Grove resources provide comprehensive status information:

- **Observed Generation**: Tracks reconciliation progress
- **Conditions**: Standard Kubernetes condition reporting
- **Phase Information**: Resource-specific lifecycle phases
- **Error Reporting**: Detailed error information for debugging

## Integration Points

### Grove Operator
- Watches PodCliqueSet resources for changes
- Creates and manages child resources (PodClique, PodCliqueScalingGroup, PodGang)
- Handles rolling updates and reconciliation
- Injects init containers for dependency management

### Grove Scheduler  
- Consumes PodGang resources for gang scheduling decisions
- Implements network topology-aware placement
- Coordinates with Kubernetes scheduler for final pod binding

### Init Container System
- `grove-initc` binary handles startup dependencies
- Blocks pod startup until dependencies are satisfied
- Provides graceful error handling and logging

## Best Practices

1. **Design for Dependencies**: Carefully plan startup order for complex workloads
2. **Set Appropriate MinAvailable**: Balance availability with resource efficiency  
3. **Group Related Cliques**: Use PodCliqueScalingGroup for components that should scale together
4. **Consider Network Topology**: Leverage Grove's network-aware scheduling for performance
5. **Monitor Resource Status**: Use Grove's comprehensive status reporting for observability

## What's Next

- [PodCliqueSet Details](./podcliqueset.md) - Complete API specification and examples
- [PodClique Details](./podclique.md) - Individual pod group management
- [Scaling Groups](./podclique-scaling-group.md) - Coordinated scaling configuration  
- [Gang Scheduling](./podgang-scheduling.md) - Scheduling primitives and network awareness
- [Dependency Management](./dependency-management.md) - Init container system and startup coordination
