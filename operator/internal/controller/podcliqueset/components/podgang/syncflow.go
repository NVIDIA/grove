// /*
// Copyright 2025 The Grove Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package podgang

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/constants"
	"github.com/NVIDIA/grove/operator/internal/controller/common/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/controller/common/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// prepareSyncFlow computes the required state required by the sync flow for the PodGang resources.
func (r _resource) prepareSyncFlow(ctx context.Context, logger logr.Logger, pcs *grovecorev1alpha1.PodCliqueSet) (*syncContext, error) {
	pcsObjectKey := client.ObjectKeyFromObject(pcs)
	sc := &syncContext{
		ctx:                  ctx,
		pcs:                  pcs,
		logger:               logger,
		expectedPodGangs:     make([]podGangInfo, 0),
		existingPodGangNames: make([]string, 0),
		pclqs:                make([]grovecorev1alpha1.PodClique, 0),
		unassignedPodsByPCLQ: make(map[string][]corev1.Pod),
		pclqPods:             make(map[string][]corev1.Pod),
	}

	pclqs, err := r.getPCLQsForPCS(ctx, pcsObjectKey)
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodCliques,
			component.OperationSync,
			fmt.Sprintf("failed to list PodCliques for PodCliqueSet %v", pcsObjectKey),
		)
	}
	sc.pclqs = pclqs

	if err := r.computeExpectedPodGangs(sc); err != nil {
		return nil, groveerr.WrapError(err,
			errCodeComputeExistingPodGangs,
			component.OperationSync,
			fmt.Sprintf("failed to compute existing PodGangs for PodCliqueSet %v", pcsObjectKey),
		)
	}

	existingPodGangNames, err := r.GetExistingResourceNames(ctx, logger, pcs.ObjectMeta)
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodGangs,
			component.OperationSync,
			fmt.Sprintf("Failed to get existing PodGang names for PodCliqueSet: %v", client.ObjectKeyFromObject(sc.pcs)),
		)
	}
	sc.existingPodGangNames = existingPodGangNames

	podsByPCLQ, err := r.getPodsByPCLQForPCS(ctx, pcsObjectKey)
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPods,
			component.OperationSync,
			fmt.Sprintf("failed to list Pods for PodCliqueSet %v", pcsObjectKey),
		)
	}

	sc.pclqPods = podsByPCLQ
	sc.initializeAssignedAndUnassignedPodsForPCS(podsByPCLQ)

	return sc, nil
}

func (r _resource) getPCLQsForPCS(ctx context.Context, pcsObjectKey client.ObjectKey) ([]grovecorev1alpha1.PodClique, error) {
	pclqList := &grovecorev1alpha1.PodCliqueList{}
	if err := r.client.List(ctx, pclqList,
		client.InNamespace(pcsObjectKey.Namespace),
		client.MatchingLabels(apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsObjectKey.Name))); err != nil {
		return nil, err
	}
	return pclqList.Items, nil
}

// computeExpectedPodGangs computes the expected PodGangs for the PodCliqueSet.
// It inspects the defined PodCliqueScalingGroup's and PodCliqueSet replicas to identify PodGangs.
func (r _resource) computeExpectedPodGangs(sc *syncContext) error {
	expectedPodGangs := make([]podGangInfo, 0, 50) // preallocate to avoid multiple allocations

	// For each PodCliqueSet replica, a base-podgang is expected to be created.
	// It contains the references to the initial set of pods that is to be gang-scheduled.
	expectedPodGangs = append(expectedPodGangs, getExpectedPodGangForPCSReplicas(sc)...)

	// For each replica of PodCliqueSet, get the PodGangs associated to PodCliqueScalingGroup replicas above MinAvailable.
	if len(sc.pcs.Spec.Template.PodCliqueScalingGroupConfigs) > 0 {
		for pcsReplica := range sc.pcs.Spec.Replicas {
			expectedPodGangsForPCSG, err := r.getExpectedPodGangsForPCSG(sc.ctx, sc.pcs, int(pcsReplica))
			if err != nil {
				return err
			}
			expectedPodGangs = append(expectedPodGangs, expectedPodGangsForPCSG...)
		}
	}
	sc.expectedPodGangs = expectedPodGangs
	return nil
}

