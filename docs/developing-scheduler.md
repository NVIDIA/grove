# Developing and testing the Grove kube-scheduler plugin

Testing the grove kube-scheduler plugin is currently supported locally using a Kind cluster. There are make targets present in the [scheduler/Makefile](../scheduler/Makefile) that support this.

Likewise, the scheduler can be tested in any kubernetes cluster by deploying the kube-scheduler built with the grove plugin as a pod.

## Building the Grove kube-scheduler

To build the kube-scheduler image, use the `docker-build-<arch>` target. The image built from here can be deployed on any kubernetes cluster for testing.
This step is *NOT NECESSARY* for testing the scheduler locally in the Kind cluster, as it is taken care of by skaffold.

## Testing the Grove kube-scheduler locally

You can test the scheduler locally in two ways:
- Replace the vanilla/default kube-scheduler bundled with a Kind cluster with the Grove kube-scheduler.
- Deploy the Grove kube-scheduler as a second scheduler running alongside the vanilla/default kube-scheduler.

### Replacing the vanilla/default kube-scheduler

- Bring up a Kind cluster through the specified target `kind-up` in [scheduler/Makefile](../scheduler/Makefile). Wait for the node to be ready.
- Configure the `KubeSchedulerConfigruation` as necessary in [scheduler-configuration.yaml](../scheduler/hack/kind/scheduler-configuration.yaml).
- Configure the static pod manifest as necessary in [kube-scheduler.yaml](../scheduler/hack/kind/kube-scheduler.yaml).
- Run `make replace-default-scheduler`.

Following the above should replace the vanilla kube-scheduler with grove-kube-scheduler.
To iterate on changes, make changes in the [scheduler](../scheduler), and rerun the make target.

### Running as a second kube-scheduler

- Bring up a Kind cluster through the specified target `kind-up` in [scheduler/Makefile](../scheduler/Makefile). Wait for the node to be ready.
- Configure the kube-scheduler as necessary by modifying the [values.yaml](../scheduler/charts/values.yaml) and the corresponding [templates](../scheduler/charts/templates).
- Run `make deploy-second-scheduler`.

Following the above should deploy grove-kube-scheduler as a second scheduler in the cluster. Specify the `schedulerName` in the `PodSpec.SchedulerName` to ensure that the grove scheduler is used to schedule the target pod(s).