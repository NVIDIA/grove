# Getting Started

To get started with Grove, you would need a running Kubernetes cluster. As there is no released version of Grove operator just yet, we recommend you try out Grove locally using a [kind](https://kind.sigs.k8s.io/) cluster, using the make targets we provide as a part of the repository.

> [!NOTE]
> In case you would like to run Grove operator in your own Kubernetes cluster, you could build your own images of Grove operator and init container. This can be done through the provided make targets in [operator/Makefile](../operator/Makefile).
> Specify your image in the operator's Helm charts in [values.yaml](../operator/charts/values.yaml); and install the Helm charts.

## Local kind cluster set-up

All grove operator Make targets are located in [Operator Makefile][./operator/Makefile].

### Kind cluster set-up

- To set up a KIND cluster with local docker registry run the following command:

  ```bash
  make kind-up
  ```

- Specify the `KUBECONFIG` environment variable in your shell session to the path printed out at the end of the previous step:

  ```bash
  # You would see something like `export KUBECONFIG=/path-to-your-grove-clone/grove/operator/hack/kind/kubeconfig` printed.
  # If you are already in `/path-to-your-grove-clone/grove/operator`, then you can simply:
  export KUBECONFIG=./hack/kind/kubeconfig
  ```

### Kubernetes cluster set-up

If you want to use your own Kubernetes cluster instead of the KIND cluster, follow these steps:

- **Set the KUBECONFIG environment variable** to point to your Kubernetes cluster configuration:

   ```bash
   # Set KUBECONFIG to use your Kubernetes cluster kubeconfig
   export KUBECONFIG=/path/to/your/kubernetes/kubeconfig
   ```

- **Set the CONTAINER_REGISTRY environment variable** to specify your container registry:

   ```bash
   # Set a container registry to push your images to
   export CONTAINER_REGISTRY=your-container-registry
   ```

### Deploy Grove operator

```bash
# If you wish to deploy all Grove Operator resources in a custom namespace then set the `NAMESPACE` environment variable
export NAMESPACE=custom-ns
# if `NAMESPACE` environment variable is set then `make deploy-local` target will use this namespace to deploy all Grove operator resources
make deploy-local
```

This make target installs all relevant CRDs, builds `grove-operator`, `grove-initc`, and deploys the operator to the kind cluster.
You can configure the Grove operator by modifying the [values.yaml](../operator/charts/values.yaml).

This make target leverages Grove [Helm](https://helm.sh/) charts and [Skaffold](https://skaffold.dev/) to install the following resources to the KIND cluster:

- [CRDs](../grove/operator/charts):
  - Grove operator CRD - `podgangsets.grove.io`, `podcliques.grove.io` and `podcliquescalinggroups.grove.io`.
  - Grove Scheduler CRDs - `podgangs.scheduler.grove.io`.
- All Grove operator resources defined as a part of [Grove Helm chart templates](../operator/charts/templates).

### Deploy a `PodGangSet`

- Deploy one of the samples present in the [samples](./operator/samples/simple) directory.

  ```bash
  kubectl apply -f ./samples/simple/simple1.yaml
  ```

- You can now fetch resources like `PodGangSet` (pgs), `PodGang` (pg), `PodClique` (pclq), `PodCliqueScalingGroup` (pcsg), etc. created by the Grove operator, by running:

  ```bash
  kubectl get pgs,pclq,pcsg,pg,pod -owide
  ```

  You would see output like this:

  ```bash
  ❯ kubectl get pgs,pclq,pcsg,pg,pod -owide
  NAME                          AGE
  podgangset.grove.io/simple1   34s

  NAME                                     AGE
  podclique.grove.io/simple1-0-pca         33s
  podclique.grove.io/simple1-0-pcd         33s
  podclique.grove.io/simple1-0-sga-0-pcb   33s
  podclique.grove.io/simple1-0-sga-0-pcc   33s

  NAME                                           AGE
  podcliquescalinggroup.grove.io/simple1-0-sga   33s

  NAME                                   AGE
  podgang.scheduler.grove.io/simple1-0   33s

  NAME                                  READY   STATUS    RESTARTS   AGE
  pod/grove-operator-699c77979f-7x2zc   1/1     Running   0          51s
  pod/simple1-0-pca-pkl2b               1/1     Running   0          33s
  pod/simple1-0-pca-s7dz2               1/1     Running   0          33s
  pod/simple1-0-pca-wjfqz               1/1     Running   0          33s
  pod/simple1-0-pcd-l4vnk               1/1     Running   0          33s
  pod/simple1-0-pcd-s7687               1/1     Running   0          33s
  pod/simple1-0-sga-0-pcb-m9shj         1/1     Running   0          33s
  pod/simple1-0-sga-0-pcb-vnrqw         1/1     Running   0          33s
  pod/simple1-0-sga-0-pcc-g8rg8         1/1     Running   0          33s
  pod/simple1-0-sga-0-pcc-hx4zn         1/1     Running   0          33s
  ```

### Scaling

As specified in the [README.md](../README.md) and the [docs](../docs), there are multiple hierarchies at which you can scale resources in a `PodGangSet`.

- Let's try scaling the `PodCliqueScalingGroup` from 1 to 2 replicas:

  ```bash
  kubectl scale pcsg simple1-0-pcsg --replicas=2
  ```

  This will create new pods that associate with cliques that belong to this scaling group, and their associated `PodGang`s.

  Fetching all resources to check the newly created resources:

  ```bash
  ❯ kubectl get pgs,pclq,pcsg,pg,pod -owide
  NAME                          AGE
  podgangset.grove.io/simple1   2m28s

  NAME                                     AGE
  podclique.grove.io/simple1-0-pca         2m27s
  podclique.grove.io/simple1-0-pcd         2m27s
  podclique.grove.io/simple1-0-sga-0-pcb   2m27s
  podclique.grove.io/simple1-0-sga-0-pcc   2m27s
  podclique.grove.io/simple1-0-sga-1-pcb   15s # newly created `PodClique`, as a consequence of scaling `PodCliqueScalingGroup`
  podclique.grove.io/simple1-0-sga-1-pcc   15s # newly created `PodClique`, as a consequence of scaling `PodCliqueScalingGroup`

  NAME                                           AGE
  podcliquescalinggroup.grove.io/simple1-0-sga   2m27s

  NAME                                         AGE
  podgang.scheduler.grove.io/simple1-0         2m27s
  podgang.scheduler.grove.io/simple1-0-sga-1   15s # newly created `PodGang`, as a consequence of scaling `PodCliqueScalingGroup`

  NAME                                  READY   STATUS    RESTARTS   AGE
  pod/grove-operator-699c77979f-7x2zc   1/1     Running   0          2m45s
  pod/simple1-0-pca-pkl2b               1/1     Running   0          2m27s
  pod/simple1-0-pca-s7dz2               1/1     Running   0          2m27s
  pod/simple1-0-pca-wjfqz               1/1     Running   0          2m27s
  pod/simple1-0-pcd-l4vnk               1/1     Running   0          2m27s
  pod/simple1-0-pcd-s7687               1/1     Running   0          2m27s
  pod/simple1-0-sga-0-pcb-m9shj         1/1     Running   0          2m27s
  pod/simple1-0-sga-0-pcb-vnrqw         1/1     Running   0          2m27s
  pod/simple1-0-sga-0-pcc-g8rg8         1/1     Running   0          2m27s
  pod/simple1-0-sga-0-pcc-hx4zn         1/1     Running   0          2m27s
  pod/simple1-0-sga-1-pcb-mbpjx         1/1     Running   0          15s # newly created Pod, as a consequence of scaling
  pod/simple1-0-sga-1-pcb-v7h2d         1/1     Running   0          15s # newly created Pod, as a consequence of scaling
  pod/simple1-0-sga-1-pcc-48fh5         1/1     Running   0          15s # newly created Pod, as a consequence of scaling
  pod/simple1-0-sga-1-pcc-l9bgd         1/1     Running   0          15s # newly created Pod, as a consequence of scaling
  ```

  This scales the `PodCliques` `pcb`, and `pcc` as a group, and in-turn doubling the number of replicas of Pods that belong to `pcb`, and `pcc`, as can be seen in the specification of this sample in the [`podCliqueScalingGroups`](../operator/samples/simple/simple1.yaml) section.

  Similarly, the `PodCliqueScalingGroup` can be scaled back in to 1 replicas like so:

  ```bash
  kubectl scale pcsg simple1-0-pcsg --replicas=1
  ```

- Scaling can also be triggered at the `PodGangSet` level, as can be seen here:

  ```bash
  kubectl scale pgs simple1 --replicas=2
  ```

  ```bash
  ❯ kubectl get pgs,pclq,pcsg,pg,pod -owide
  NAME                          AGE
  podgangset.grove.io/simple1   6m25s

  NAME                                     AGE
  podclique.grove.io/simple1-0-pca         6m24s
  podclique.grove.io/simple1-0-pcd         6m24s
  podclique.grove.io/simple1-0-sga-0-pcb   6m24s
  podclique.grove.io/simple1-0-sga-0-pcc   6m24s
  podclique.grove.io/simple1-1-pca         51s # newly created `PodClique`, as a consequence of scaling `PodGangSet`
  podclique.grove.io/simple1-1-pcd         51s # newly created `PodClique`, as a consequence of scaling `PodGangSet`
  podclique.grove.io/simple1-1-sga-0-pcb   51s # newly created `PodClique`, as a consequence of scaling `PodGangSet`
  podclique.grove.io/simple1-1-sga-0-pcc   51s # newly created `PodClique`, as a consequence of scaling `PodGangSet`

  NAME                                           AGE
  podcliquescalinggroup.grove.io/simple1-0-sga   6m24s
  podcliquescalinggroup.grove.io/simple1-1-sga   51s # newly created `PodCliqueScalingGroup`, as a consequence of scaling `PodGangSet`

  NAME                                   AGE
  podgang.scheduler.grove.io/simple1-0   6m24s
  podgang.scheduler.grove.io/simple1-1   51s # newly created `PodGang`, as a consequence of scaling `PodGangSet`

  NAME                                  READY   STATUS    RESTARTS   AGE
  pod/grove-operator-699c77979f-7x2zc   1/1     Running   0          6m42s
  pod/simple1-0-pca-pkl2b               1/1     Running   0          6m24s
  pod/simple1-0-pca-s7dz2               1/1     Running   0          6m24s
  pod/simple1-0-pca-wjfqz               1/1     Running   0          6m24s
  pod/simple1-0-pcd-l4vnk               1/1     Running   0          6m24s
  pod/simple1-0-pcd-s7687               1/1     Running   0          6m24s
  pod/simple1-0-sga-0-pcb-m9shj         1/1     Running   0          6m24s
  pod/simple1-0-sga-0-pcb-vnrqw         1/1     Running   0          6m24s
  pod/simple1-0-sga-0-pcc-g8rg8         1/1     Running   0          6m24s
  pod/simple1-0-sga-0-pcc-hx4zn         1/1     Running   0          6m24s
  pod/simple1-1-pca-29njw               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pca-9cgm8               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pca-lhrw8               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pcd-6fjzm               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pcd-n288d               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-sga-0-pcb-5kvzx         1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-sga-0-pcb-h8g8l         1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-sga-0-pcc-6gxfb         1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-sga-0-pcc-rqfqf         1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodGangSet`
  ```

  Similarly, the `PodGangSet` can be scaled back in to 1 replicas like so:

  ```bash
  kubectl scale pgs simple1 --replicas=1
  ```

## Supported Schedulers

Currently the following schedulers support gang scheduling of `PodGang`s created by the Grove operator:

- [NVIDIA/KAI-scheduler](https://github.com/NVIDIA/KAI-Scheduler)
