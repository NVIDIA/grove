# developing and testing the grove kube-scheduler plugin

Testing the grove kube-scheduler plugin is currently supported locally using a kind cluster. There are make targets present in the [scheduler/Makefile](../scheduler/Makefile) that support this.

Likewise, the scheduler can be tested in any kubernetes cluster by deploying the kube-scheduler built with the grove plugin as a pod.

## building the scheduler

To build the kube-scheduler image, use the `docker-build-<arch>` target. The image built from here can be deployed on any kubernetes cluster for testing.
This step is *NOT NECESSARY* for testing the scheduler locally in the kind cluster, as it is taken care of by skaffold.

## testing the scheduler locally

You can test the scheduler locally in two ways:
- Replace the vanilla kube-scheduler that a kind cluster starts with with the grove-kube-scheduler.
- Deploy the grove-kube-scheduler as a second scheduler running alongside the vanilla kube-scheduler.

### replacing the vanilla kube-scheduler

- Bring up a kind cluster through the specified target `kind-up` in [scheduler/Makefile](../scheduler/Makefile). (wait for the node to be ready)
- Modify the `KubeSchedulerConfigruation` according to your needs in `generate_kube_scheduler_configuration` function in [replace-scheduler.sh](../scheduler/hack/replace-scheduler.sh) if necessary. This function generates the configuration that you want to pass to the kube-scheduler with the grove plugin; and is later copied to the kind node.
- Modify the static pod manifest in `generate_kube_scheduler_manifest` function in [replace-scheduler.sh](../scheduler/hack/replace-scheduler.sh) if necessary. This function generates the static pod manfiest of the kube-scheduler pod that will be replacing the vanilla kube-scheduler in the kind cluster.
- Run `make deploy-local-replace`.

Following the above should replace the vanilla kube-scheduler with grove-kube-scheduler.
To iterate on changes, make changes in the [scheduler](../scheduler), and rerun the make target.

### running as a second kube-scheduler

- Bring up a kind cluster through the specified target `kind-up` in [scheduler/Makefile](../scheduler/Makefile). (wait for the node to be ready)
- Modify the [values.yaml](../scheduler/charts/values.yaml) and the corresponding [templates](../scheduler/charts/templates).
- Run `make deploy-local-second`.

Following the above should deploy grove-kube-scheduler as a second scheduler in the cluster. Specify the `schedulerName` to make use of the second scheduler.
