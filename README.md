[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/NVIDIA/grove) 

[!NOTE]
>
> :construction_worker: `This project is currently under active construction, keep watching for announcements as we approach alpha launch!`

# Grove

Grove is a flexible Kubernetes API for orchestrating complex AI inference workloads in GPU clusters. It enables hierarchical composition of AI components with gang-scheduling, auto-scaling, and topology-aware placement through simple, declarative custom resources.

Grove was originally motivated by the challenges of orchestrating multinode, disaggregated inference systems, providing a unified API to define, configure, and scale components like prefill, decode, and routing within a single custom resource.

**Key Features:**
- Role-based pod groups for multi-component AI systems
- Hierarchical gang scheduling with flexible requirements
- Multi-level horizontal auto-scaling
- Network topology-aware scheduling
- Custom startup dependencies and rolling updates

For detailed information about Grove's motivation, architecture, and capabilities, see [docs/motivation.md](docs/motivation.md).

## Getting Started

### Prerequisites

- Kubernetes cluster (v1.19+)
- kubectl configured to access your cluster
- Helm (v3.0+)

### Quick Start

1. **Install Grove Operator**
   
   ```bash
   helm upgrade -i grove oci://ghcr.io/nvidia/grove/grove-charts:<tag>
   ```
   
   For additional installation methods and detailed setup instructions, see our [installation guide](docs/installation.md).

2. **Deploy Your First Grove Workload**
   
   Create a simple PodCliqueSet to get familiar with Grove concepts:
   
   ```bash
   kubectl apply -f operator/samples/simple/simple1.yaml
   ```

3. **Monitor Your Workload**
   
   Check the status of your Grove resources:
   
   ```bash
   # View PodCliqueSets
   kubectl get podcliquesets
   
   # View PodCliques
   kubectl get podcliques
   
   # Check detailed status
   kubectl describe podcliqueset <name>
   ```

4. **Explore Advanced Features**
   
   - [Explicit startup ordering](operator/samples/simple/simple2-explicit-startup-order.yaml)
   - [Complex startup dependencies](operator/samples/simple/simple3-explicit-startup-order.yaml)

### Next Steps

- Review [sample configurations](operator/samples/) for common patterns
- Read the [API documentation](docs/api-reference/) for detailed resource specifications
- Join our [community](#community-discussion-and-support) for support and discussions

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

Please feel free to start a [discussion thread](https://github.com/NVIDIA/grove/discussions) if you want to discuss a topic of interest.

In case, you have run into any issue or would like a feature enhancement, please create a [GitHub Issue](https://github.com/NVIDIA/grove/issues) with the appropriate tag.

To directly reach out to the Grove user and developer community, please join the [Grove mailing list](https://groups.google.com/g/grove-k8s).
