## GREP-270: Automatic MNNVL Support in Grove

**GREP owner**: TBD  
**Status**: Draft  
**Tracking issue**: [Add automatic support for MNNVL](https://github.com/ai-dynamo/grove/issues/270)

### Problem statement

Grove today does not provide first-class support for NVIDIA Multi-Node NVLink (MNNVL). As a result, workloads running on systems like the NVIDIA GB200 NVL72 cannot automatically form secure, high-bandwidth GPU fabrics across nodes using NVIDIA ComputeDomains and Dynamic Resource Allocation (DRA).

We need a way for a `PodCliqueSet` (PCS) workload to:

- Express that it **requires a ComputeDomain** (i.e. wants to run with MNNVL).
- Have Grove take care of:
  - Creating and wiring **ComputeDomains**.
  - Creating and wiring **ResourceClaimTemplates** (RCTs).
  - Injecting **pod-level resource claims** and **container-level resource claim usage**.
  - Cleaning up these resources when the workload terminates.

### Goals and non-goals

- **Goals**
  - Allow a PCS to declare MNNVL usage declaratively in its spec.
  - Support **per-replica ComputeDomains** so each PCS replica has its own isolated GPU fabric.
  - Automatically manage the lifecycle of ComputeDomains and RCTs for managed mode.
  - Ensure pods are correctly wired via Kubernetes `resourceClaims` and container `resources.claims`.

### User-facing API changes

Add a `computeDomainConfig` section to `PodCliqueSet.spec.template`:

- **`enabled`**: bool – turn MNNVL on/off for the PCS.
- **`scope`**: enum – `PerReplica` vs `PerPCS` (default: `PerReplica`).
- **`deviceClassName`**: string – device class for the RCT, default `compute-domain.nvidia.com`.

In this design, the PCS API only captures **high-level intent** (enable MNNVL and choose scope).  
Even in `PerReplica` mode, users **do not define per-replica ComputeDomains explicitly** (names or instances); Grove derives and manages those resources from the PCS spec and replica indices.

#### Manual / non-automatic usage limitations

If `computeDomainConfig.enabled` is **disabled**, users can still hand-craft `resourceClaims` and `resources.claims` in the PCS/PodClique templates, but only in a **replica-agnostic** way:

- The PCS and PodClique specs do not carry the PCS replica index.
- Therefore, users **cannot express true per-replica ComputeDomains manually** (for example, “replica-0 uses CD-0, replica-1 uses CD-1”) just by editing the PCS.
- Per-replica ComputeDomains are only available through Grove’s automatic wiring, which has access to the replica index at reconciliation time.

Admission webhooks for PCS will:

- **Default** these fields when `computeDomainConfig.enabled=true`.
- **Validate** that the configuration is internally consistent (e.g. GPU requests present, no conflicting user-defined ResourceClaims).

### Design dimension 1: where to inject ResourceClaims

There are three plausible layers where we can inject the `resourceClaims` and container `resources.claims`:

1. **PCS-level (admission on `PodCliqueSet`)**
   - Inject directly into `PodCliqueSet.spec.template.Cliques[*].Spec.PodSpec`.
   - **Pros**:
     - PCS spec fully describes ResourceClaims; very transparent to users.
     - Implementation is localized to existing PCS defaulting logic.
   - **Cons**:
     - PCS template is replica-agnostic; this is a natural fit for a **single** ComputeDomain/RCT per PCS (PerPCS), not for **per-replica** domains.
     - Per-replica naming and wiring become awkward to represent at this level.

2. **PodClique-level (PCS controller’s PodClique component)**
   - PCS carries high-level `computeDomainConfig` intent.
   - The PCS controller’s PodClique operator builds a `PodClique` for each `(PCS replica index, clique)` pair.
   - At this point we know:
     - The `PodCliqueSet` object.
     - The **replica index**.
     - The associated `PodCliqueTemplateSpec`.
   - We can then:
     - Derive **per-replica ComputeDomain** and **ResourceClaimTemplate** names from PCS name + replica index.
     - Inject the appropriate `resourceClaims` entry and container `resources.claims` into `PodClique.spec.PodSpec` for that replica.
   - **Pros**:
     - Clean mapping from **PCS replica index → PodClique → ComputeDomain/RCT**.
     - `PodClique` spec remains the single source of truth for the pod template; pods just mirror it.
     - Works very naturally with per-replica semantics.
     - Keeps PCS responsible for high-level **intent** only (e.g. enable MNNVL, choose scope).
     - Keeps controllers responsible for **per-replica concrete resources and wiring**.
     - Keeps webhooks focused on **defaulting and validation** rather than replica-specific wiring.
   - **Cons**:
     - Replica-specific wiring is visible at the `PodClique` level, not directly in the PCS spec.

3. **Pod-level (PodClique→Pod component)**
   - Inject directly into `corev1.Pod` in the Pod building path, just before pod creation.
   - **Pros**:
     - Maximum flexibility (even per-pod variation is possible).
     - No additional fields are required in PCS or PodClique specs.
   - **Cons**:
     - Neither PCS nor PodClique specs show the final pod shape; behavior is more “magical”.
     - Harder to validate and reason about, especially with policy engines that look at CRDs rather than pods.

### Design dimension 2: ComputeDomain & ResourceClaimTemplate lifecycle (managed path)

Lifecycle of ComputeDomains and RCTs is orthogonal to where we inject claims.

- Grove, given a PCS and its `computeDomainConfig`, computes the desired set of ComputeDomains/RCTs.
- In **PerReplica** mode:
  - For each PCS replica index, ensure there is:
    - One ComputeDomain instance.
    - One ResourceClaimTemplate instance that selects that ComputeDomain via CEL.
- These resources are owned by the PCS (or a PCS-replica abstraction) and are deleted when:
  - A replica is scaled down, or
  - The PCS is deleted.

### Open questions for discussion

- Do we want to ship **PerReplica only** initially, or support `scope=PerPCS` from day one?
- Are we comfortable with replica-specific wiring being visible primarily at the `PodClique` level?
- What is the desired failure behavior if the NVIDIA stack (ComputeDomain CRD/DRA) is not installed or misconfigured?


