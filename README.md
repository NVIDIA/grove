> [!NOTE]
>
> :construction_worker: `This project site is currently under active construction, keep watching for announcements as we approach alpha launch!`

# Grove

Grove is a Kubernetes API purpose-built for orchestrating AI workloads in GPU clusters, where a single custom resource allows you to hierarchically compose multiple AI components with flexible gang-scheduling and auto-scaling specfications at multiple levels. Through native support for network topology-aware gang scheduling, multi-dimensional auto-scaling and prescriptive startup ordering, Grove enables developers to define complex AI stacks in a concise, declarative, and framework-agnostic manner.

Grove was originally motivated by the challenges of orchestrating multinode, disaggregated inference systems. It provides a consistent and unified API that allows users to define, configure, and scale prefill, decode, and any other components like routing within a single custom resource. However, it is flexible enough to map naturally to the roles, scaling behaviors, and dependencies of any real-world inference systems, from "traditional" single node aggregated inference to agentic pipelines with multiple models.

## Why Grove?

Modern inference systems are often no longer single-pod workloads. They involve multiple components running across many nodes, often requiring coordination, colocation, custom roles, and precise startup ordering. Inference workloads also need better scheduler coordination to achieve key performance SLAs with features such as network topology-aware gang-scheduling, auto-scaling, rolling updates and more. The Grove project was created so that AI developers can define their workload in a declarative manner and influence scheduler level optimizations with easy-to-use, high-level, Grove scheduling APIs.


## Core Concepts

The Grove API consists of a user API and a scheduling API. While the user API (`PodGangSet`, `PodClique`, `PodCliqueScalingGroup`) allows users to represent their AI workloads, the scheduling API (`PodGang`) enables scheduler integration to support the network topology-optimized gang-scheduling and auto-scaling requirements of the workload.

| Concept                                                      | Description                                                  |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| [PodGangSet](operator/api/core/v1alpha1/podgangset.go) | The top-level Grove object that defines a group of components managed and colocated together. Also supports autoscaling with topology aware spread of PodGangSet replicas for availability. |
| [PodClique](operator/api/core/v1alpha1/podclique.go) | A group of pods representing a specific role (e.g., leader, worker, frontend). Each clique has an independent configuration and supports custom scaling logic. |
| [PodCliqueScalingGroup](operator/api/core/v1alpha1/scalinggroup.go) | A set of PodCliques that scale and are scheduled together. Ideal for tightly coupled roles like prefill leader and worker. |
| [PodGang](scheduler/api/core/v1alpha1/podgang.go)            | The scheduler API that defines a unit of gang-scheduling. A PodGang is a collection of groups of similar pods, where each pod group defines a minimum number of replicas guaranteed for gang-scheduling. |


## Key Capabilities

- **Declarative composition of Role-Based Pod Groups**
  `PodGangSet` API provides users a capability to declaratively compose tightly coupled group of pods with explicit role based logic, e.g. disaggregated roles in a model serving stack such as `prefill`, `decode` and `routing`.
- **Flexible Gang Scheduling**
  `PodClique`'s and `PodCliqueScalingGroup`s allow users to specify flexible gang-scheduling requirements at multiple levels within a `PodGangSet` to prevent resource deadlocks.
- **Multi-level Horizontal Auto-Scaling**
  Supports pluggable horizontal auto-scaling solutions to scale `PodGangSet`, `PodClique` and `PodCliqueScalingGroup` custom resources.
- **Network Topology-Aware Scheduling**
  Allows specifying network topology pack and spread constraints to optimize for both network performance and service availability.
- **Custom Startup Dependencies**
  Prescribe the order in which the `PodClique`s must start in a declarative specification. Pod startup is decoupled from pod creation or scheduling.
- **Resource-Aware Rolling Updates**
  Supports reuse of resource reservations of `Pod`s during updates in order to preserve topology-optimized placement.

## Example Use Cases

- **Multi-Node, Disaggregated Inference for large models** ***(DeepSeek-R1, Llama-4-Maverick)*** : [Visualization](docs/assets/multinode-disaggregated.excalidraw.png)
- **Single-Node, Disaggregated Inference** : [Visualization](docs/assets/singlenode-disaggregated.excalidraw.png)
- **Agentic Pipeline of Models** : [Visualization](docs/assets/agentic-pipeline.excalidraw.png)
- **Standard Aggregated Single Node or Single GPU Inference** : [Visualization](docs/assets/singlenode-aggregated.excalidraw.png)

## Getting Started

You can get started with the Grove operator by following our [installation guide](docs/installation.md).

## Roadmap

### 2025 Priorities

Update: We are aligning our release schedule with [Nvidia Dynamo](https://github.com/ai-dynamo/dynamo) to ensure seamless integration. Once our release cadence (e.g., weekly, monthly) is finalized, it will be reflected here.

**Release v0.1.0** *(ETA: 8/27/2025)*
- Grove v1alpha1 API
- Hierarchical Gang Scheduling and Gang Termination
- Multi-Level Horizontal Auto-Scaling
- Startup Ordering
- Rolling Updates

**Release v0.2.0** *(ETA: Mid September 2025)*
- Topology-Aware Scheduling
- Resource-Optimized Rolling Updates

**Release v0.3.0** *(ETA: November 2025)*
- Multi-Node NVLink Auto-Scaling Support

## Contributions

:construction:

## Community, Discussion, and Support

:construction:
