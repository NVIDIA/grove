# Design and Implementation Notes

## Scheduler PodGang type

### Gang semantics in the presence of MinReplicas

* If `PodClique.Spec.ScaleConfig` is defined then `PodGang.Spec.MemberCliques[i].MinReplicas` should be set to `PodClique.Spec.ScaleConfig.MinReplicas`. Else, 
`PodGang.Spec.MemberCliques[i].MinReplicas` should be set to respective `PodClique.Spec.Replicas`.

* PodGang gang-scheduling semantics:
  * Guaranteed for pods `PodGang.Spec.MemberCliques[i].PodReferences[0:(MinReplicas-1)]`.
  * Best-effort for pods `PodGang.Spec.MemberCliques[i].PodReferences[MinReplicas:]`.

* PodGang gang-termination and gang-restart semantics:
  * When the number of running pods in `PodGang.Spec.MemberCliques[i].PodReferences[]` < `PodGang.Spec.MemberCliques[i].MinReplicas`,
then gang-termination will be triggered after `PodGang.Spec.TerminationDelay`. If pending pods get scheduled within `PodGang.Spec.TerminationDelay`
and minReplica constraints are satisfied, then gang-termination will not be triggered.
  * Gang-termination semantics will terminate all pods in the PodGang.
  * After gang-termination, PodGang pods will be re-created.

