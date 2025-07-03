# Getting Started

To get started with Grove, you would need a running Kubernetes cluster. As there is no released version of Grove operator just yet, we recommend you try out Grove locally using a [kind](https://kind.sigs.k8s.io/) cluster, using the make targets we provide as a part of the repository.

> [!NOTE]
> In case you would like to run Grove operator in your own Kubernetes cluster, you could build an image of Grove operator and init container by running the provided make targets in [operator/Makefile](../operator/Makefile), and specifying your image in the operator's Helm charts in [values.yaml](../operator/charts/values.yaml); and make a Helm release of Grove in your cluster.

## Local kind cluster set-up

All code and corresponding make targets for the operator are present in the [operator](./operator) directory, so you would first switch your working directory to that.

### Kind cluster set-up

- Set up the kind cluster configured to run the Grove operator by running:

  ```bash
  make kind-up
  ```

- Specify the `KUBECONFIG` environment variable in your shell session to the path printed out at the end of the previous step:

  ```bash
  # You would see something like `export KUBECONFIG=/path-to-your-grove-clone/grove/operator/hack/kind/kubeconfig` printed.
  # If you are already in `/path-to-your-grove-clone/grove/operator`, then you can simply:
  export KUBECONFIG=./hack/kind/kubeconfig
  ```

### Deploy Grove operator

```bash
make deploy-local
```

This make target installs all relevant CRDs, builds `grove-operator`, `grove-initc`, and deploys the operator to the kind cluster.
You can configure the Grove operator by modifying the [values.yaml](../operator/charts/values.yaml).

### Deploy a `PodGangSet`

- Deploy one of the samples present in the [samples](./operator/samples/simple) directory.

  ```bash
  kubectl apply -f ./samples/simple/simple1.yaml
  ```

- You can now fetch resources like `PodGangSet`, `PodGang`, `PodClique`, `PodCliqueScalingGroup`, etc. created by the Grove operator, by running:

  ```bash
  kubectl get pgs,pclq,pcsg,pg,pod -owide
  ```

  You would see output like this:

  ```bash
  ❯ kubectl get pgs,pclq,pcsg,pg,pod -owide
  NAME                          AGE
  podgangset.grove.io/simple1   27s

  NAME                               AGE
  podclique.grove.io/simple1-0-pca   26s
  podclique.grove.io/simple1-0-pcb   26s
  podclique.grove.io/simple1-0-pcc   26s
  podclique.grove.io/simple1-0-pcd   26s

  NAME                                            AGE
  podcliquescalinggroup.grove.io/simple1-0-pcsg   26s

  NAME                                   AGE
  podgang.scheduler.grove.io/simple1-0   26s

  NAME                                  READY   STATUS    RESTARTS   AGE
  pod/grove-operator-74cc5866c8-s7ssj   1/1     Running   0          34s
  pod/simple1-0-pca-hk7pk               1/1     Running   0          26s
  pod/simple1-0-pca-m8hm6               1/1     Running   0          26s
  pod/simple1-0-pca-nrfs9               1/1     Running   0          26s
  pod/simple1-0-pcb-pg5dr               1/1     Running   0          26s
  pod/simple1-0-pcb-r46xn               1/1     Running   0          26s
  pod/simple1-0-pcc-hw9lf               1/1     Running   0          26s
  pod/simple1-0-pcc-pb6hg               1/1     Running   0          26s
  pod/simple1-0-pcd-7nvm5               1/1     Running   0          26s
  pod/simple1-0-pcd-dtfr2               1/1     Running   0          26s
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
  podgangset.grove.io/simple1   87s

  NAME                               AGE
  podclique.grove.io/simple1-0-pca   86s
  podclique.grove.io/simple1-0-pcb   86s
  podclique.grove.io/simple1-0-pcc   86s
  podclique.grove.io/simple1-0-pcd   86s

  NAME                                            AGE
  podcliquescalinggroup.grove.io/simple1-0-pcsg   86s

  NAME                                          AGE
  podgang.scheduler.grove.io/simple1-0          86s
  podgang.scheduler.grove.io/simple1-0-pcsg-0   11s # newly created `PodGang`, as a consequence of scaling `PodCliqueScalingGroup`

  NAME                                  READY   STATUS    RESTARTS   AGE
  pod/grove-operator-74cc5866c8-s7ssj   1/1     Running   0          94s
  pod/simple1-0-pca-hk7pk               1/1     Running   0          86s
  pod/simple1-0-pca-m8hm6               1/1     Running   0          86s
  pod/simple1-0-pca-nrfs9               1/1     Running   0          86s
  pod/simple1-0-pcb-f7gt9               1/1     Running   0          11s # newly created Pod, as a consequence of scaling `PodCliqueScalingGroup`
  pod/simple1-0-pcb-pg5dr               1/1     Running   0          86s
  pod/simple1-0-pcb-qw855               1/1     Running   0          11s # newly created Pod, as a consequence of scaling `PodCliqueScalingGroup`
  pod/simple1-0-pcb-r46xn               1/1     Running   0          86s
  pod/simple1-0-pcc-59mh8               1/1     Running   0          11s # newly created Pod, as a consequence of scaling `PodCliqueScalingGroup`
  pod/simple1-0-pcc-hw9lf               1/1     Running   0          86s
  pod/simple1-0-pcc-pb6hg               1/1     Running   0          86s
  pod/simple1-0-pcc-sgkxz               1/1     Running   0          11s # newly created Pod, as a consequence of scaling `PodCliqueScalingGroup`
  pod/simple1-0-pcd-7nvm5               1/1     Running   0          86s
  pod/simple1-0-pcd-dtfr2               1/1     Running   0          86s
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
  podgangset.grove.io/simple1   14m

  NAME                               AGE
  podclique.grove.io/simple1-0-pca   14m
  podclique.grove.io/simple1-0-pcb   14m
  podclique.grove.io/simple1-0-pcc   14m
  podclique.grove.io/simple1-0-pcd   14m
  podclique.grove.io/simple1-1-pca   79s # newly created `PodClique`, as a consequence of scaling `PodGangSet`
  podclique.grove.io/simple1-1-pcb   79s # newly created `PodClique`, as a consequence of scaling `PodGangSet`
  podclique.grove.io/simple1-1-pcc   79s # newly created `PodClique`, as a consequence of scaling `PodGangSet`
  podclique.grove.io/simple1-1-pcd   79s # newly created `PodClique`, as a consequence of scaling `PodGangSet`

  NAME                                            AGE
  podcliquescalinggroup.grove.io/simple1-0-pcsg   14m
  podcliquescalinggroup.grove.io/simple1-1-pcsg   79s # newly created `PodCliqueScalingGroup`, as a consequence of scaling `PodGangSet`

  NAME                                   AGE
  podgang.scheduler.grove.io/simple1-0   14m
  podgang.scheduler.grove.io/simple1-1   79s # newly created `PodGang`, as a consequence of scaling `PodGangSet`

  NAME                                  READY   STATUS    RESTARTS   AGE
  pod/grove-operator-74cc5866c8-s7ssj   1/1     Running   0          14m
  pod/simple1-0-pca-hk7pk               1/1     Running   0          14m
  pod/simple1-0-pca-m8hm6               1/1     Running   0          14m
  pod/simple1-0-pca-nrfs9               1/1     Running   0          14m
  pod/simple1-0-pcb-pg5dr               1/1     Running   0          14m
  pod/simple1-0-pcb-r46xn               1/1     Running   0          14m
  pod/simple1-0-pcc-hw9lf               1/1     Running   0          14m
  pod/simple1-0-pcc-pb6hg               1/1     Running   0          14m
  pod/simple1-0-pcd-7nvm5               1/1     Running   0          14m
  pod/simple1-0-pcd-dtfr2               1/1     Running   0          14m
  pod/simple1-1-pca-nhv6h               1/1     Running   0          79s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pca-q7gxl               1/1     Running   0          79s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pca-xscll               1/1     Running   0          79s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pcb-4wcvh               1/1     Running   0          79s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pcb-8dbr7               1/1     Running   0          79s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pcc-ff9pr               1/1     Running   0          79s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pcc-rlx5t               1/1     Running   0          79s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pcd-7tbgc               1/1     Running   0          79s # newly created Pod, as a consequence of scaling `PodGangSet`
  pod/simple1-1-pcd-vgv2z               1/1     Running   0          79s # newly created Pod, as a consequence of scaling `PodGangSet`
  ```

  Similarly, the `PodGangSet` can be scaled back in to 1 replicas like so:

  ```bash
  kubectl scale pgs simple1 --replicas=1
  ```

## Supported Schedulers

Currently the following schedulers support gang scheduling of `PodGang`s created by the Grove operator:

- [NVIDIA/KAI-scheduler](https://github.com/NVIDIA/KAI-Scheduler)