// getExpectedPodGangForPCSReplicas creates the BASE PodGangs for each PodCliqueSet replica.
//
// These are the foundational PodGangs that contain:
// 1. Standalone PodCliques (not part of any scaling group)
// 2. Base scaling group PodCliques (replicas 0 through minAvailable-1 of each scaling group)
//
// Scaled PodGangs (for scaling group replicas >= minAvailable) are handled
// separately by getExpectedPodGangsForPCSG() and managed by the PodCliqueScalingGroup controller.
func getExpectedPodGangForPCSReplicas(sc *syncContext) []podGangInfo {
	expectedPodGangs := make([]podGangInfo, 0, int(sc.pcs.Spec.Replicas))
	for pcsReplica := range sc.pcs.Spec.Replicas {
		podGangName := apicommon.GenerateBasePodGangName(apicommon.ResourceNameReplica{Name: sc.pcs.Name, Replica: int(pcsReplica)})
		expectedPodGangs = append(expectedPodGangs, podGangInfo{
			fqn:   podGangName,
			pclqs: identifyConstituentPCLQsForPCSBasePodGang(sc, pcsReplica),
		})
	}
	return expectedPodGangs
}

func (r _resource) getExpectedPodGangsForPCSG(ctx context.Context, pcs *grovecorev1alpha1.PodCliqueSet, pcsReplica int) ([]podGangInfo, error) {
	existingPCSGs, err := r.getExistingPodCliqueScalingGroups(ctx, pcs, pcsReplica)
	if err != nil {
		return nil, err
	}

	expectedPodGangs := make([]podGangInfo, 0, 50) // preallocate to avoid multiple allocations

	// For each template PCSG config, compute the expected podGangInfo's
	for _, pcsgConfig := range pcs.Spec.Template.PodCliqueScalingGroupConfigs {
		// Generate PCSG resource name from template name
		pcsgFQN := apicommon.GeneratePodCliqueScalingGroupName(apicommon.ResourceNameReplica{Name: pcs.Name, Replica: pcsReplica}, pcsgConfig.Name)

		// MinAvailable should always be non-nil due to kubebuilder default and defaulting webhook
		minAvailable := int(*pcsgConfig.MinAvailable)

		// Compute the replicas value from PCSG resource if it exists. If not, then use template replicas.
		replicas := int(*pcsgConfig.Replicas)
		pcsg, ok := lo.Find(existingPCSGs, func(sg grovecorev1alpha1.PodCliqueScalingGroup) bool {
			return sg.Name == pcsgFQN
		})
		if ok {
			replicas = int(pcsg.Spec.Replicas)
		}

		// Create scaled PodGangs for replicas starting from minAvailable
		// The first 0..(minAvailable-1) replicas are handled by the PCS replica PodGang
		// Scaled PodGangs use 0-based indexing regardless of minAvailable value
		scaledReplicas := replicas - minAvailable
		for podGangIndex, pcsgReplica := 0, minAvailable; podGangIndex < scaledReplicas; podGangIndex, pcsgReplica = podGangIndex+1, pcsgReplica+1 {
			podGangName := apicommon.CreatePodGangNameFromPCSGFQN(pcsgFQN, podGangIndex)
			pclqs, err := identifyConstituentPCLQsForPCSGPodGang(pcs, pcsgFQN, pcsgReplica, pcsgConfig.CliqueNames)
			if err != nil {
				return nil, err
			}
			expectedPodGangs = append(expectedPodGangs, podGangInfo{
				fqn:   podGangName,
				pclqs: pclqs,
			})
		}
	}
	return expectedPodGangs, nil
}

