# Rolling Update

## Overview

`Grove` offers a hierarchical and a flexible API to define AI inference workloads. There are primarily three groupings, namely `PodGangSet`, `PodCliqueScalingGroup` and `PodClique` as depicted below.

<img src="/Users/i062009/go/src/github.com/unmarshall/grove/docs/assets/pgs-composition.excalidraw.png" style="zoom:60%;" />

<< TODO >>

## Abbreviations

We will be using the following `short-names` in the document for brevity. 

| Abbreviation / Short Name | Long Form / Description                                      |
| ------------------------- | ------------------------------------------------------------ |
| PGS                       | PodGangSet                                                   |
| PCLQ                      | PodClique                                                    |
| PCSG                      | PodCliqueScalingGroup                                        |
| PCLQ-S                    | Standalone PodClique, is a PodClique that is not associated to any PodCliqueScalingGroup |
| PCLQ-G                    | PodClique that is associated to a PodCliqueScalingGroup      |

## Requirements

Rolling update is an evolving feature. In the initial version of rolling updates following are the requirements:

* `Scale` subresource has been exposed for PGS, PCLQ-S and PCSG. Scale-in and Scale-out at all levels should be supported during rolling update.
* Availability is paramount for deployed AI workloads. It is therefore advised that multiple replica deployments for each of PGS, PCLQ and PCSG have more than one replica. Keeping availability in view folling requirements are defined:
  * Only one PGS replica should be updated at a time. If update of the currently updating replica is not complete then the rolling update will get paused till the time the criteria for update completion is met.
  * For PCSG and PCLQ:
    * During the rolling update if there are any replicas in `Pending` or in `Unhealthy` state then they should be force updated and rolling update should not wait for them to be ready.
    * Only one ready replica should be updated at a time. The update of each replica is done by recreating the entire replica. Rolling update must progress to the next replica only when the criter for update completion for a replica is met.
* Partial rolling updates for the PGS should be supported. Since each `PodClique` can have a different `PodSpec`, it is possible that updates are only available for a subset of `PodClique`s in a `PodGangSet`. Since each PodClique can utilize large number of GPUs (e.g. wide-EP deployment), relinquishing GPU resources (which are scarce resource) for PodCliques that have no updates is not desirable as it can lead to unexpected and longer unavailability.
* It can take a long time (from seconds to several minutes) to update each constituent `Pod`.  Thus it is important that the update progress be indicated via appropriate custom resource status fields.

## How to identify if there are pending updates?

> << TODO >>
> Here we talk about Generation hash that we have introduced in PGS and how is it computed and used to identify pending change.

## Tracking Rolling update progress

> << TODO >>
>
> Here we mention the API changes `RollingUpdateProgress` introduced in all resources and how are they populated and semantically explain each field. When is this initialized and when marked as completed.
>
> In this section we also explain when is PCSG or PCLQ or PGS replica update considered complete.
>
> In this section we also explain when is the update considered stuck and how to identify it.

## Order in which resources are selected for update

> << TODO >>
>
> In this section we explain at each level - PGS, PCSG and PCLQ in which order are the replicas chosen for update.

## Flow

> << TODO >>
>
> In this section we describe:
>
> * Actors participating in the rolling update
> * Flow of control amongst them

