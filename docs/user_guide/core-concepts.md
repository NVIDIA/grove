# Grove Core Concepts Tutorial

This tutorial provides a comprehensive overview of Grove's core concepts: **PodClique**, **PodCliqueSet**, and **PodCliqueScalingGroup**. Through practical examples, you'll learn how to deploy and scale inference workloads from simple single-node setups to complex multi-node distributed systems. Since Grove's creation was motivated by inference the examples are tailored to inference but the core idea is to demonstrate how Grove's primitives allow you to express a collection of single node and multinode components that require tighter coupling from a scheduling (and in future releases network topology) aspect.

## Prerequisites

Before starting this tutorial, ensure you have:
- [A Grove demo cluster running.](../installation.md#developing-grove) Make sure to run `make kind-up FAKE_NODES=40`, set `KUBECONFIG` env variable as directed in the instructions, and run `make deploy`
- [Or an actual k8s cluster with Grove installed.](../installation.md#deploying-grove) If you choose this path make sure to adjust the tolerations in the example to fit your cluster
- Basic understanding of Kubernetes concepts


## Core Concepts Overview

### PodClique: The Fundamental Unit
A **PodClique** is the core building block in Grove. It represents a group of pods with the same exact configuration (similar to a Deployment) that can be used in a standalone manner to represent single-node components of your inference system, or can represent roles within a multi-node component such as leader and worker.

### PodCliqueScalingGroup: Multi-Node Coordination
A **PodCliqueScalingGroup** coordinates multiple PodCliques that must scale together, preserving specified replica ratios across roles (e.g. leader/worker) in multi-node components.

### PodCliqueSet: The Inference Service Container
A **PodCliqueSet** contains all the inference components for a complete service. It manages one or more PodCliques or PodCliqueScalingGroups that work together to provide inference capabilities. Can be replicated in order to provide blue-green deployment and spread across availability zones.





---

## Example 1: Single-Node Aggregated Inference

In this simplest scenario, each pod is a complete model instance that can service requests. This is mapped to a single standalone PodClique within the PodCliqueSet. The PodClique provides horizontal scaling capabilities at the model replica level similar to a Deployment, and the PodCliqueSet provides horizontal scaling capabilities at the system level (useful for things such as blue-green deployments and spreading across availability zones).

```yaml
apiVersion: grove.io/v1alpha1
kind: PodCliqueSet
metadata:
  name: single-node-aggregated
  namespace: default
spec:
  replicas: 1
  template:
    cliques:
    - name: model-worker
      spec:
        replicas: 2
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: model-worker
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Model Worker (Aggregated) on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "1"
                memory: "2Gi"
```

### **Key Points:**
- Single PodClique named `model-worker`
- `replicas: 2` creates 2 instances for horizontal scaling
- Each replica handles complete inference pipeline
- Tolerations allow scheduling on fake nodes for demo, remove if you are trying to deploy on a real cluster

### **Deploy:**
```bash
# actual single-node-aggregated.yaml file is in samples/user_guide/concept_overview, change path accordingly
kubectl apply -f [single-node-aggregated.yaml](../../operator/samples/user_guide/concept_overview/single-node-aggregated.yaml)
kubectl get pods -l app.kubernetes.io/part-of=single-node-aggregated -o wide
```

If you are using the demo-cluster you should observe output similar to
```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=single-node-aggregated -o wide
NAME                                          READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
single-node-aggregated-0-model-worker-n9gcq   1/1     Running   0          18m   10.244.7.0    fake-node-007   <none>           <none>
single-node-aggregated-0-model-worker-zfhbb   1/1     Running   0          18m   10.244.15.0   fake-node-015   <none>           <none>
```
The demo-cluster consists of fake nodes spawned by [KWOK](https://kwok.sigs.k8s.io/) so the pods won't have any logs, but if you deployed to a real cluster you should observe the echo command complete successfully. Note that the spawned pods have descriptive names. Grove intentionally aims to allow users to immediately be able to map pods to their specifications in the yaml. All pods are prefixed with `single-node-aggregated-0` to represent they are part of the first replica of the `single-node-aggregated` PodCliqueSet. After the PodCliqueSet identifier is `model-worker`, signifying that the pods belong to the `model-worker` PodClique.

### **Scaling**
As mentioned earlier, you can scale the `model-worker` PodClique to get more model replicas similar to a Deployment. For instance run the following command to increase the replicas on `model-worker` from 2 to 4. `pclq` is short for PodClique and can be used to reference PodClique as a resource in kubectl commands. Note that the name of the PodClique provided to the scaling command is `single-node-aggregated-0-model-worker` and not just `model-worker`. This is necessary since the PodCliqueSet can be replicated (as we will see later) and therefore the name of PodCliques includes the PodCliqueSet replica they belong to.
```bash
kubectl scale pclq single-node-aggregated-0-model-worker --replicas=4
```
After running you will observe there are now 4 `model-worker` pods belonging to the `single-node-aggregated-0` PodCliqueSet
```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=single-node-aggregated -o wide
NAME                                          READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
single-node-aggregated-0-model-worker-jvgdd   1/1     Running   0          22s   10.244.11.0   fake-node-011   <none>           <none>
single-node-aggregated-0-model-worker-n9gcq   1/1     Running   0          44m   10.244.7.0    fake-node-007   <none>           <none>
single-node-aggregated-0-model-worker-tjb78   1/1     Running   0          22s   10.244.8.0    fake-node-008   <none>           <none>
single-node-aggregated-0-model-worker-zfhbb   1/1     Running   0          44m   10.244.15.0   fake-node-015   <none>           <none>
```
You can also scale the entire PodCliqueSet. For instance run the following command to increase the replicas on `single-node-aggregated` to 3. `pcs` is short for PodCliqueSet and can be used to reference PodCliqueSet as a resource in kubectl commands.

```bash
kubectl scale pcs single-node-aggregated --replicas=3
```
After running you will observe
```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=single-node-aggregated -o wide
NAME                                          READY   STATUS    RESTARTS   AGE    IP            NODE            NOMINATED NODE   READINESS GATES
single-node-aggregated-0-model-worker-2xl58   1/1     Running   0          99s    10.244.7.0    fake-node-007   <none>           <none>
single-node-aggregated-0-model-worker-5jlfr   1/1     Running   0          2m7s   10.244.20.0   fake-node-020   <none>           <none>
single-node-aggregated-0-model-worker-78974   1/1     Running   0          2m7s   10.244.5.0    fake-node-005   <none>           <none>
single-node-aggregated-0-model-worker-zn888   1/1     Running   0          99s    10.244.13.0   fake-node-013   <none>           <none>
single-node-aggregated-1-model-worker-kkmsq   1/1     Running   0          74s    10.244.10.0   fake-node-010   <none>           <none>
single-node-aggregated-1-model-worker-pn5cm   1/1     Running   0          74s    10.244.15.0   fake-node-015   <none>           <none>
single-node-aggregated-2-model-worker-h5xqk   1/1     Running   0          74s    10.244.3.0    fake-node-003   <none>           <none>
single-node-aggregated-2-model-worker-p4kjj   1/1     Running   0          74s    10.244.16.0   fake-node-016   <none>           <none>
```

Note how there are pods belonging to `single-node-aggregated-0`, `single-node-aggregated-1`, and `single-node-aggregated-2`, representing 3 different PodCliqueSets. Also note how `single-node-aggregated-1` and `single-node-aggregated-2` only have two replicas in their `model-worker` PodClique. This is in line with k8s patterns and occurs because the template that was applied (single-node-aggregated.yaml) specified the number of replicas on `model-worker` as 2. To scale them up you would have to apply `kubectl scale pclq` commands like previously done for `single-node-aggregated-0-model-worker` above.

### Cleanup
To teardown the example delete the `single-node-aggregated` PodCliqueSet, the operator will tear down all the constituent pieces

```bash
kubectl delete pcs single-node-aggregated
```

---

## Example 2: Single-Node Disaggregated Inference

Here we separate prefill and decode operations into different workers, allowing independent scaling of each component. Modelling this in Grove primitives is simple, in the previous example that demonstrated aggregated serving, we had one PodClique for the model-worker, which handled both prefill and decode. To disaggregate prefill and decode, we simply create two PodCliques, one for prefill, and one for decode. Note that the clique names can be set to whatever your want, although we recommend setting them up to match the component they represent (e.g prefill, decode).

```yaml
apiVersion: grove.io/v1alpha1
kind: PodCliqueSet
metadata:
  name: single-node-disaggregated
  namespace: default
spec:
  replicas: 1
  template:
    cliques:
    - name: prefill
      spec:
        roleName: prefill
        replicas: 3
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: prefill
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Prefill Worker on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "2"
                memory: "4Gi"
    - name: decode
      spec:
        roleName: decode
        replicas: 2
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: decode
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Decode Worker on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "1"
                memory: "2Gi"
```

### **Key Points:**
- Two separate PodCliques: `prefill` and `decode`
- Independent scaling: We start with 3 prefill workers, 2 decode workers and can scale them independently based on workload characteristics
- Different resource requirements for each component are supported (in the example prefill requests 2 cpu and decode only 1)

### **Deploy**
```bash
# actual single-node-disaggregated.yaml file is in samples/user_guide/concept_overview, change path accordingly
kubectl apply -f [single-node-disaggregated.yaml](../../operator/samples/user_guide/concept_overview/single-node-disaggregated.yaml)
kubectl get pods -l app.kubernetes.io/part-of=single-node-disaggregated -o wide
```
After running you will observe

```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=single-node-disaggregated -o wide
NAME                                        READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
single-node-disaggregated-0-decode-bvl94    1/1     Running   0          29s   10.244.6.0    fake-node-006   <none>           <none>
single-node-disaggregated-0-decode-wlqxj    1/1     Running   0          29s   10.244.4.0    fake-node-004   <none>           <none>
single-node-disaggregated-0-prefill-tnw26   1/1     Running   0          29s   10.244.17.0   fake-node-017   <none>           <none>
single-node-disaggregated-0-prefill-xvvtk   1/1     Running   0          29s   10.244.11.0   fake-node-011   <none>           <none>
single-node-disaggregated-0-prefill-zglvn   1/1     Running   0          29s   10.244.9.0    fake-node-009   <none>           <none>
```
Note how within the `single-node-disaggregated-0` PodCliqueSet replica there are pods from the `prefill` PodClique and `decode` PodClique

### **Scaling**
You can scale the `prefill` and `decode` PodCliques the same way the [`model-worker` PodClique was scaled](#scaling) in the previous example. 

Additionally, the `single-node-disaggregated` PodCliqueSet can be scaled the same way the `single-node-aggregated` PodCliqueSet was scaled in the previous example. We show an exampleto demonstrate how when PodCliqueSets are scaled, all constituent PodCliques are replicated, underscoring why scaling PodCliqueSets should be treated as scaling the entire system (usually for blue-green deployment or high availability across zones).

```bash
kubectl scale pcs single-node-aggregated --replicas=2
```
After running this you will observe
```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=single-node-disaggregated -o wide
NAME                                        READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
single-node-disaggregated-0-decode-9fvsj    1/1     Running   0          77s   10.244.13.0   fake-node-013   <none>           <none>
single-node-disaggregated-0-decode-xw62b    1/1     Running   0          77s   10.244.18.0   fake-node-018   <none>           <none>
single-node-disaggregated-0-prefill-dfss8   1/1     Running   0          77s   10.244.8.0    fake-node-008   <none>           <none>
single-node-disaggregated-0-prefill-fgkrc   1/1     Running   0          77s   10.244.14.0   fake-node-014   <none>           <none>
single-node-disaggregated-0-prefill-ljnms   1/1     Running   0          77s   10.244.11.0   fake-node-011   <none>           <none>
single-node-disaggregated-1-decode-f9tmf    1/1     Running   0          10s   10.244.16.0   fake-node-016   <none>           <none>
single-node-disaggregated-1-decode-psd6h    1/1     Running   0          10s   10.244.10.0   fake-node-010   <none>           <none>
single-node-disaggregated-1-prefill-2mktc   1/1     Running   0          10s   10.244.7.0    fake-node-007   <none>           <none>
single-node-disaggregated-1-prefill-4smsf   1/1     Running   0          10s   10.244.3.0    fake-node-003   <none>           <none>
single-node-disaggregated-1-prefill-5n6qv   1/1     Running   0          10s   10.244.12.0   fake-node-012   <none>           <none>
```
Note how now there is `single-node-disaggregated-0` and `single-node-disaggregated-1` each with their own `prefill` and `decode` PodCliques that can be scaled.

### Cleanup
To teardown the example delete the `single-node-disaggregated` PodCliqueSet, the operator will tear down all the constituent pieces

```bash
kubectl delete pcs single-node-disaggregated
```

---

## Example 3: Multi-Node Aggregated Inference

Now we introduce **PodCliqueScalingGroup** for multi-node deployments, where multiple pods collectively make up a single instance of the application and must scale together.
These setups are increasingly common for serving large models that do not fit on one node and consequently one model instance ends up spanning multiple nodes and therefore multiple pods. In thse cases, inference frameworks typically follow a leader-worker topology: one leader pod coordinates work for N workers that connect to it.
Scaling out means replicating the entire unit (1 leader + N workers) to create additional model instances.
A PodCliqueScalingGroup encodes this by grouping the relevant PodCliques and scaling them in lockstep while preserving the pod ratios.
The example below shows how to model this leader-worker pattern in Grove:

```yaml
apiVersion: grove.io/v1alpha1
kind: PodCliqueSet
metadata:
  name: multinode-aggregated
  namespace: default
spec:
  replicas: 1
  template:
    cliques:
    - name: leader
      spec:
        roleName: leader
        replicas: 1
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: model-leader
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Model Leader (Aggregated) on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "2"
                memory: "4Gi"
    - name: worker
      spec:
        roleName: worker
        replicas: 3
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: model-worker
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Model Worker (Aggregated) on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "4"
                memory: "8Gi"
    podCliqueScalingGroups:
    - name: model-instance
      cliqueNames: [leader, worker]
      replicas: 2
```

### **Key Points:**
- **PodCliqueScalingGroup** named `model-cluster` with `replicas: 2`
- Creates 2 model isntances, each with 1 leader + 3 workers
- Total pods: 2 × (1 leader + 3 workers) = 8 pods
- Scaling the group preserves the 1:3 leader-to-worker ratio

### **Deploy:**
```bash
# actual multi-node-aggregated.yaml file is in samples/user_guide/concept_overview, change path accordingly
kubectl apply -f [multi-node-aggregated.yaml](../../operator/samples/user_guide/concept_overview/multi-node-aggregated.yaml)
kubectl get pods -l app.kubernetes.io/part-of=multinode-aggregated -o wide
```
After running you should observe

```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=multinode-aggregated -o wide
NAME                                                   READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
multinode-aggregated-0-model-instance-0-leader-zq4j5   1/1     Running   0          11s   10.244.2.0    fake-node-002   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-7kcv7   1/1     Running   0          11s   10.244.13.0   fake-node-013   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-829k9   1/1     Running   0          11s   10.244.7.0    fake-node-007   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-vrmrb   1/1     Running   0          11s   10.244.10.0   fake-node-010   <none>           <none>
multinode-aggregated-0-model-instance-1-leader-t8ptp   1/1     Running   0          11s   10.244.6.0    fake-node-006   <none>           <none>
multinode-aggregated-0-model-instance-1-worker-bscfv   1/1     Running   0          11s   10.244.4.0    fake-node-004   <none>           <none>
multinode-aggregated-0-model-instance-1-worker-sgd6r   1/1     Running   0          11s   10.244.17.0   fake-node-017   <none>           <none>
multinode-aggregated-0-model-instance-1-worker-vpkwb   1/1     Running   0          11s   10.244.18.0   fake-node-018   <none>           <none>
```
Note how within the same `multinode-aggregated-0` PodCliqueSet there are two replicas of the `model-instance` PodCliqueScalingGroup, `model-instance-0` and `model-instance-1`, each consisting of a `leader` PodClique with one replica and a `worker` PodClique with 3 replicas.

### **Scaling**

As mentioned before, PodCliqueScalingGroups represent "super-pods" where scaling means replicating the pods in constituent PodCliques together while preserving the ratios. To illustrate this, run the following command to scale the replicas of the `model-instance` PodCliqueScalingGroup from two to three. `pcsg` is short for PodCliqueScalingGroup and can be used to reference PodCliqueScalingGroup as a resource in kubectl commands. Similar to standalone PodCliques, PodCliqueScalingGroups include the name of the PodCliqueSet in their name to disambiguate from replicas of the same PodCliqueScalingGroup in a different PodCliqueSet. This is why the scaling command references `multinode-aggregated-0-model-instance` instead of `model-instance`

```bash
kubectl scale pcsg multinode-aggregated-0-model-instance --replicas=3
```
After running this command you should observe

```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=multinode-aggregated -o wide
NAME                                                   READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
multinode-aggregated-0-model-instance-0-leader-zq4j5   1/1     Running   0          68m   10.244.2.0    fake-node-002   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-7kcv7   1/1     Running   0          68m   10.244.13.0   fake-node-013   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-829k9   1/1     Running   0          68m   10.244.7.0    fake-node-007   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-vrmrb   1/1     Running   0          68m   10.244.10.0   fake-node-010   <none>           <none>
multinode-aggregated-0-model-instance-1-leader-t8ptp   1/1     Running   0          68m   10.244.6.0    fake-node-006   <none>           <none>
multinode-aggregated-0-model-instance-1-worker-bscfv   1/1     Running   0          68m   10.244.4.0    fake-node-004   <none>           <none>
multinode-aggregated-0-model-instance-1-worker-sgd6r   1/1     Running   0          68m   10.244.17.0   fake-node-017   <none>           <none>
multinode-aggregated-0-model-instance-1-worker-vpkwb   1/1     Running   0          68m   10.244.18.0   fake-node-018   <none>           <none>
multinode-aggregated-0-model-instance-2-leader-w5wfm   1/1     Running   0          25s   10.244.19.0   fake-node-019   <none>           <none>
multinode-aggregated-0-model-instance-2-worker-59qm9   1/1     Running   0          25s   10.244.14.0   fake-node-014   <none>           <none>
multinode-aggregated-0-model-instance-2-worker-9qqnx   1/1     Running   0          25s   10.244.20.0   fake-node-020   <none>           <none>
multinode-aggregated-0-model-instance-2-worker-qqnl8   1/1     Running   0          25s   10.244.5.0    fake-node-005   <none>           <none>
```
Note how now there is now an additional leader pod `multinode-aggregated-0-model-instance-2-leader` and 3 additional worker pods `multinode-aggregated-0-model-instance-2-leader`. This demonstrates how PodCliqueScalingGroups allow you to create "super-pods" that are a group of pods that scale together.

While you can scale the PodCliqueScalingGroup to replicate the "super-pod" unit, you can still scale the individual PodCliques on a given PodCliqueScalingGroup replica. Before showing an example of that it is important to explain that the naming format of PodCliques that are in a PodCliqueScalingGroup is different than for standalone PodCliques. For standalone PodCliques the format is `<pcs-name>-<pcs-replica-idx>-<pclq-name>` whereas for PodCliques that are part of a PodCliqueScalingGroup, the format is `<pcs-name>-<pcs-replica-idx>-<pcsg-name>-<pcsg-replica-idx>-<pclq-name>`. To illustrate this run the following command to show the names of the leader and worker PodCliques

```bash
kubectl get pclq
```
After running this you should observe the following PodCliques, with the naming format in line with what we described above.
```
rohanv@rohanv-mlt operator % kubectl get pclq
NAME                                             AGE
multinode-aggregated-0-model-instance-0-leader   95m
multinode-aggregated-0-model-instance-0-worker   95m
multinode-aggregated-0-model-instance-1-leader   95m
multinode-aggregated-0-model-instance-1-worker   95m
multinode-aggregated-0-model-instance-2-leader   27m
multinode-aggregated-0-model-instance-2-worker   27m
```
Now that we know the PodClique names we can scale the replicas on a specific PodClique similar to previous examples. Run the following command to increase `multinode-aggregated-0-model-instance-0-worker` from three replicas to four

```bash
kubectl scale pclq multinode-aggregated-0-model-instance-0-worker --replicas=4
```
After running this you will observe:

```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=multinode-aggregated -o wide
NAME                                                   READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
multinode-aggregated-0-model-instance-0-leader-zq4j5   1/1     Running   0          12h   10.244.2.0    fake-node-002   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-7kcv7   1/1     Running   0          12h   10.244.13.0   fake-node-013   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-829k9   1/1     Running   0          12h   10.244.7.0    fake-node-007   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-gjc87   1/1     Running   0          83s   10.244.1.0    fake-node-001   <none>           <none>
multinode-aggregated-0-model-instance-0-worker-vrmrb   1/1     Running   0          12h   10.244.10.0   fake-node-010   <none>           <none>
multinode-aggregated-0-model-instance-1-leader-t8ptp   1/1     Running   0          12h   10.244.6.0    fake-node-006   <none>           <none>
multinode-aggregated-0-model-instance-1-worker-bscfv   1/1     Running   0          12h   10.244.4.0    fake-node-004   <none>           <none>
multinode-aggregated-0-model-instance-1-worker-sgd6r   1/1     Running   0          12h   10.244.17.0   fake-node-017   <none>           <none>
multinode-aggregated-0-model-instance-1-worker-vpkwb   1/1     Running   0          12h   10.244.18.0   fake-node-018   <none>           <none>
multinode-aggregated-0-model-instance-2-leader-w5wfm   1/1     Running   0          11h   10.244.19.0   fake-node-019   <none>           <none>
multinode-aggregated-0-model-instance-2-worker-59qm9   1/1     Running   0          11h   10.244.14.0   fake-node-014   <none>           <none>
multinode-aggregated-0-model-instance-2-worker-9qqnx   1/1     Running   0          11h   10.244.20.0   fake-node-020   <none>           <none>
multinode-aggregated-0-model-instance-2-worker-qqnl8   1/1     Running   0          11h   10.244.5.0    fake-node-005   <none>           <none>
```
Note how there are now four pods belonging to `multinode-aggregated-0-model-instance-0-worker`

Overall, you can scale the PodCliqueScalingGroup to scale a multi-node component and "super-pod", but you can also still scale the PodCliques that make up a PodCliqueScalingGroup (this will likely become more relevant ones inference frameworks have elastic world sizes)

### Cleanup
To teardown the example delete the `multinode-aggregated` PodCliqueSet, the operator will tear down all the constituent pieces

```bash
kubectl delete pcs multinode-aggregated
```

---

## Example 4: Multi-Node Disaggregated Inference

You can put together all the things we've covered to represent the most complex scenario: multi-node disaggregated serving where both the prefill and decode components are multi-node. We represent this in Grove by creating PodCliqueScalingGroups for both prefill and decode. Additionally each PodCliqueScalingGroup consists of two PodCliques, one for the leader and one for the worker. 

```yaml
apiVersion: grove.io/v1alpha1
kind: PodCliqueSet
metadata:
  name: multinode-disaggregated
  namespace: default
spec:
  replicas: 1
  template:
    cliques:
    - name: pleader
      spec:
        roleName: pleader
        replicas: 1
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: prefill-leader
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Prefill Leader on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "2"
                memory: "4Gi"
    - name: pworker
      spec:
        roleName: pworker
        replicas: 4
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: prefill-worker
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Prefill Worker on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "4"
                memory: "8Gi"
    - name: dleader
      spec:
        roleName: dleader
        replicas: 1
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: decode-leader
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Decode Leader on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "1"
                memory: "2Gi"
    - name: dworker
      spec:
        roleName: dworker
        replicas: 2
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: decode-worker
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Decode Worker on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "2"
                memory: "4Gi"
    podCliqueScalingGroups:
    - name: prefill
      cliqueNames: [pleader, pworker]
      replicas: 2
    - name: decode
      cliqueNames: [dleader, dworker]
      replicas: 1
```

### **Key Points:**
- Two independent **PodCliqueScalingGroups**: `prefill` and `decode`
- Each PodCliqueScalingGroup (PCSG) has PodCliques for leader and worker `pleader`,`pworker`,`dleader`,`dworker`. PodClique names need to be unique within a PodCliqueSet which is why we do not name the PodCliques `leader` and `worker` unlike the previous example
- Prefill PCSG consists of : 2 replicas × (1 leader + 4 workers) = 10 pods
- Decode PCSG: 1 replica × (1 leader + 2 workers) = 3 pods
- Each PCSG can scale independently based on workload demands
- Each PCSG can have different resource allocations

### **Deploy**
```bash
# actual multi-node-disaggregated.yaml is under /operator/samples/user_guide/concept_overview. Adjust paths accordingly
kubectl apply -f [multi-node-disaggregated.yaml](../../operator/samples/user_guide/concept_overview/multi-node-disaggregated.yaml)
kubectl get pods -l app.kubernetes.io/part-of=multinode-disaggregated -o wide
```
After running you will observe
```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=multinode-disaggregated -o wide
NAME                                                READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
multinode-disaggregated-0-decode-0-dleader-khqxf    1/1     Running   0          35s   10.244.19.0   fake-node-019   <none>           <none>
multinode-disaggregated-0-decode-0-dworker-6d7cq    1/1     Running   0          35s   10.244.18.0   fake-node-018   <none>           <none>
multinode-disaggregated-0-decode-0-dworker-g6ksp    1/1     Running   0          35s   10.244.20.0   fake-node-020   <none>           <none>
multinode-disaggregated-0-prefill-0-pleader-f5w5j   1/1     Running   0          35s   10.244.6.0    fake-node-006   <none>           <none>
multinode-disaggregated-0-prefill-0-pworker-7spmm   1/1     Running   0          35s   10.244.9.0    fake-node-009   <none>           <none>
multinode-disaggregated-0-prefill-0-pworker-jgnkq   1/1     Running   0          35s   10.244.10.0   fake-node-010   <none>           <none>
multinode-disaggregated-0-prefill-0-pworker-v49gf   1/1     Running   0          35s   10.244.11.0   fake-node-011   <none>           <none>
multinode-disaggregated-0-prefill-0-pworker-xst4z   1/1     Running   0          35s   10.244.2.0    fake-node-002   <none>           <none>
multinode-disaggregated-0-prefill-1-pleader-xwf45   1/1     Running   0          35s   10.244.16.0   fake-node-016   <none>           <none>
multinode-disaggregated-0-prefill-1-pworker-6jrpz   1/1     Running   0          35s   10.244.15.0   fake-node-015   <none>           <none>
multinode-disaggregated-0-prefill-1-pworker-bd5ct   1/1     Running   0          35s   10.244.14.0   fake-node-014   <none>           <none>
multinode-disaggregated-0-prefill-1-pworker-fdl7s   1/1     Running   0          35s   10.244.7.0    fake-node-007   <none>           <none>
multinode-disaggregated-0-prefill-1-pworker-kpplp   1/1     Running   0          35s   10.244.4.0    fake-node-004   <none>           <none>
```
Note how we have one replica of the decode PodCliqueScalingGroup and two replicas of the prefill PodCliqueScalingGroup. Also note how each prefill replica consists of 4 pods whereas each decode replica consists of 3 pods. This independence is critical to disaggregated serving as you can independently specify and scale prefill and decode components.

### **Scaling**
Each of the PodCliqueScalingGroups and PodCliques can be scaled similar to the [previous example](#scaling-3). If you scale a PodCliqueScalingGroup it will replicate all its PodCliques while maintaining the replica ratio between them. If you scale a PodClique it will horizontally scale like a deployment.

### Cleanup
To teardown the example delete the `multinode-disaggregated` PodCliqueSet, the operator will tear down all the constituent pieces

```bash
kubectl delete pcs multinode-disaggregated
```
---

## Example 5: Complete Inference Pipeline

The examples above have focused on mapping various inference workloads into Grove primitives, focusing on the model instances. However, the primitives are generic and the point of Grove is to allow the user to represent as many components as they'd like. To illustrate this point we now provide an example where we represent additional components such as a frontend and vision encoder. To add additional components you simply add additional PodCliques and PodCliqueScalingGroups into the PodCliqueSet

```yaml
apiVersion: grove.io/v1alpha1
kind: PodCliqueSet
metadata:
  name: comp-inf-ppln
  namespace: default
spec:
  replicas: 1
  template:
    cliques:
    #single node components
    - name: frontend
      spec:
        roleName: frontend
        replicas: 2
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: frontend
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Frontend Service on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "0.5"
                memory: "1Gi"
    - name: vision-encoder
      spec:
        roleName: vision-encoder
        replicas: 1
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: vision-encoder
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Vision Encoder on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "3"
                memory: "6Gi"
    # Multi-node components
    - name: pleader
      spec:
        roleName: pleader
        replicas: 1
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: prefill-leader
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Prefill Leader on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "2"
                memory: "4Gi"
    - name: pworker
      spec:
        roleName: pworker
        replicas: 4
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: prefill-worker
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Prefill Worker on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "4"
                memory: "8Gi"
    - name: dleader
      spec:
        roleName: dleader
        replicas: 1
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: decode-leader
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Decode Leader on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "1"
                memory: "2Gi"
    - name: dworker
      spec:
        roleName: dworker
        replicas: 2
        podSpec:
          tolerations:
          - key: fake-node
            operator: Equal
            value: "true"
            effect: NoSchedule
          containers:
          - name: decode-worker
            image: nginx:latest
            command: ["/bin/sh"]
            args: ["-c", "echo 'Decode Worker on node:' && hostname && sleep 3600"]
            resources:
              requests:
                cpu: "2"
                memory: "4Gi"
    podCliqueScalingGroups:
    - name: prefill
      cliqueNames: [pleader, pworker]
      replicas: 1
    - name: decode-cluster
      cliqueNames: [dleader, dworker]
      replicas: 1
```

**Architecture Summary:**
- **Single-node components**: Frontend (2 replicas), Vision Encoder (1 replica)
- **Multi-node prefill**: 1 replica × (1 leader + 4 workers) = 5 pods
- **Multi-node decode**: 1 replica × (1 leader + 2 workers) = 3 pods
- **Total**: 11 pods providing a complete inference pipeline

**Deploy and explore:**
```bash
# Actual complete-inference-pipeline.yaml is under /operator/samples/user_guide/concept_overview, adjust path accordingly
kubectl apply -f complete-inference-pipeline.yaml
kubectl get pods -l app.kubernetes.io/part-of=comp-inf-ppln -o wide
```
After running you will observe
```
rohanv@rohanv-mlt operator % kubectl get pods -l app.kubernetes.io/part-of=comp-inf-ppln -o wide
NAME                                             READY   STATUS    RESTARTS   AGE   IP            NODE            NOMINATED NODE   READINESS GATES
comp-inf-ppln-0-decode-cluster-0-dleader-wr7r2   1/1     Running   0          51s   10.244.8.0    fake-node-008   <none>           <none>
comp-inf-ppln-0-decode-cluster-0-dworker-4nm98   1/1     Running   0          51s   10.244.5.0    fake-node-005   <none>           <none>
comp-inf-ppln-0-decode-cluster-0-dworker-wqzb9   1/1     Running   0          51s   10.244.2.0    fake-node-002   <none>           <none>
comp-inf-ppln-0-frontend-fxxsg                   1/1     Running   0          51s   10.244.1.0    fake-node-001   <none>           <none>
comp-inf-ppln-0-frontend-shp8h                   1/1     Running   0          51s   10.244.20.0   fake-node-020   <none>           <none>
comp-inf-ppln-0-prefill-0-pleader-vgz8n          1/1     Running   0          51s   10.244.17.0   fake-node-017   <none>           <none>
comp-inf-ppln-0-prefill-0-pworker-95jls          1/1     Running   0          51s   10.244.9.0    fake-node-009   <none>           <none>
comp-inf-ppln-0-prefill-0-pworker-k8bck          1/1     Running   0          51s   10.244.4.0    fake-node-004   <none>           <none>
comp-inf-ppln-0-prefill-0-pworker-qlsb9          1/1     Running   0          51s   10.244.14.0   fake-node-014   <none>           <none>
comp-inf-ppln-0-prefill-0-pworker-wfxdg          1/1     Running   0          51s   10.244.15.0   fake-node-015   <none>           <none>
comp-inf-ppln-0-vision-encoder-rwvz5             1/1     Running   0          51s   10.244.7.0    fake-node-007   <none>           <none>
```

### Cleanup
To teardown the example delete the `comp-inf-ppln` PodCliqueSet, the operator will tear down all the constituent pieces

```bash
kubectl delete pcs comp-inf-ppln
```
---

## Key Takeaways

Overall Grove primitives aim to provide a declarative way to express all the components of your system, allowing you to stitch together an arbitrary amount of single-node and multi-node components.

### When to Use Each Component

1. **PodClique**:
  -Standalone Manner (not part of PodCliqueScalingGroup)
   - Single-node components that can scale independently
   - Examples: Frontend, API gateway, single-node model instances
  -Within a PodCliqueScalingGroup
   - Specific roles within a multi-node component, e.g. leader, worker

2. **PodCliqueScalingGroup**:
   - Multi-node components where one instance spans multiple pods and there are potentially different roles each pod takes (e.g. leader worker)
   - When scaled creates new copy of constituent PodCliques while maintaining the ratio between them

3. **PodCliqueSet**:
   - Top level Custom Resource for representing the entire system
   - Allows for replicating the entire system for blue-green deployments and/or availability across zones
   - Contains user specified number of PodCliques and PodCliqueScalingGroups





---

## Cleanup

Remove all examples:
```bash
kubectl delete podcliqueset single-node-aggregated single-node-disaggregated multinode-aggregated multinode-disaggregated complete-inference-pipeline
```
