# System Architecture

Grove is a Kubernetes orchestration system for AI workloads that provides hierarchical gang scheduling and multi-dimensional auto-scaling through custom resources and operators. This document describes the high-level system architecture showing how Grove components interact to manage disaggregated inference systems and complex AI pipelines.

The system consists of three main architectural layers: custom resource APIs for workload definition, the Grove Operator for resource lifecycle management, and the Grove Scheduler for network-aware gang scheduling.

## Grove System High-Level Architecture

Grove provides a comprehensive orchestration system for AI workloads with hierarchical custom resources, operator-managed lifecycle control, and gang scheduling capabilities.

Grove operates as a layered system within Kubernetes:

**User API Layer (`grove.io/v1alpha1`)**
- **PodCliqueSet**: Top-level orchestration resource that defines complete AI systems (e.g., a disaggregated inference pipeline)
- **PodClique**: Individual pod groups representing specific roles (prefill engines, decode workers, routing components, etc.)
- **PodCliqueScalingGroup**: Coordinated scaling units for tightly coupled pod groups that must scale together

**Grove Operator**
- **Controllers**: Reconcile desired state for all Grove custom resources, managing the full lifecycle from creation to deletion
- **Admission Webhooks**: Validate resource specifications and apply sensible defaults to reduce configuration complexity
- **Component Operators**: Manage underlying Kubernetes resources (Pods, Services, HPAs) with specialized logic for AI workload patterns

**Scheduler Integration**
- **PodGang** (`scheduler.grove.io/v1alpha1`): Bridge resource that translates Grove specifications into gang scheduling requirements
- **Grove Scheduler**: Custom Kubernetes scheduler that provides gang scheduling, network topology-aware placement, and AI workload optimizations

**Kubernetes Foundation**
- Standard Kubernetes resources (Pods, Services, HPAs) created and managed by Grove operators
- Native Kubernetes scheduling extended with Grove's advanced AI workload scheduling capabilities

```
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                          │
│                                                                 │
│  ┌─────────────────┐    ┌─────────────────────────────────────┐ │
│  │ Grove Scheduler │    │        Grove Operator               │ │
│  │ Gang scheduling │    │  ┌─────────────┬─────────────────┐  │ │
│  │ & topology      │    │  │ Controllers │  Admission      │  │ │
│  │ optimization    │    │  │ Reconcile   │  Webhooks       │  │ │
│  └─────────────────┘    │  │ resources   │  Validate &     │  │ │
│                         │  │             │  default        │  │ │
│                         │  └─────────────┴─────────────────┘  │ │
│                         └─────────────────────────────────────┘ │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │           Grove APIs (grove.io/v1alpha1)                   │ │
│  │  PodCliqueSet → PodClique → PodCliqueScalingGroup          │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                    ↓                           │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │        Standard Kubernetes Resources                        │ │
│  │         Pods • Services • HPAs                              │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Grove Resource Hierarchy and Ownership

Grove uses a hierarchical ownership model where `PodCliqueSet` acts as the top-level orchestrator that manages multiple `PodClique` instances representing different AI workload roles (prefill, decode, etc.).

### Ownership and Control Flow

**1. User Definition Phase**
Users define their AI workloads using Grove's declarative APIs. They specify components, scaling requirements, dependencies, and resource needs in a single `PodCliqueSet` specification.

**2. PodCliqueSet: Top-Level Orchestration**
The `PodCliqueSet` serves as the primary interface for defining complete AI systems. For example, a disaggregated inference system might include:
- **prefill** `PodClique`: Handles initial token processing
- **decode** `PodClique`: Manages iterative token generation
- **routing** `PodClique`: Distributes requests across components

**3. PodCliqueScalingGroup: Coordinated Scaling**
Some AI workloads require components to scale together. A `PodCliqueScalingGroup` ensures that tightly coupled components (like leader/worker pairs in distributed inference) maintain proper ratios when scaling up or down.

**4. Scheduler Integration via PodGang**
Grove controllers automatically create `PodGang` resources that translate user specifications into scheduler-friendly gang scheduling requirements. This enables:
- **Standalone PodGangs**: For independent `PodClique` instances
- **Coordinated PodGangs**: For scaling groups that must be scheduled together

**5. Kubernetes Resource Management**
The Grove operator creates and manages standard Kubernetes resources:
- **Pods**: The actual workload containers
- **HeadlessServices**: Enable direct inter-pod communication for AI workloads
- **HorizontalPodAutoscalers**: Provide dynamic scaling based on custom metrics

```
                        User defines