func identifyConstituentPCLQsForPCSBasePodGang(sc *syncContext, pcsReplica int32) []pclqInfo {
	constituentPCLQs := make([]pclqInfo, 0, len(sc.pcs.Spec.Template.Cliques))
	for _, pclqTemplateSpec := range sc.pcs.Spec.Template.Cliques {
		// Check if this PodClique belongs to a scaling group
		pcsgConfig := componentutils.FindScalingGroupConfigForClique(sc.pcs.Spec.Template.PodCliqueScalingGroupConfigs, pclqTemplateSpec.Name)
		if pcsgConfig != nil {
			// Add scaling group PodClique instances for replicas 0 through (minAvailable-1)
			scalingGroupPclqs := buildPCSGPodCliqueInfosForBasePodGang(sc, pclqTemplateSpec, pcsgConfig, pcsReplica)
			constituentPCLQs = append(constituentPCLQs, scalingGroupPclqs...)
		} else {
			// Add standalone PodClique (not part of a scaling group)
			standalonePclq := buildNonPCSGPodCliqueInfosForBasePodGang(sc, pclqTemplateSpec, pcsReplica)
			constituentPCLQs = append(constituentPCLQs, standalonePclq)
		}
	}
	return constituentPCLQs
}

// buildPCSGPodCliqueInfosForBasePodGang generates PodClique info for the BASE PODGANG portion of a scaling group.
//
// IMPORTANT: This function only generates PodClique info for replicas 0 through (minAvailable-1).
// These PodCliques will be grouped into the BASE PodGang, which represents the minimum viable
// cluster that must be scheduled together as a gang.
//
// Scaled PodGangs (for replicas >= minAvailable) are generated separately by the
// PodCliqueScalingGroup controller and get their own scaled PodGang resources.
//
// EXAMPLE with minAvailable=3:
//   - This function creates PodCliques for replicas 0, 1, 2 → go into base PodGang "simple1-0"
//   - PCSG controller creates PodCliques for replicas 3, 4 → get scaled PodGangs "simple1-0-sga-0", etc.
func buildPCSGPodCliqueInfosForBasePodGang(sc *syncContext, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, pcsgConfig *grovecorev1alpha1.PodCliqueScalingGroupConfig, pcsReplica int32) []pclqInfo {
	// MinAvailable should always be non-nil due to kubebuilder default and defaulting webhook
	minAvailable := int(*pcsgConfig.MinAvailable)

	pclqs := make([]pclqInfo, 0, minAvailable)
	for replicaIndex := range minAvailable {
		pcsgFQN := apicommon.GeneratePodCliqueScalingGroupName(
			apicommon.ResourceNameReplica{Name: sc.pcs.Name, Replica: int(pcsReplica)},
			pcsgConfig.Name,
		)
		pclqFQN := apicommon.GeneratePodCliqueName(
			apicommon.ResourceNameReplica{Name: pcsgFQN, Replica: replicaIndex},
			pclqTemplateSpec.Name,
		)

		pclqInfo := buildPodCliqueInfo(sc, pclqTemplateSpec, pclqFQN)
		pclqs = append(pclqs, pclqInfo)
	}
	return pclqs
}

// buildNonPCSGPodCliqueInfosForBasePodGang generates pclqInfo for a PodClique that doesn't belong to a scaling group
func buildNonPCSGPodCliqueInfosForBasePodGang(sc *syncContext, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, pcsReplica int32) pclqInfo {
	pclqFQN := apicommon.GeneratePodCliqueName(
		apicommon.ResourceNameReplica{Name: sc.pcs.Name, Replica: int(pcsReplica)},
		pclqTemplateSpec.Name,
	)
	return buildPodCliqueInfo(sc, pclqTemplateSpec, pclqFQN)
}

// buildPodCliqueInfo creates a pclqInfo object with the correct replicas and minAvailable values
func buildPodCliqueInfo(sc *syncContext, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, pclqFQN string) pclqInfo {
	replicas := determinePodCliqueReplicas(sc, pclqTemplateSpec, pclqFQN)
	return pclqInfo{
		fqn:          pclqFQN,
		replicas:     replicas,
		minAvailable: *pclqTemplateSpec.Spec.MinAvailable,
	}
}

// determinePodCliqueReplicas determines the correct replicas count for a PodClique
func determinePodCliqueReplicas(sc *syncContext, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, pclqFQN string) int32 {
	if pclqTemplateSpec.Spec.ScaleConfig == nil {
		return pclqTemplateSpec.Spec.Replicas
	}
	matchingPCLQ, found := lo.Find(sc.pclqs, func(pclq grovecorev1alpha1.PodClique) bool {
		return pclqFQN == pclq.Name
	})
	if !found {
		// PodClique resource not found - might be during initial creation
		// Fall back to template replicas but log warning for visibility
		sc.logger.Info("[WARN]: PodClique resource not found, using template replicas",
			"podCliqueFQN", pclqFQN,
			"templateReplicas", pclqTemplateSpec.Spec.Replicas)
		return pclqTemplateSpec.Spec.Replicas
	}
	return matchingPCLQ.Spec.Replicas
}

