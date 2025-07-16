# API Reference

## Packages
- [grove.io/v1alpha1](#groveiov1alpha1)
- [operator.config.grove.io/v1alpha1](#operatorconfiggroveiov1alpha1)


## grove.io/v1alpha1


### Resource Types
- [PodClique](#podclique)
- [PodCliqueScalingGroup](#podcliquescalinggroup)
- [PodGangSet](#podgangset)



#### AutoScalingConfig



AutoScalingConfig defines the configuration for the horizontal pod autoscaler.



_Appears in:_
- [PodCliqueScalingGroupConfig](#podcliquescalinggroupconfig)
- [PodCliqueSpec](#podcliquespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minReplicas` _integer_ | MinReplicas is the lower limit for the number of replicas for the target resource.<br />It will be used by the horizontal pod autoscaler to determine the minimum number of replicas to scale-in to. |  |  |
| `maxReplicas` _integer_ | maxReplicas is the upper limit for the number of replicas to which the autoscaler can scale up.<br />It cannot be less that minReplicas. |  |  |
| `metrics` _[MetricSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#metricspec-v2-autoscaling) array_ | Metrics contains the specifications for which to use to calculate the<br />desired replica count (the maximum replica count across all metrics will<br />be used).  The desired replica count is calculated multiplying the<br />ratio between the target value and the current value by the current<br />number of pods.  Ergo, metrics used must decrease as the pod count is<br />increased, and vice versa.  See the individual metric source types for<br />more information about how each type of metric must respond.<br />If not set, the default metric will be set to 80% average CPU utilization. |  |  |


#### CliqueStartupType

_Underlying type:_ _string_

CliqueStartupType defines the order in which each PodClique is started.

_Validation:_
- Enum: [CliqueStartupTypeAnyOrder CliqueStartupTypeInOrder CliqueStartupTypeExplicit]

_Appears in:_
- [PodGangSetTemplateSpec](#podgangsettemplatespec)

| Field | Description |
| --- | --- |
| `CliqueStartupTypeAnyOrder` | CliqueStartupTypeAnyOrder defines that the cliques can be started in any order. This allows for concurrent starts of cliques.<br />This is the default CliqueStartupType.<br /> |
| `CliqueStartupTypeInOrder` | CliqueStartupTypeInOrder defines that the cliques should be started in the order they are defined in the PodGang Cliques slice.<br /> |
| `CliqueStartupTypeExplicit` | CliqueStartupTypeExplicit defines that the cliques should be started after the cliques defined in PodClique.StartsAfter have started.<br /> |


#### ErrorCode

_Underlying type:_ _string_

ErrorCode is a custom error code that uniquely identifies an error.



_Appears in:_
- [LastError](#lasterror)



#### HeadlessServiceConfig



HeadlessServiceConfig defines the config options for the headless service.



_Appears in:_
- [PodGangSetTemplateSpec](#podgangsettemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `publishNotReadyAddresses` _boolean_ | PublishNotReadyAddresses if set to true will publish the DNS records of pods even if the pods are not ready. |  |  |


#### LastError



LastError captures the last error observed by the controller when reconciling an object.



_Appears in:_
- [PodCliqueScalingGroupStatus](#podcliquescalinggroupstatus)
- [PodCliqueStatus](#podcliquestatus)
- [PodGangSetStatus](#podgangsetstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `code` _[ErrorCode](#errorcode)_ | Code is the error code that uniquely identifies the error. |  |  |
| `description` _string_ | Description is a human-readable description of the error. |  |  |
| `observedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | ObservedAt is the time at which the error was observed. |  |  |


#### LastOperation



LastOperation captures the last operation done by the respective reconciler on the PodGangSet.



_Appears in:_
- [PodCliqueScalingGroupStatus](#podcliquescalinggroupstatus)
- [PodCliqueStatus](#podcliquestatus)
- [PodGangSetStatus](#podgangsetstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[LastOperationType](#lastoperationtype)_ | Type is the type of the last operation. |  |  |
| `state` _[LastOperationState](#lastoperationstate)_ | State is the state of the last operation. |  |  |
| `description` _string_ | Description is a human-readable description of the last operation. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta)_ | LastUpdateTime is the time at which the last operation was updated. |  |  |


#### LastOperationState

_Underlying type:_ _string_

LastOperationState is a string alias for the state of the last operation.



_Appears in:_
- [LastOperation](#lastoperation)

| Field | Description |
| --- | --- |
| `Processing` | LastOperationStateProcessing indicates that the last operation is in progress.<br /> |
| `Succeeded` | LastOperationStateSucceeded indicates that the last operation succeeded.<br /> |
| `Error` | LastOperationStateError indicates that the last operation completed with errors and will be retried.<br /> |


#### LastOperationType

_Underlying type:_ _string_

LastOperationType is a string alias for the type of the last operation.



_Appears in:_
- [LastOperation](#lastoperation)

| Field | Description |
| --- | --- |
| `Reconcile` | LastOperationTypeReconcile indicates that the last operation was a reconcile operation.<br /> |
| `Delete` | LastOperationTypeDelete indicates that the last operation was a delete operation.<br /> |


#### NetworkPackGroupConfig



NetworkPackGroupConfig indicates that all the Pods belonging to the constituent PodCliques should be optimally placed w.r.t cluster's network topology.
If a constituent PodClique belongs to a PodCliqueScalingGroup then ensure that all constituent PodCliques of that PodCliqueScalingGroup are also part of the NetworkPackGroupConfig.



_Appears in:_
- [SchedulingPolicyConfig](#schedulingpolicyconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cliqueNames` _string array_ | CliqueNames is the list of PodClique names that are part of the network pack group. |  |  |


#### PodClique



PodClique is a set of pods running the same image. TODO: @renormalize expand on this.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `grove.io/v1alpha1` | | |
| `kind` _string_ | `PodClique` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PodCliqueSpec](#podcliquespec)_ | Spec defines the specification of a PodClique. |  |  |
| `status` _[PodCliqueStatus](#podcliquestatus)_ | Status defines the status of a PodClique. |  |  |


#### PodCliqueScalingGroup



PodCliqueScalingGroup is the schema to define scaling groups that is used to scale a group of PodClique's.
An instance of this custom resource will be created for every pod clique scaling group defined as part of PodGangSet.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `grove.io/v1alpha1` | | |
| `kind` _string_ | `PodCliqueScalingGroup` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PodCliqueScalingGroupSpec](#podcliquescalinggroupspec)_ | Spec is the specification of the PodCliqueScalingGroup. |  |  |
| `status` _[PodCliqueScalingGroupStatus](#podcliquescalinggroupstatus)_ | Status is the status of the PodCliqueScalingGroup. |  |  |


#### PodCliqueScalingGroupConfig



PodCliqueScalingGroupConfig is a group of PodClique's that are scaled together.
Each member PodClique.Replicas will be computed as a product of PodCliqueScalingGroupConfig.Replicas and PodCliqueTemplateSpec.Spec.Replicas.
NOTE: If a PodCliqueScalingGroupConfig is defined, then for the member PodClique's, individual AutoScalingConfig cannot be defined.



_Appears in:_
- [PodGangSetTemplateSpec](#podgangsettemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the PodCliqueScalingGroupConfig. This should be unique within the PodGangSet.<br />It allows consumers to give a semantic name to a group of PodCliques that needs to be scaled together. |  |  |
| `cliqueNames` _string array_ | CliqueNames is the list of names of the PodClique's that are part of the scaling group. |  |  |
| `scaleConfig` _[AutoScalingConfig](#autoscalingconfig)_ | ScaleConfig is the horizontal pod autoscaler configuration for the pod clique scaling group. |  |  |


#### PodCliqueScalingGroupSpec



PodCliqueScalingGroupSpec is the specification of the PodCliqueScalingGroup.



_Appears in:_
- [PodCliqueScalingGroup](#podcliquescalinggroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the desired number of replicas for the PodCliqueScalingGroup.<br />If not specified, it defaults to 1. |  |  |
| `cliqueNames` _string array_ | CliqueNames is the list of PodClique names that are configured in the<br />matching PodCliqueScalingGroup in PodGangSet.Spec.Template.PodCliqueScalingGroupConfigs. |  |  |


#### PodCliqueScalingGroupStatus



PodCliqueScalingGroupStatus is the status of the PodCliqueScalingGroup.



_Appears in:_
- [PodCliqueScalingGroup](#podcliquescalinggroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the observed number of replicas for the PodCliqueScalingGroup. |  |  |
| `selector` _string_ | Selector is the selector used to identify the pods that belong to this scaling group. |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed by the controller. |  |  |
| `lastOperation` _[LastOperation](#lastoperation)_ | LastOperation captures the last operation done by the respective reconciler on the PodClique. |  |  |
| `lastErrors` _[LastError](#lasterror) array_ | LastErrors captures the last errors observed by the controller when reconciling the PodClique. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | Conditions represents the latest available observations of the PodCliqueScalingGroup by its controller. |  |  |


#### PodCliqueSpec



PodCliqueSpec defines the specification of a PodClique.



_Appears in:_
- [PodClique](#podclique)
- [PodCliqueTemplateSpec](#podcliquetemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `roleName` _string_ | RoleName is the name of the role that this PodClique will assume. |  |  |
| `podSpec` _[PodSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#podspec-v1-core)_ | Spec is the spec of the pods in the clique. |  |  |
| `replicas` _integer_ | Replicas is the number of replicas of the pods in the clique. It cannot be less than 1. |  |  |
| `minAvailable` _integer_ | MinAvailable serves two purposes:<br />1. It defines the minimum number of pods that are guaranteed to be gang scheduled.<br />2. It defines the minimum requirement of available pods in a PodClique. Violation of this threshold will result in termination of the PodGang that it belongs to.<br />If MinAvailable is not set, then it will default to the template Replicas. |  |  |
| `startsAfter` _string array_ | StartsAfter provides you a way to explicitly define the startup dependencies amongst cliques.<br />If CliqueStartupType in PodGang has been set to 'CliqueStartupTypeExplicit', then to create an ordered start amongst PodClique's StartsAfter can be used.<br />A forest of DAG's can be defined to model any start order dependencies. If there are more than one PodClique's defined and StartsAfter is not set for any of them,<br />then their startup order is random at best and must not be relied upon.<br />Validations:<br />1. If a StartsAfter has been defined and one or more cycles are detected in DAG's then it will be flagged as validation error.<br />2. If StartsAfter is defined and does not identify any PodClique then it will be flagged as a validation error. |  |  |
| `autoScalingConfig` _[AutoScalingConfig](#autoscalingconfig)_ | ScaleConfig is the horizontal pod autoscaler configuration for a PodClique. |  |  |


#### PodCliqueStatus



PodCliqueStatus defines the status of a PodClique.



_Appears in:_
- [PodClique](#podclique)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed by the controller. |  |  |
| `lastOperation` _[LastOperation](#lastoperation)_ | LastOperation captures the last operation done by the respective reconciler on the PodClique. |  |  |
| `lastErrors` _[LastError](#lasterror) array_ | LastErrors captures the last errors observed by the controller when reconciling the PodClique. |  |  |
| `replicas` _integer_ | Replicas is the total number of non-terminated Pods targeted by this PodClique. |  |  |
| `readyReplicas` _integer_ | ReadyReplicas is the number of ready Pods targeted by this PodClique. |  |  |
| `updatedReplicas` _integer_ | UpdatedReplicas is the number of Pods that have been updated and are at the desired revision of the PodClique. |  |  |
| `scheduleGatedReplicas` _integer_ | ScheduleGatedReplicas is the number of Pods that have been created with one or more scheduling gate(s) set.<br />Sum of ReadyReplicas and ScheduleGatedReplicas will always be <= Replicas. |  |  |
| `hpaPodSelector` _string_ | Selector is the label selector that determines which pods are part of the PodClique.<br />PodClique is a unit of scale and this selector is used by HPA to scale the PodClique based on metrics captured for the pods that match this selector. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | Conditions represents the latest available observations of the clique by its controller. |  |  |


#### PodCliqueTemplateSpec



PodCliqueTemplateSpec defines a template spec for a PodClique.



_Appears in:_
- [PodGangSetTemplateSpec](#podgangsettemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name must be unique within a PodGangSet and is used to denote a role.<br />Once set it cannot be updated.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#names |  |  |
| `labels` _object (keys:string, values:string)_ | Labels is a map of string keys and values that can be used to organize and categorize<br />(scope and select) objects. May match selectors of replication controllers<br />and services.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations is an unstructured key value map stored with a resource that may be<br />set by external tools to store and retrieve arbitrary metadata. They are not<br />queryable and should be preserved when modifying objects.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations |  |  |
| `spec` _[PodCliqueSpec](#podcliquespec)_ | Specification of the desired behavior of a PodClique.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  |  |


#### PodGangPhase

_Underlying type:_ _string_

PodGangPhase represents the phase of a PodGang.

_Validation:_
- Enum: [Pending Starting Running Failed Succeeded]

_Appears in:_
- [PodGangStatus](#podgangstatus)

| Field | Description |
| --- | --- |
| `Pending` | PodGangPending indicates that the pods in a PodGang have not yet been taken up for scheduling.<br /> |
| `Starting` | PodGangStarting indicates that the pods are bound to nodes by the scheduler and are starting.<br /> |
| `Running` | PodGangRunning indicates that the all the pods in a PodGang are running.<br /> |
| `Failed` | PodGangFailed indicates that one or more pods in a PodGang have failed.<br />This is a terminal state and is typically used for batch jobs.<br /> |
| `Succeeded` | PodGangSucceeded indicates that all the pods in a PodGang have succeeded.<br />This is a terminal state and is typically used for batch jobs.<br /> |


#### PodGangSet



PodGangSet is a set of PodGangs defining specification on how to spread and manage a gang of pods and monitoring their status.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `grove.io/v1alpha1` | | |
| `kind` _string_ | `PodGangSet` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PodGangSetSpec](#podgangsetspec)_ | Spec defines the specification of the PodGangSet. |  |  |
| `status` _[PodGangSetStatus](#podgangsetstatus)_ | Status defines the status of the PodGangSet. |  |  |


#### PodGangSetSpec



PodGangSetSpec defines the specification of a PodGangSet.



_Appears in:_
- [PodGangSet](#podgangset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the number of desired replicas of the PodGang. | 0 |  |
| `template` _[PodGangSetTemplateSpec](#podgangsettemplatespec)_ | Template describes the template spec for PodGangs that will be created in the PodGangSet. |  |  |
| `replicaSpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#topologyspreadconstraint-v1-core) array_ | ReplicaSpreadConstraints defines the constraints for spreading each replica of PodGangSet across domains identified by a topology key. |  |  |


#### PodGangSetStatus



PodGangSetStatus defines the status of a PodGangSet.



_Appears in:_
- [PodGangSet](#podgangset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed by the controller. |  |  |
| `lastOperation` _[LastOperation](#lastoperation)_ | LastOperation captures the last operation done by the respective reconciler on the PodGangSet. |  |  |
| `lastErrors` _[LastError](#lasterror) array_ | LastErrors captures the last errors observed by the controller when reconciling the PodGangSet. |  |  |
| `replicas` _integer_ | Replicas is the total number of PodGangSet replicas created. |  |  |
| `updatedReplicas` _integer_ | UpdatedReplicas is the number of replicas that have been updated to the desired revision of the PodGangSet. |  |  |
| `hpaPodSelector` _string_ | Selector is the label selector that determines which pods are part of the PodGang.<br />PodGang is a unit of scale and this selector is used by HPA to scale the PodGang based on metrics captured for the pods that match this selector. |  |  |
| `podGangStatuses` _[PodGangStatus](#podgangstatus) array_ | PodGangStatuses captures the status for all the PodGang's that are part of the PodGangSet. |  |  |


#### PodGangSetTemplateSpec



PodGangSetTemplateSpec defines a template spec for a PodGang.
A PodGang does not have a RestartPolicy field because the restart policy is predefined:
If the number of pods in any of the cliques falls below the threshold, the entire PodGang will be restarted.
The threshold is determined by either:
- The value of "MinReplicas", if specified in the ScaleConfig of that clique, or
- The "Replicas" value of that clique



_Appears in:_
- [PodGangSetSpec](#podgangsetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cliques` _[PodCliqueTemplateSpec](#podcliquetemplatespec) array_ | Cliques is a slice of cliques that make up the PodGang. There should be at least one PodClique. |  |  |
| `cliqueStartupType` _[CliqueStartupType](#cliquestartuptype)_ | StartupType defines the type of startup dependency amongst the cliques within a PodGang.<br />If it is not defined then default of CliqueStartupTypeAnyOrder is used. | CliqueStartupTypeAnyOrder | Enum: [CliqueStartupTypeAnyOrder CliqueStartupTypeInOrder CliqueStartupTypeExplicit] <br /> |
| `priorityClassName` _string_ | PriorityClassName is the name of the PriorityClass to be used for the PodGangSet.<br />If specified, indicates the priority of the PodGangSet. "system-node-critical" and<br />"system-cluster-critical" are two special keywords which indicate the<br />highest priorities with the former being the highest priority. Any other<br />name must be defined by creating a PriorityClass object with that name.<br />If not specified, the pod priority will be default or zero if there is no default. |  |  |
| `headlessServiceConfig` _[HeadlessServiceConfig](#headlessserviceconfig)_ | HeadlessServiceConfig defines the config options for the headless service.<br />If present, create headless service for each PodGang. |  |  |
| `schedulingPolicyConfig` _[SchedulingPolicyConfig](#schedulingpolicyconfig)_ | SchedulingPolicyConfig defines the scheduling policy configuration for the PodGang.<br />Defaulting only works for optional fields.<br />See https://github.com/kubernetes-sigs/controller-tools/issues/893#issuecomment-1991256368 |  |  |
| `terminationDelay` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | TerminationDelay is the delay after which the gang termination will be triggered.<br />A gang is a candidate for termination if number of running pods fall below a threshold for any PodClique.<br />If a PodGang remains a candidate past TerminationDelay then it will be terminated. This allows additional time<br />to the kube-scheduler to re-schedule sufficient pods in the PodGang that will result in having the total number of<br />running pods go above the threshold. |  |  |
| `podCliqueScalingGroups` _[PodCliqueScalingGroupConfig](#podcliquescalinggroupconfig) array_ | PodCliqueScalingGroupConfigs is a list of scaling groups for the PodGangSet. |  |  |


#### PodGangStatus



PodGangStatus defines the status of a PodGang.



_Appears in:_
- [PodGangSetStatus](#podgangsetstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the PodGang. |  |  |
| `phase` _[PodGangPhase](#podgangphase)_ | Phase is the current phase of the PodGang. |  | Enum: [Pending Starting Running Failed Succeeded] <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | Conditions represents the latest available observations of the PodGang by its controller. |  |  |




#### SchedulingPolicyConfig



SchedulingPolicyConfig defines the scheduling policy configuration for the PodGang.



_Appears in:_
- [PodGangSetTemplateSpec](#podgangsettemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `networkPackGroupConfigs` _[NetworkPackGroupConfig](#networkpackgroupconfig) array_ | NetworkPackGroupConfigs is a list of NetworkPackGroupConfig's that define how the pods in the PodGangSet are optimally packaged w.r.t cluster's network topology.<br />PodCliques that are not part of any NetworkPackGroupConfig are scheduled with best-effort network packing strategy.<br />Exercise caution when defining NetworkPackGroupConfig. Some of the downsides include:<br />1. Scheduling may be delayed until optimal placement is available.<br />2. Pods created due to scale-out or rolling upgrades is not guaranteed optimal placement. |  |  |



## operator.config.grove.io/v1alpha1




#### AuthorizerConfig



AuthorizerConfig defines the configuration for the authorizer admission webhook.



_Appears in:_
- [OperatorConfiguration](#operatorconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled indicates whether the authorizer is enabled. |  |  |
| `exemptServiceAccounts` _string array_ | ExemptServiceAccounts is a list of service accounts that are exempt from authorizer checks. |  |  |


#### ClientConnectionConfiguration



ClientConnectionConfiguration defines the configuration for constructing a client.



_Appears in:_
- [OperatorConfiguration](#operatorconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `qps` _float_ | QPS controls the number of queries per second allowed for this connection. |  |  |
| `burst` _integer_ | Burst allows extra queries to accumulate when a client is exceeding its rate. |  |  |
| `contentType` _string_ | ContentType is the content type used when sending data to the server from this client. |  |  |
| `acceptContentTypes` _string_ | AcceptContentTypes defines the Accept header sent by clients when connecting to the server,<br />overriding the default value of 'application/json'. This field will control all connections<br />to the server used by a particular client. |  |  |


#### ControllerConfiguration



ControllerConfiguration defines the configuration for the controllers.



_Appears in:_
- [OperatorConfiguration](#operatorconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podGangSet` _[PodGangSetControllerConfiguration](#podgangsetcontrollerconfiguration)_ | PodGangSet is the configuration for the PodGangSet controller. |  |  |
| `podClique` _[PodCliqueControllerConfiguration](#podcliquecontrollerconfiguration)_ | PodClique is the configuration for the PodClique controller. |  |  |
| `podCliqueScalingGroup` _[PodCliqueScalingGroupControllerConfiguration](#podcliquescalinggroupcontrollerconfiguration)_ | PodCliqueScalingGroup is the configuration for the PodCliqueScalingGroup controller. |  |  |


#### DebuggingConfiguration



DebuggingConfiguration defines the configuration for debugging.



_Appears in:_
- [OperatorConfiguration](#operatorconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enableProfiling` _boolean_ | EnableProfiling enables profiling via host:port/debug/pprof/ endpoints. |  |  |


#### LeaderElectionConfiguration



LeaderElectionConfiguration defines the configuration for the leader election.



_Appears in:_
- [OperatorConfiguration](#operatorconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled specifies whether leader election is enabled. Set this<br />to true when running replicated instances of the operator for high availability. |  |  |
| `leaseDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | LeaseDuration is the duration that non-leader candidates will wait<br />after observing a leadership renewal until attempting to acquire<br />leadership of the occupied but un-renewed leader slot. This is effectively the<br />maximum duration that a leader can be stopped before it is replaced<br />by another candidate. This is only applicable if leader election is<br />enabled. |  |  |
| `renewDeadline` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | RenewDeadline is the interval between attempts by the acting leader to<br />renew its leadership before it stops leading. This must be less than or<br />equal to the lease duration.<br />This is only applicable if leader election is enabled. |  |  |
| `retryPeriod` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | RetryPeriod is the duration leader elector clients should wait<br />between attempting acquisition and renewal of leadership.<br />This is only applicable if leader election is enabled. |  |  |
| `resourceLock` _string_ | ResourceLock determines which resource lock to use for leader election.<br />This is only applicable if leader election is enabled. |  |  |
| `resourceName` _string_ | ResourceName determines the name of the resource that leader election<br />will use for holding the leader lock.<br />This is only applicable if leader election is enabled. |  |  |
| `resourceNamespace` _string_ | ResourceNamespace determines the namespace in which the leader<br />election resource will be created.<br />This is only applicable if leader election is enabled. |  |  |


#### LogFormat

_Underlying type:_ _string_

LogFormat defines the format of the log.



_Appears in:_
- [OperatorConfiguration](#operatorconfiguration)

| Field | Description |
| --- | --- |
| `json` | LogFormatJSON is the JSON log format.<br /> |
| `text` | LogFormatText is the text log format.<br /> |


#### LogLevel

_Underlying type:_ _string_

LogLevel defines the log level.



_Appears in:_
- [OperatorConfiguration](#operatorconfiguration)

| Field | Description |
| --- | --- |
| `debug` | DebugLevel is the debug log level, i.e. the most verbose.<br /> |
| `info` | InfoLevel is the default log level.<br /> |
| `error` | ErrorLevel is a log level where only errors are logged.<br /> |




#### PodCliqueControllerConfiguration



PodCliqueControllerConfiguration defines the configuration for the PodClique controller.



_Appears in:_
- [ControllerConfiguration](#controllerconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `concurrentSyncs` _integer_ | ConcurrentSyncs is the number of workers used for the controller to concurrently work on events. |  |  |


#### PodCliqueScalingGroupControllerConfiguration



PodCliqueScalingGroupControllerConfiguration defines the configuration for the PodCliqueScalingGroup controller.



_Appears in:_
- [ControllerConfiguration](#controllerconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `concurrentSyncs` _integer_ | ConcurrentSyncs is the number of workers used for the controller to concurrently work on events. |  |  |


#### PodGangSetControllerConfiguration



PodGangSetControllerConfiguration defines the configuration for the PodGangSet controller.



_Appears in:_
- [ControllerConfiguration](#controllerconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `concurrentSyncs` _integer_ | ConcurrentSyncs is the number of workers used for the controller to concurrently work on events. |  |  |


#### Server



Server contains information for HTTP(S) server configuration.



_Appears in:_
- [ServerConfiguration](#serverconfiguration)
- [WebhookServer](#webhookserver)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bindAddress` _string_ | BindAddress is the IP address on which to listen for the specified port. |  |  |
| `port` _integer_ | Port is the port on which to serve requests. |  |  |


#### ServerConfiguration



ServerConfiguration defines the configuration for the HTTP(S) servers.



_Appears in:_
- [OperatorConfiguration](#operatorconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `webhooks` _[WebhookServer](#webhookserver)_ | Webhooks is the configuration for the HTTP(S) webhook server. |  |  |
| `healthProbes` _[Server](#server)_ | HealthProbes is the configuration for serving the healthz and readyz endpoints. |  |  |
| `metrics` _[Server](#server)_ | Metrics is the configuration for serving the metrics endpoint. |  |  |


#### WebhookServer



WebhookServer defines the configuration for the HTTP(S) webhook server.



_Appears in:_
- [ServerConfiguration](#serverconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bindAddress` _string_ | BindAddress is the IP address on which to listen for the specified port. |  |  |
| `port` _integer_ | Port is the port on which to serve requests. |  |  |
| `serverCertDir` _string_ | ServerCertDir is the directory containing the server certificate and key. |  |  |


