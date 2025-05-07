# developing and testing the grove kube-scheduler plugin

Testing the grove kube-scheduler plugin is currently supported locally using a kind cluster. There are make targets present in the [scheduler-plugins/Makefile](../scheduler-plugins/Makefile) that support this.

Likewise, the scheduler can be tested in any kubernetes cluster by deploying the kube-scheduler built with the grove plugin as a pod.

## building the scheduler

To build the kube-scheduler image, use the `docker-build-<arch>` target. The image built from here can be deployed on any kubernetes cluster for testing.
This step is *NOT NECESSARY* for testing the scheduler locally in the kind cluster, as it is taken care of by skaffold.

## testing the scheduler locally

- Bring up a kind cluster through the specified target `kind-up` in [scheduler-plugins/Makefile](../scheduler-plugins/Makefile). (wait for the node to be ready)
- Modify the `KubeSchedulerConfigruation` according to your needs in `generate_kube_scheduler_configuration` function in [replace-scheduler.sh](../scheduler-plugins/hack/replace-scheduler.sh) if necessary. This function generates the configuration that you want to pass to the kube-scheduler with the grove plugin; and is later copied to the kind node.
- Modify the static pod manifest in `generate_kube_scheduler_manifest` function in [replace-scheduler.sh](../scheduler-plugins/hack/replace-scheduler.sh) if necessary. This function generates the static pod manfiest of the kube-scheduler pod that will be replacing the vanilla kube-scheduler in the kind cluster.
- Run `make deploy-local-replace`.

Once all the steps are followed, the vanilla kube-scheduler in the kind cluster is replaced with a kube-scheduler with the grove plugin enabled.

To iterate on changes, make changes in the [scheduler-plugins](../scheduler-plugins), and rerun the make target.