func identifyConstituentPCLQsForPCSGPodGang(pcs *grovecorev1alpha1.PodCliqueSet, pcsgFQN string, pcsgReplica int, cliqueNames []string) ([]pclqInfo, error) {
	constituentPCLQs := make([]pclqInfo, 0, len(cliqueNames))
	for _, pclqName := range cliqueNames {
		pclqTemplate, ok := lo.Find(pcs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return pclqName == pclqTemplateSpec.Name
		})
		if !ok {
			return nil, fmt.Errorf("PodCliqueScalingGroup references a PodClique that does not exist in the PodCliqueSet: %s", pclqName)
		}

		pclqFQN := apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{Name: pcsgFQN, Replica: pcsgReplica}, pclqName)

		// Create pclqInfo using consistent logic (note: we use template replicas here since this is for scaling group instances)
		constituentPCLQs = append(constituentPCLQs, pclqInfo{
			fqn:          pclqFQN,
			replicas:     pclqTemplate.Spec.Replicas, // For scaling group instances, always use template replicas
			minAvailable: *pclqTemplate.Spec.MinAvailable,
		})
	}
	return constituentPCLQs, nil
}

func (r _resource) getExistingPodCliqueScalingGroups(ctx context.Context, pcs *grovecorev1alpha1.PodCliqueSet, pcsReplica int) ([]grovecorev1alpha1.PodCliqueScalingGroup, error) {
	pcsgList := &grovecorev1alpha1.PodCliqueScalingGroupList{}
	if err := r.client.List(ctx,
		pcsgList,
		client.InNamespace(pcs.Namespace),
		client.MatchingLabels(
			lo.Assign(
				apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcs.Name),
				map[string]string{
					apicommon.LabelPodCliqueSetReplicaIndex: strconv.Itoa(pcsReplica),
				},
			),
		),
	); err != nil {
		return nil, err
	}
	return lo.Filter(pcsgList.Items, func(pcsg grovecorev1alpha1.PodCliqueScalingGroup, _ int) bool {
		return metav1.IsControlledBy(&pcsg, pcs)
	}), nil
}

func (r _resource) getPodsByPCLQForPCS(ctx context.Context, pcsObjectKey client.ObjectKey) (map[string][]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := r.client.List(ctx,
		podList,
		client.InNamespace(pcsObjectKey.Namespace),
		client.MatchingLabels(apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsObjectKey.Name)),
	); err != nil {
		return nil, err
	}

	podsByPCLQ := make(map[string][]corev1.Pod)
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		pclqFQN := k8sutils.GetFirstOwnerName(pod.ObjectMeta)
		podsByPCLQ[pclqFQN] = append(podsByPCLQ[pclqFQN], pod)
	}

	return podsByPCLQ, nil
}

func (r _resource) runSyncFlow(sc *syncContext) syncFlowResult {
	result := syncFlowResult{}
	if err := r.deleteExcessPodGangs(sc); err != nil {
		result.errs = append(result.errs, err)
		return result
	}
	return r.createOrUpdatePodGangs(sc)
}

