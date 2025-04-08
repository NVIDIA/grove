# Design and Implementation Notes

## Scheduler PodGang type

* If `PodClique.Spec.ScaleConfig` is defined then `PodGang.Spec.MemberCliques[i].MinReplicas` should be set to `PodClique.Spec.ScaleConfig.MinReplicas` else 
`PodGang.Spec.MemberCliques[i].MinReplicas` should be set to respective `PodClique.Spec.Replicas`.
* Gang scheduling and termination is guaranteed for `PodGang.Spec.MemberCliques[i].MinReplicas` only.
* If length of `PodGang.Spec.MemberCliques[i].PodReferences` > `PodGang.Spec.MemberCliques[i].MinReplicas`, then scheduler makes best effort to gang schedule all the pods in the `PodGang.Spec.MemberCliques[i].PodReferences`. However if 
resources are only available for `PodGang.Spec.MemberCliques[i].MinReplicas` then only those pods will be scheduled and the rest will be in pending state.