┌─────────────────────────────────────────────────────────────────┐
│                    PodCliqueSet                                 │
│               "Disaggregated Inference System"                 │
│                                                                 │
│  ┌─────────────────────┬─────────────────────┐                  │
│  │   PodClique         │   PodClique         │                  │
│  │   "prefill"         │   "decode"          │                  │
│  │   ┌─────────────┐   │   ┌─────────────┐   │                  │
│  │   │ Pod         │   │   │ Pod         │   │                  │
│  │   │ Pod         │   │   │ Pod         │   │                  │
│  │   │ Pod         │   │   │ Pod         │   │                  │
│  │   └─────────────┘   │   └─────────────┘   │                  │
│  └─────────────────────┴─────────────────────┘                  │
│                                                                 │
│           PodCliqueScalingGroup                                 │
│           "Coordinated Workers"                                 │
│  ┌─────────────────────┬─────────────────────┐                  │
│  │   PodClique         │   PodClique         │                  │
│  │   "leader"          │   "workers"         │                  │
│  │   ┌─────────────┐   │   ┌─────────────┐   │                  │
│  │   │ Pod         │   │   │ Pod         │   │                  │
│  │   └─────────────┘   │   │ Pod         │   │                  │
│  │                     │   │ Pod         │   │                  │
│  │                     │   └─────────────┘   │                  │
│  └─────────────────────┴─────────────────────┘                  │
└─────────────────────────────────────────────────────────────────┘
                           │
                    Grove Operator creates
                           ▼
                     PodGang resources
                   (scheduler integration)
                           │
                    Grove Scheduler
                     gang schedules
                           ▼
                  Kubernetes Nodes & Pods