func (r _resource) deleteExcessPodGangs(sc *syncContext) error {
	expectedPodGangNames := lo.Map(sc.expectedPodGangs, func(pg podGangInfo, _ int) string {
		return pg.fqn
	})
	excessPodGangs, _ := lo.Difference(sc.existingPodGangNames, expectedPodGangNames)
	namespace := sc.pcs.Namespace
	for _, podGangToDelete := range excessPodGangs {
		pgObjectKey := client.ObjectKey{Namespace: namespace, Name: podGangToDelete}
		pg := emptyPodGang(pgObjectKey)
		sc.logger.Info("Delete excess PodGang", "objectKey", client.ObjectKeyFromObject(pg))
		if err := client.IgnoreNotFound(r.client.Delete(sc.ctx, pg)); err != nil {
			r.eventRecorder.Eventf(sc.pcs, corev1.EventTypeWarning, constants.ReasonPodGangDeleteFailed, "Error deleting PodGang %v: %v", pgObjectKey, err)
			return groveerr.WrapError(err,
				errCodeDeleteExcessPodGang,
				component.OperationSync,
				fmt.Sprintf("failed to delete PodGang %v", pgObjectKey),
			)
		}
		r.eventRecorder.Eventf(sc.pcs, corev1.EventTypeNormal, constants.ReasonPodGangDeleteSuccessful, "Deleted PodGang %v", pgObjectKey)
		sc.deletedPodGangNames = append(sc.deletedPodGangNames, podGangToDelete)
		sc.logger.Info("Triggered delete of excess PodGang", "objectKey", client.ObjectKeyFromObject(pg))
	}
	return nil
}

func (r _resource) createOrUpdatePodGangs(sc *syncContext) syncFlowResult {
	result := syncFlowResult{}
	pendingPodGangNames := sc.getPodGangNamesPendingCreation()
	for _, podGang := range sc.expectedPodGangs {
		sc.logger.Info("[createOrUpdatePodGangs] processing PodGang", "fqn", podGang.fqn)
		isPodGangPendingCreation := slices.Contains(pendingPodGangNames, podGang.fqn)
		// check the health of each podclique
		numPendingPods := r.getPodsPendingCreationOrAssociation(sc, podGang)
		if isPodGangPendingCreation && numPendingPods > 0 {
			sc.logger.Info("skipping creation of PodGang as all desired replicas have not yet been created or assigned", "fqn", podGang.fqn, "numPendingPodsToCreateOrAssociate", numPendingPods)
			result.recordPodGangPendingCreation(podGang.fqn)
			continue
		}
		if err := r.createOrUpdatePodGang(sc, podGang); err != nil {
			sc.logger.Error(err, "failed to create PodGang", "PodGangName", podGang.fqn)
			result.recordError(err)
			return result
		}
		result.recordPodGangCreation(podGang.fqn)
	}
	return result
}

// getPodsForPodCliquesPendingCreation returns the number of expected pods from PodCliques that are pending creation
func (r _resource) getPodsForPodCliquesPendingCreation(sc *syncContext, podGang podGangInfo) int {
	existingPCLQNames := lo.Map(sc.pclqs, func(pclq grovecorev1alpha1.PodClique, _ int) string {
		return pclq.Name
	})

	return lo.Reduce(podGang.pclqs, func(agg int, pclq pclqInfo, _ int) int {
		if !slices.Contains(existingPCLQNames, pclq.fqn) {
			return agg + int(pclq.replicas)
		}
		return agg
	}, 0)
}

func (r _resource) getPodsPendingCreationOrAssociation(sc *syncContext, podGang podGangInfo) int {
	// Find the number of expected pods from PodCliques that are pending creation
	numPodsPendingPCLQCreate := r.getPodsForPodCliquesPendingCreation(sc, podGang)

	// Find the number of pods pending creation of existing PodCliques
	var numPodsPendingCreateOrAssociate int
	pclqs := sc.getPodCliques(podGang)
	for _, pclq := range pclqs {
		existingPCLQPods := sc.pclqPods[pclq.Name]
		// If there is a difference between the expected replicas and the existing pods, we need to account for that.
		// If the difference is positive, it means there are pending pods to create.
		// If the difference is negative, it means there are more existing pods than expected. In this case, we do not need to create any new pods, therefore we can ignore the negative difference.
		numPodsPendingCreateOrAssociate += max(0, int(pclq.Spec.Replicas)-len(existingPCLQPods))

		// For all existing pods in the PCLQ, check if they have the PodGang label set. If that is not set then add them to numPodsPendingCreateOrAssociate.
		for _, existingPod := range existingPCLQPods {
			podGangLabelValue, ok := existingPod.GetLabels()[apicommon.LabelPodGang]
			if !ok {
				sc.logger.Info("Pod does not have a PodGang label yet", "podObjectKey", client.ObjectKeyFromObject(&existingPod), "expectedPodGangName", podGang.fqn)
				numPodsPendingCreateOrAssociate += 1
				continue
			}
			if podGangLabelValue != podGang.fqn {
				sc.logger.Error(nil, "PodGang label does not match expected PodGang name. This should ideally never happen and indicates a coding error", "podObjectKey", client.ObjectKeyFromObject(&existingPod), "expectedPodGangName", podGang.fqn, "podGangLabelValue", podGangLabelValue)
				numPodsPendingCreateOrAssociate += 1
			}
		}
	}
	return numPodsPendingPCLQCreate + numPodsPendingCreateOrAssociate
}

