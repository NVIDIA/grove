> [!NOTE]
>
> :construction_worker: `This project site is currently under active construction, keep watching for announcements!`

# Grove

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub Release](https://img.shields.io/github/v/release/ai-dynamo/grove)](https://github.com/ai-dynamo/grove/releases/latest)
[![Discord](https://dcbadge.limes.pink/api/server/D92uqZRjCZ?style=flat)](https://discord.gg/GF45xZAX)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/ai-dynamo/grove)

Grove is a Kubernetes API purpose-built for orchestrating AI workloads on GPU clusters. The modern inference landscape spans a wide range of workload types — from traditional single-node deployments where each model instance runs in a single pod, to large-scale disaggregated systems where one model instance may include multiple components such as prefill and decode, each distributed across many pods and nodes. Grove is designed to unify this entire spectrum under a single API, allowing developers to declaratively represent any inference workload by composing as many components as their system requires — whether single-node or multi-node — within one cohesive custom resource.

Additionally, as workloads scale in size and complexity, achieving efficient resource utilization and optimal performance depends on capabilities such as all-or-nothing (“gang”) scheduling, topology-aware placement, prescriptive startup ordering, and independent scaling of components. Grove is designed with these needs as first-class citizens — providing native abstractions for expressing scheduling intent, topology constraints, startup dependencies, and per-component scaling behaviors that can be directly interpreted by underlying schedulers.

## Core Concepts

The Grove API consists of a user API and a scheduling API. While the user API (`PodCliqueSet`, `PodClique`, `PodCliqueScalingGroup`) allows users to represent their AI workloads, the scheduling API (`PodGang`) enables scheduler integration to support the network topology-optimized gang-scheduling and auto-scaling requirements of the workload.

| Concept                                                             | Description                                                                                                                                                                                              |
|---------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [PodCliqueSet](operator/api/core/v1alpha1/podcliqueset.go)          | The top-level Grove object that defines a group of components managed and colocated together. Also supports autoscaling with topology aware spread of PodCliqueSet replicas for availability.            |
| [PodClique](operator/api/core/v1alpha1/podclique.go)                | A group of pods representing a specific role (e.g., leader, worker, frontend). Each clique has an independent configuration and supports custom scaling logic.                                           |
| [PodCliqueScalingGroup](operator/api/core/v1alpha1/scalinggroup.go) | A set of PodCliques that scale and are scheduled together as a gang. Ideal for tightly coupled roles like prefill leader and worker.                                                                               |
| [PodGang](scheduler/api/core/v1alpha1/podgang.go)                   | The scheduler API that defines a unit of gang-scheduling. A PodGang is a collection of groups of similar pods, where each pod group defines a minimum number of replicas guaranteed for gang-scheduling. |


## Key Capabilities

- **Declarative composition of Role-Based Pod Groups**
  `PodCliqueSet` API provides users a capability to declaratively compose tightly coupled group of pods with explicit role based logic, e.g. disaggregated roles in a model serving stack such as `prefill`, `decode` and `routing`.
- **Flexible Gang Scheduling**
  `PodClique`'s and `PodCliqueScalingGroup`s allow users to specify flexible gang-scheduling requirements at multiple levels within a `PodCliqueSet` to prevent resource deadlocks.
- **Multi-level Horizontal Auto-Scaling**
  Supports pluggable horizontal auto-scaling solutions to scale `PodCliqueSet`, `PodClique` and `PodCliqueScalingGroup` custom resources.
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

**Release v0.1.0** *(ETA: Mid September 2025)*
- Grove v1alpha1 API
- Hierarchical Gang Scheduling and Gang Termination
- Multi-Level Horizontal Auto-Scaling
- Startup Ordering
- Rolling Updates

**Release v0.2.0** *(ETA: October 2025)*
- Topology-Aware Scheduling
- Resource-Optimized Rolling Updates

**Release v0.3.0** *(ETA: November 2025)*
- Multi-Node NVLink Auto-Scaling Support

## Contributions

Please read the [contribution guide](CONTRIBUTING.md) before creating you first PR!

## Community, Discussion, and Support

Grove is an open-source project and we welcome community engagement!

Please feel free to start a [discussion thread](https://github.com/ai-dynamo/grove/discussions) if you want to discuss a topic of interest.

In case, you have run into any issue or would like a feature enhancement, please create a [GitHub Issue](https://github.com/ai-dynamo/grove/issues) with the appropriate tag.

To directly reach out to the Grove user and developer community, please join the [NVIDIA Dynamo Discord server](https://discord.gg/D92uqZRjCZ), or [Grove mailing list](https://groups.google.com/g/grove-k8s).
