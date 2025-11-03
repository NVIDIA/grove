# Grove

Modern AI inference workloads need capabilities that Kubernetes doesn't provide out-of-the-box:

- **Gang scheduling** - Prefill and decode pods must start together or not at all
- **Grouped scaling** - Tightly-coupled components that need to scale as a unit
- **Startup ordering** - Different components in a workload which must start in an explicit ordering
- **Topology-aware placement** - NVLink-connected GPUs or workloads shouldn't be scattered across nodes

Grove is a Kubernetes API that provides a single declarative interface for orchestrating any AI inference workload — from simple, single-pod deployments to complex multi-node, disaggregated systems. Grove lets you scale your multinode inference deployment from a single replica to data center scale, supporting tens of thousands of GPUs. It allows you to describe your whole inference serving system in Kubernetes - e.g. prefill, decode, routing or any other component - as a single Custom Resource (CR). From that one spec, the platform coordinates hierarchical gang scheduling, topology‑aware placement, multi-level autoscaling and explicit startup ordering. You get precise control of how the system behaves without stitching together scripts, YAML files, or custom controllers.

**One API. Any inference architecture.**

## Quick Start

Get Grove running in 5 minutes:
[![Go Report Card](https://goreportcard.com/badge/github.com/ai-dynamo/grove/operator)](https://goreportcard.com/report/github.com/NVIDIA/grove/operator)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub Release](https://img.shields.io/github/v/release/ai-dynamo/grove)](https://github.com/ai-dynamo/grove/releases/latest)
[![Discord](https://dcbadge.limes.pink/api/server/D92uqZRjCZ?style=flat)](https://discord.gg/GF45xZAX)

```bash
# 1. Create a local kind cluster
cd operator && make kind-up

# 2. Deploy Grove
make deploy

# 3. Deploy your first workload
kubectl apply -f samples/simple/simple1.yaml

# 4. Fetch the resources created by grove
kubectl get pcs,pclq,pcsg,pg,pod -owide
```

**→ [Installation Docs](docs/installation.md)**

## What Grove Solves

Grove handles the complexities of modern AI inference deployments:

| Your Setup | What Grove Does |
|------------|-----------------|
| **Disaggregated inference** (prefill + decode) | Gang schedules all components together, scales them independently and as a unit |
| **Multi-model pipelines** | Enforces startup order (router → workers), auto-scales each stage |
| **Multi-node inference** (DeepSeek-R1, Llama 405B) | Packs pods onto NVLink-connected GPUs for optimal network performance |
| **Simple single-pod serving** | Works for this too! One API for any architecture |

**Use Cases:** [Multi-node disaggregated](docs/assets/multinode-disaggregated.excalidraw.png) · [Single-node disaggregated](docs/assets/singlenode-disaggregated.excalidraw.png) · [Agentic pipelines](docs/assets/agentic-pipeline.excalidraw.png) · [Standard serving](docs/assets/singlenode-aggregated.excalidraw.png)

## How It Works

Grove introduces four simple concepts:

| Concept | What It Does |
|---------|--------------|
| **PodCliqueSet** | Your entire workload (e.g., "my-inference-stack") |
| **PodClique** | A component role (e.g., "prefill", "decode", "router") |
| **PodCliqueScalingGroup** | Components that must scale together (e.g., prefill + decode) |
| **PodGang** | Internal scheduler primitive for gang scheduling (you don't touch this) |

**→ [API Reference](docs/api-reference/operator-api.md)**

## Roadmap

### 2025 Priorities

> **Note:** We are aligning our release schedule with [NVIDIA Dynamo](https://github.com/ai-dynamo/dynamo) to ensure seamless integration. Release dates will be updated once our cadence (e.g., weekly, monthly) is finalized.

**Q4 2025**
- Topology-Aware Scheduling
- Multi-Level Horizontal Auto-Scaling
- Startup Ordering
- Rolling Updates

**Q1 2026**
- Resource-Optimized Rolling Updates
- Multi-Node NVLink Auto-Scaling Support

## Contributions

Please read the [contribution guide](CONTRIBUTING.md) before creating you first PR!

## Community, Discussion, and Support

Grove is an open-source project and we welcome community engagement!

Please feel free to start a [discussion thread](https://github.com/ai-dynamo/grove/discussions) if you want to discuss a topic of interest.

In case, you have run into any issue or would like a feature enhancement, please create a [GitHub Issue](https://github.com/ai-dynamo/grove/issues) with the appropriate tag.

To directly reach out to the Grove user and developer community, please join the [NVIDIA Dynamo Discord server](https://discord.gg/D92uqZRjCZ), or [Grove mailing list](https://groups.google.com/g/grove-k8s).