func (r _resource) createOrUpdatePodGang(sc *syncContext, pgInfo podGangInfo) error {
	pgObjectKey := client.ObjectKey{
		Namespace: sc.pcs.Namespace,
		Name:      pgInfo.fqn,
	}
	pg := emptyPodGang(pgObjectKey)
	sc.logger.Info("CreateOrPatch PodGang", "objectKey", pgObjectKey)
	_, err := controllerutil.CreateOrPatch(sc.ctx, r.client, pg, func() error {
		return r.buildResource(sc.pcs, pgInfo, pg)
	})
	if err != nil {
		r.eventRecorder.Eventf(sc.pcs, corev1.EventTypeWarning, constants.ReasonPodGangCreateOrUpdateFailed, "Error Creating/Updating PodGang %v: %v", pgObjectKey, err)
		return groveerr.WrapError(err,
			errCodeCreateOrPatchPodGang,
			component.OperationSync,
			fmt.Sprintf("Failed to CreateOrPatch PodGang %v", pgObjectKey),
		)
	}
	r.eventRecorder.Eventf(sc.pcs, corev1.EventTypeNormal, constants.ReasonPodGangCreateOrUpdateSuccessful, "Created/Updated PodGang %v", pgObjectKey)
	sc.logger.Info("Triggered CreateOrPatch of PodGang", "objectKey", pgObjectKey)
	return nil
}

func createPodGroupsForPodGang(namespace string, pgInfo podGangInfo) []groveschedulerv1alpha1.PodGroup {
	podGroups := lo.Map(pgInfo.pclqs, func(pclq pclqInfo, _ int) groveschedulerv1alpha1.PodGroup {
		namespacedNames := lo.Map(pclq.associatedPodNames, func(associatedPodName string, _ int) groveschedulerv1alpha1.NamespacedName {
			return groveschedulerv1alpha1.NamespacedName{
				Namespace: namespace,
				Name:      associatedPodName,
			}
		})
		// sorting the slice of NamespaceName. This prevents unnecessary updates to the PodGang resource if the only thing
		// that is difference is the order of NamespaceNames.
		sort.Slice(namespacedNames, func(i, j int) bool {
			return namespacedNames[i].Name < namespacedNames[j].Name
		})
		return groveschedulerv1alpha1.PodGroup{
			Name:          pclq.fqn,
			PodReferences: namespacedNames,
			MinReplicas:   pclq.minAvailable,
		}
	})
	return podGroups
}

// Convenience types and methods on these types that are used during sync flow run.
// ------------------------------------------------------------------------------------------------

// syncContext holds the relevant state required during the sync flow run.
type syncContext struct {
	ctx                  context.Context
	pcs                  *grovecorev1alpha1.PodCliqueSet
	logger               logr.Logger
	expectedPodGangs     []podGangInfo
	existingPodGangNames []string
	deletedPodGangNames  []string
	pclqs                []grovecorev1alpha1.PodClique
	unassignedPodsByPCLQ map[string][]corev1.Pod
	pclqPods             map[string][]corev1.Pod
}

func (sc *syncContext) getPodGangNamesPendingCreation() []string {
	return lo.FilterMap(sc.expectedPodGangs, func(podGang podGangInfo, _ int) (string, bool) {
		return podGang.fqn, !slices.Contains(sc.existingPodGangNames, podGang.fqn)
	})
}

