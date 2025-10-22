# Installation

To get started with Grove, you can use the published Helm charts of Grove under the [GitHub packages section](https://github.com/orgs/ai-dynamo/packages?repo_name=grove), and install Grove in your cluster.
You can also try out Grove locally on your machine using a [kind](https://kind.sigs.k8s.io/) cluster, using the make targets we provide as a part of the repository.

## Deploying Grove

You can use the published [Helm `grove-charts` package](https://github.com/ai-dynamo/grove/pkgs/container/grove%2Fgrove-charts), and install it in your cluster. Set the `KUBECONFIG` in your shell session, and run the following:

```bash
helm upgrade -i grove oci://ghcr.io/ai-dynamo/grove/grove-charts:<tag>
```

You could also deploy Grove to your cluster through the provided make targets, by following [installation using make targets](#installation-using-make-targets).

## Developing Grove

All grove operator Make targets are located in [Operator Makefile](../operator/Makefile).

### Cluster set-up

#### Local Kind cluster set-up

In case you wish to develop Grove using a local kind cluster, please do the following:

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

#### Remote cluster set-up

If you wish to use your own Kubernetes cluster instead of the KIND cluster, follow these steps:

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

### Installation using make targets

```bash
# If you wish to deploy all Grove Operator resources in a custom namespace then set the `NAMESPACE` environment variable
export NAMESPACE=custom-ns
# if `NAMESPACE` environment variable is set then `make deploy` target will use this namespace to deploy all Grove operator resources
make deploy
```

This make target installs all relevant CRDs, builds `grove-operator`, `grove-initc`, and deploys the operator to the cluster.
You can configure the Grove operator by modifying the [values.yaml](../operator/charts/values.yaml).

This make target leverages Grove [Helm](https://helm.sh/) charts and [Skaffold](https://skaffold.dev/) to install the following resources to the cluster:

- [CRDs](../operator/charts):
  - Grove operator CRD - `podcliquesets.grove.io`, `podcliques.grove.io` and `podcliquescalinggroups.grove.io`.
  - Grove Scheduler CRDs - `podgangs.scheduler.grove.io`.
- All Grove operator resources defined as a part of [Grove Helm chart templates](../operator/charts/templates).

## Deploy a `PodCliqueSet`

- Deploy one of the samples present in the [samples](../operator/samples/simple) directory.

  ```bash
  kubectl apply -f ./samples/simple/simple1.yaml
  ```

- You can now fetch resources like `PodCliqueSet` (pcs), `PodGang` (pg), `PodClique` (pclq), `PodCliqueScalingGroup` (pcsg), etc. created by the Grove operator, by running:

  ```bash
  kubectl get pcs,pclq,pcsg,pg,pod -owide
  ```

  You would see output like this:

  ```bash
  ❯ kubectl get pcs,pclq,pcsg,pg,pod -owide
  NAME                          AGE
  podcliqueset.grove.io/simple1   34s

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

## Scaling

As specified in the [README.md](../README.md) and the [docs](../docs), there are multiple hierarchies at which you can scale resources in a `PodCliqueSet`.

- Let's try scaling the `PodCliqueScalingGroup` from 1 to 2 replicas:

  ```bash
  kubectl scale pcsg simple1-0-pcsg --replicas=2
  ```

  This will create new pods that associate with cliques that belong to this scaling group, and their associated `PodGang`s.

  Fetching all resources to check the newly created resources:

  ```bash
  ❯ kubectl get pcs,pclq,pcsg,pg,pod -owide
  NAME                          AGE
  podcliqueset.grove.io/simple1   2m28s

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

- Scaling can also be triggered at the `PodCliqueSet` level, as can be seen here:

  ```bash
  kubectl scale pcs simple1 --replicas=2
  ```

  ```bash
  ❯ kubectl get pcs,pclq,pcsg,pg,pod -owide
  NAME                          AGE
  podcliqueset.grove.io/simple1   6m25s

  NAME                                     AGE
  podclique.grove.io/simple1-0-pca         6m24s
  podclique.grove.io/simple1-0-pcd         6m24s
  podclique.grove.io/simple1-0-sga-0-pcb   6m24s
  podclique.grove.io/simple1-0-sga-0-pcc   6m24s
  podclique.grove.io/simple1-1-pca         51s # newly created `PodClique`, as a consequence of scaling `PodCliqueSet`
  podclique.grove.io/simple1-1-pcd         51s # newly created `PodClique`, as a consequence of scaling `PodCliqueSet`
  podclique.grove.io/simple1-1-sga-0-pcb   51s # newly created `PodClique`, as a consequence of scaling `PodCliqueSet`
  podclique.grove.io/simple1-1-sga-0-pcc   51s # newly created `PodClique`, as a consequence of scaling `PodCliqueSet`

  NAME                                           AGE
  podcliquescalinggroup.grove.io/simple1-0-sga   6m24s
  podcliquescalinggroup.grove.io/simple1-1-sga   51s # newly created `PodCliqueScalingGroup`, as a consequence of scaling `PodCliqueSet`

  NAME                                   AGE
  podgang.scheduler.grove.io/simple1-0   6m24s
  podgang.scheduler.grove.io/simple1-1   51s # newly created `PodGang`, as a consequence of scaling `PodCliqueSet`

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
  pod/simple1-1-pca-29njw               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodCliqueSet`
  pod/simple1-1-pca-9cgm8               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodCliqueSet`
  pod/simple1-1-pca-lhrw8               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodCliqueSet`
  pod/simple1-1-pcd-6fjzm               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodCliqueSet`
  pod/simple1-1-pcd-n288d               1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodCliqueSet`
  pod/simple1-1-sga-0-pcb-5kvzx         1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodCliqueSet`
  pod/simple1-1-sga-0-pcb-h8g8l         1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodCliqueSet`
  pod/simple1-1-sga-0-pcc-6gxfb         1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodCliqueSet`
  pod/simple1-1-sga-0-pcc-rqfqf         1/1     Running   0          51s # newly created Pod, as a consequence of scaling `PodCliqueSet`
  ```

  Similarly, the `PodCliqueSet` can be scaled back in to 1 replicas like so:

  ```bash
  kubectl scale pcs simple1 --replicas=1
  ```

## Supported Schedulers

Currently the following schedulers support gang scheduling of `PodGang`s created by the Grove operator:

- [NVIDIA/KAI-scheduler](https://github.com/NVIDIA/KAI-Scheduler)
