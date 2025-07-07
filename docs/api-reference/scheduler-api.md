# API Reference

## Packages
- [scheduler.grove.io/v1alpha1](#schedulergroveiov1alpha1)


## scheduler.grove.io/v1alpha1


### Resource Types
- [PodGang](#podgang)



#### NamespacedName



NamespacedName is a struct that contains the namespace and name of an object.
types.NamespacedName does not have json tags, so we define our own for the time being.
If https://github.com/kubernetes/kubernetes/issues/131313 is resolved, we can switch to using the APIMachinery type instead.



_Appears in:_
- [PodGangSpec](#podgangspec)
- [PodGroup](#podgroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `namespace` _string_ | Namespace is the namespace of the object. |  |  |
| `name` _string_ | Name is the name of the object. |  |  |


#### NetworkPackGroupConfig



NetworkPackGroupConfig indicates that all the Pods belonging to the constituent PodGroup's should be optimally placed w.r.t cluster's network topology.



_Appears in:_
- [PodGangSpec](#podgangspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podGroupNames` _string array_ | PodGroupNames is the list of PodGroup.Name that are part of the network pack group. |  |  |




#### PodGang



PodGang defines a specification of a group of pods that should be scheduled together.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `scheduler.grove.io/v1alpha1` | | |
| `kind` _string_ | `PodGang` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PodGangSpec](#podgangspec)_ | Spec defines the specification of the PodGang. |  |  |
| `status` _[PodGangStatus](#podgangstatus)_ | Status defines the status of the PodGang. |  |  |


#### PodGangSpec



PodGangSpec defines the specification of a PodGang.



_Appears in:_
- [PodGang](#podgang)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podgroups` _[PodGroup](#podgroup) array_ | PodGroups is a list of member pod groups in the PodGang. |  |  |
| `networkPackGroupConfigs` _[NetworkPackGroupConfig](#networkpackgroupconfig) array_ | NetworkPackGroupConfigs is a list of network pack group configurations. |  |  |
| `spreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#topologyspreadconstraint-v1-core) array_ | SpreadConstraints defines the constraints for spreading PodGang's filtered by the same label selector, across domains identified by a topology key. |  |  |
| `priorityClassName` _string_ | PriorityClassName is the name of the PriorityClass to be used for the PodGangSet.<br />If specified, indicates the priority of the PodGangSet. "system-node-critical" and<br />"system-cluster-critical" are two special keywords which indicate the<br />highest priorities with the former being the highest priority. Any other<br />name must be defined by creating a PriorityClass object with that name.<br />If not specified, the pod priority will be default or zero if there is no default. |  |  |
| `terminationDelay` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | TerminationDelay is the delay after which the gang termination will be triggered.<br />A gang is a candidate for termination if number of running pods fall below a threshold for any PodClique.<br />If a PodGang remains a candidate past TerminationDelay then it will be terminated. This allows additional time<br />to the kube-scheduler to re-schedule sufficient pods in the PodGang that will result in having the total number of<br />running pods go above the threshold. |  |  |
| `reuseReservationRef` _[NamespacedName](#namespacedname)_ | ReuseReservationRef holds the reference to another PodGang resource scheduled previously.<br />During updates, an operator can suggest to reuse the reservation of the previous PodGang for a newer version of the<br />PodGang resource. This is a suggestion for the scheduler and not a requirement that must be met. If the scheduler plugin<br />finds that the reservation done previously was network optimised and there are no better alternatives available, then it<br />will reuse the reservation. If there are better alternatives available, then the scheduler will ignore this suggestion. |  |  |


#### PodGangStatus



PodGangStatus defines the status of a PodGang.



_Appears in:_
- [PodGang](#podgang)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `schedulingPhase` _string_ | SchedulingPhase is the current phase of scheduling for the PodGang. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | Conditions is a list of conditions that describe the current state of the PodGang. |  |  |
| `placementScore` _float_ | PlacementScore is network optimality score for the PodGang. If the choice that the scheduler has made corresponds to the<br />best possible placement of the pods in the PodGang, then the score will be 1.0. Higher the score, better the placement. |  |  |


#### PodGroup



PodGroup defines a set of pods in a PodGang that share the same PodTemplateSpec.



_Appears in:_
- [PodGangSpec](#podgangspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the PodGroup. |  |  |
| `podReferences` _[NamespacedName](#namespacedname) array_ | PodReferences is a list of references to the Pods that are part of this group. |  |  |
| `minReplicas` _integer_ | MinReplicas is the number of replicas that needs to be gang scheduled.<br />If the MinReplicas is greater than len(PodReferences) then scheduler makes the best effort to schedule as many pods beyond<br />MinReplicas. However, guaranteed gang scheduling is only provided for MinReplicas. |  |  |