func (sc *syncContext) initializeAssignedAndUnassignedPodsForPCS(podsByPLCQ map[string][]corev1.Pod) {
	for pclqName, pods := range podsByPLCQ {
		for _, pod := range pods {
			if metav1.HasLabel(pod.ObjectMeta, apicommon.LabelPodGang) {
				podGangName := pod.GetLabels()[apicommon.LabelPodGang]
				// Find the index to work with the original slice element, not a copy
				pgiIndex := slices.IndexFunc(sc.expectedPodGangs, func(pgi podGangInfo) bool {
					return podGangName == pgi.fqn
				})
				if pgiIndex == -1 {
					continue
				}
				// Work with the original element in the slice, not a copy
				sc.expectedPodGangs[pgiIndex].refreshAssociatedPCLQPods(pclqName, pod.Name)
			} else {
				sc.unassignedPodsByPCLQ[pclqName] = append(sc.unassignedPodsByPCLQ[pclqName], pod)
			}
		}
	}
}

func (sc *syncContext) getPodCliques(podGang podGangInfo) []grovecorev1alpha1.PodClique {
	constituentPCLQs := make([]grovecorev1alpha1.PodClique, 0, len(podGang.pclqs))
	for _, podGangConstituentPCLQInfo := range podGang.pclqs {
		for _, pclq := range sc.pclqs {
			if pclq.Name == podGangConstituentPCLQInfo.fqn {
				constituentPCLQs = append(constituentPCLQs, pclq)
			}
		}
	}
	return constituentPCLQs
}

// syncFlowResult captures the result of a sync flow run.
type syncFlowResult struct {
	// podsGangsPendingCreation are the names of PodGangs that could not be created in this sync run.
	// It could be due to all PCLQs not present, or it could be due to presence of at least one PCLQ that is not ready.
	podsGangsPendingCreation []string
	// createdPodGangNames are the names of the PodGangs that got created during the sync flow run.
	createdPodGangNames []string
	// errs are the list of errors during the sync flow run.
	errs []error
}

func (sfr *syncFlowResult) hasErrors() bool {
	return len(sfr.errs) > 0
}

func (sfr *syncFlowResult) recordError(err error) {
	sfr.errs = append(sfr.errs, err)
}

func (sfr *syncFlowResult) hasPodGangsPendingCreation() bool {
	return len(sfr.podsGangsPendingCreation) > 0
}

func (sfr *syncFlowResult) recordPodGangCreation(podGangName string) {
	sfr.createdPodGangNames = append(sfr.createdPodGangNames, podGangName)
}

func (sfr *syncFlowResult) recordPodGangPendingCreation(podGangName string) {
	sfr.podsGangsPendingCreation = append(sfr.podsGangsPendingCreation, podGangName)
}

func (sfr *syncFlowResult) getAggregatedError() error {
	return errors.Join(sfr.errs...)
}

// podGangInfo is a convenience type that holds the information about
// its constituent PodClique names and expected replicas per PodClique for this PodGang.
// Each PodClique constituent is directly mapped to a groveschedulerv1alpha1.PodGroup.
// This struct will be used to check if all pods required by this PodGang are created and determine if this PodGang can be created.
type podGangInfo struct {
	// fqn is a fully qualified name of a PodGang.
	fqn string
	// pclqs holds the relevant information for all constituent PodCliques for this PodGang.
	pclqs []pclqInfo
}

func (pgi *podGangInfo) refreshAssociatedPCLQPods(pclqName string, newlyAssociatedPods ...string) {
	for i := range pgi.pclqs {
		if pgi.pclqs[i].fqn == pclqName {
			pgi.pclqs[i].associatedPodNames = append(pgi.pclqs[i].associatedPodNames, newlyAssociatedPods...)
		}
	}
}

// pclqInfo represents a groveschedulerv1alpha1.PodGroup and captures information relative to the PodGang of which
// this PodClique is a constituent.
type pclqInfo struct {
	// fqn is a fully qualified name for the PodClique
	fqn string
	// replicas is the number of Pods that are assigned to the PodGang for which this PodClique is a constituent.
	replicas int32
	// minAvailable is the minimum number of pods that are required for gang scheduling from this PodClique
	minAvailable int32
	// associatedPodNames are Pod names (having this PodClique as an owner) that have already been associated to this PodGang.
	// This will be updated as and when pods are either deleted or new pods are associated.
	associatedPodNames []string
}