```

## Grove Operator Internal Architecture

The Grove operator implements a component-based architecture using controller-runtime with specialized reconcilers for each custom resource type and admission webhooks for validation.

### Operator Components

**Controller Runtime Manager**
The foundation of Grove's operator architecture, managing the lifecycle of all controllers, webhooks, and watchers. It coordinates resource reconciliation and ensures consistent state management across all Grove components.

**Admission Control Layer**
- **Defaulting Webhooks**: Automatically apply sensible defaults to Grove resources, reducing configuration complexity for users. For example, setting default resource requests and inter-pod communication ports.
- **Validating Webhooks**: Enforce business rules and constraints, such as validating that startup dependencies don't create cycles and ensuring gang scheduling requirements are feasible.

**Primary Controllers**
- **PodCliqueSet Controller**: Orchestrates top-level AI system definitions, managing the complete lifecycle from creation through scaling to deletion. Handles replica management and coordinates with child resources.
- **PodClique Controller**: Manages individual pod groups representing specific AI roles. Handles pod creation, health monitoring, and role-specific configuration.
- **PodCliqueScalingGroup Controller**: Coordinates scaling operations across tightly coupled components, ensuring proper ratios and dependencies are maintained during scale events.

**Component Operators**
Specialized operators that manage specific Kubernetes resources on behalf of Grove:
- **Pod Operator**: Creates and manages individual pods with AI-specific configurations (GPU allocations, network settings, etc.)
- **Service Operator**: Manages HeadlessServices to enable direct inter-pod communication required by AI workloads
- **HPA Operator**: Configures horizontal pod autoscaling based on custom AI metrics (GPU utilization, inference latency, etc.)
- **PodGang Operator**: Creates scheduler integration resources and manages gang scheduling lifecycle

**Status and Error Management**
- **Status Recorder**: Tracks the state of all reconciliation operations, providing detailed visibility into system health and progress
- **Condition Management**: Maintains structured condition reporting for health monitoring, readiness checks, and troubleshooting

## Grove Scheduler Integration

Grove integrates with Kubernetes scheduling through the `PodGang` resource that acts as a bridge between the operator domain and scheduler domain, enabling gang scheduling with network topology awareness.

### Scheduler Integration Flow

**1. Operator to Scheduler Translation**
When Grove controllers reconcile user-defined resources (`PodCliqueSet`, `PodClique`, `PodCliqueScalingGroup`), they automatically create corresponding `PodGang` resources. This translation process converts high-level AI workload specifications into scheduler-consumable gang scheduling requirements.

**2. PodGang Resource Structure**
The `PodGang` resource (`scheduler.grove.io/v1alpha1`) serves as the scheduler API bridge, containing:
- **PodGroups**: Defines minimum replica requirements for coordinated scheduling
- **NetworkPackGroupConfigs**: Specifies network topology preferences for optimal communication
- **SpreadConstraints**: Controls how pod groups are distributed across failure domains
- **MinReplicas**: Sets minimum thresholds for gang scheduling decisions

**3. Grove Scheduler Processing**
The Grove scheduler extends standard Kubernetes scheduling with AI workload optimizations:
- **Gang Scheduling Logic**: Ensures all pods in a gang are scheduled together or not at all, preventing resource deadlocks
- **Network Topology Optimization**: Places tightly coupled components (like prefill/decode pairs) on nodes with optimal network connectivity
- **Resource-Aware Placement**: Considers AI-specific requirements like GPU topology, memory bandwidth, and inter-node communication patterns

**4. Scheduling Coordination**
Grove scheduler operates alongside the default Kubernetes scheduler:
- **Grove Scheduler**: Handles pods with gang scheduling requirements and network topology constraints
- **Default Scheduler**: Processes non-gang pods and provides fallback scheduling capabilities

**5. Node Binding and Execution**
Once scheduling decisions are made, pods are bound to appropriate nodes where they execute as standard Kubernetes workloads, but with coordinated placement that optimizes AI workload performance.

## Multi-Level Scaling and Startup Ordering

Grove provides hierarchical scaling capabilities at multiple levels and supports explicit startup ordering between `PodClique` instances using dependency declarations.

### Scaling Hierarchy

Grove enables scaling operations at three distinct levels, each affecting different scopes of resources:

**PodCliqueSet Scaling**
- **Command**: `kubectl scale pcs simple1 --replicas=2`
- **Effect**: Creates complete replica sets of the entire AI system
- **Result**: Generates new instances like `simple1-1-pca`, `simple1-1-pcb`, etc., maintaining all component relationships and dependencies
- **Use Case**: Scaling entire inference pipelines for increased throughput

**PodCliqueScalingGroup Scaling**
- **Command**: `kubectl scale pcsg simple1-0-pcsg --replicas=2`
- **Effect**: Scales tightly coupled components together within a single system replica
- **Result**: Creates coordinated instances like `simple1-0-sga-1-pcb`, `simple1-0-sga-1-pcc`
- **Use Case**: Scaling worker pools while maintaining leader-worker ratios

**Individual PodClique Auto-scaling**
- **Mechanism**: HorizontalPodAutoscaler integration
- **Effect**: Dynamically adjusts individual component replicas based on metrics
- **Triggers**: CPU utilization, memory usage, custom AI metrics (GPU utilization, inference latency)
- **Use Case**: Responsive scaling of specific components based on workload patterns

### Startup Ordering (CliqueStartupTypeExplicit)

Grove supports explicit startup dependencies to ensure proper initialization sequences for AI workloads:

**Dependency Chain Example**:
1. **pca** (`startsAfter: []`): Starts immediately as the foundation component (e.g., model loader)
2. **pcb** (`startsAfter: [pca]`): Waits for pca readiness before starting (e.g., prefill engine)
3. **pcc** (`startsAfter: [pca]`): Also waits for pca but can start parallel with pcb (e.g., decode engine)
4. **pcd** (`startsAfter: [pcb, pcc]`): Waits for both pcb and pcc to be ready (e.g., routing/load balancer)

**Key Benefits**:
- **Prevents startup race conditions** in complex AI pipelines
- **Ensures proper resource initialization** (models loaded before inference starts)
- **Enables parallel startup** where dependencies allow
- **Supports complex dependency graphs** for sophisticated AI architectures

## Status and Error Handling Architecture

Grove implements a comprehensive status tracking system with structured error handling and operation recording. Each custom resource maintains detailed status information about reconciliation operations and component states.

### Status Tracking Components

Grove provides comprehensive status reporting across all resource types to enable effective monitoring and troubleshooting of AI workloads.

**Resource Status Structures**

Each Grove resource maintains detailed status information:

- **PodCliqueSetStatus**: Tracks replica counts (total, ready, updated), reconciliation operations, error history, and observed generation for change detection
- **PodCliqueStatus**: Maintains condition reporting, pod selectors, operation history, and error tracking for individual pod groups
- **PodCliqueScalingGroupStatus**: Coordinates with HPA systems, tracks scaling operations, and maintains selector information for grouped components
- **PodGangStatus**: Array of individual gang scheduling states, providing visibility into scheduler coordination

**Operation Lifecycle Tracking**

Grove tracks two primary operation types:
- **Reconcile Operations**: Normal lifecycle management (create, update, scale)
- **Delete Operations**: Resource cleanup and termination procedures

Each operation progresses through defined states:
- **Processing**: Operation is actively being executed
- **Succeeded**: Operation completed successfully
- **Error**: Operation encountered issues requiring attention

**Status Management Methods**

The Grove operator provides structured methods for status updates:
- **SetLastOperation()**: Records operation type, state, description, and timestamp for audit trails
- **SetLastErrors()**: Captures error details with error codes and observation timestamps for debugging
- **RecordStart()**: Marks the beginning of operations and clears previous error states
- **RecordCompletion()**: Updates final operation state and records any encountered errors

This comprehensive status system enables users to monitor AI workload health, troubleshoot issues, and track system behavior over time.

## Summary

Grove's system architecture provides a comprehensive orchestration platform for AI workloads through:

- **Hierarchical Resource Model**: From high-level `PodCliqueSet` down to individual `Pod` resources
- **Multi-layered Scheduling**: Integration between operator-managed resources and scheduler-aware gang scheduling
- **Component-based Operator**: Modular controllers and operators for different resource types
- **Comprehensive Status Tracking**: Detailed operation and error recording across all resource levels
- **Flexible Scaling**: Support for scaling at multiple hierarchical levels with coordinated dependencies
- **Network-aware Scheduling**: Topology optimization for AI workload performance requirements

This architecture enables Grove to handle complex AI workloads ranging from simple single-node inference to sophisticated multi-node disaggregated systems with precise coordination and scaling requirements.
