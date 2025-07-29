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

package podclique

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errListPodClique             grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUE"
	errSyncPodClique             grovecorev1alpha1.ErrorCode = "ERR_SYNC_PODCLIQUE"
	errDeletePodClique           grovecorev1alpha1.ErrorCode = "ERR_DELETE_PODCLIQUE"
	errGetPodGangSet             grovecorev1alpha1.ErrorCode = "ERR_GET_PODGANGSET"
	errMissingPGSReplicaIndex    grovecorev1alpha1.ErrorCode = "ERR_MISSING_PODGANGSET_REPLICA_INDEX"
	errReplicaIndexIntConversion grovecorev1alpha1.ErrorCode = "ERR_PODGANGSET_REPLICA_INDEX_CONVERSION"
	errCodeListPodCliquesForPCSG grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUE_FOR_PCSG"
	errCodeCreatePodClique       grovecorev1alpha1.ErrorCode = "ERR_CREATE_PODCLIQUE"
)

type _resource struct {
	client        client.Client
	scheme        *runtime.Scheme
	eventRecorder record.EventRecorder
}

// New creates an instance of PodClique component operator.
func New(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) component.Operator[grovecorev1alpha1.PodCliqueScalingGroup] {
	return &_resource{
		client:        client,
		scheme:        scheme,
		eventRecorder: eventRecorder,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the PodClique Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, logger logr.Logger, pcsgObjMeta metav1.ObjectMeta) ([]string, error) {
	logger.Info("Looking for existing PodCliques managed by PodCliqueScalingGroup")
	pclqPartialObjMetaList, err := k8sutils.ListExistingPartialObjectMetadata(ctx,
		r.client,
		grovecorev1alpha1.SchemeGroupVersion.WithKind("PodClique"),
		pcsgObjMeta,
		getPodCliqueSelectorLabels(pcsgObjMeta))
	if err != nil {
		return nil, groveerr.WrapError(err,
			errListPodClique,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodCliques for PodCliqueScalingGroup: %v", k8sutils.GetObjectKeyFromObjectMeta(pcsgObjMeta)),
		)
	}
	return k8sutils.FilterMapOwnedResourceNames(pcsgObjMeta, pclqPartialObjMetaList), nil
}

// Sync synchronizes all resources that the PodClique Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) error {
	expectedPCLQs := int(pcsg.Spec.Replicas) * len(pcsg.Spec.CliqueNames)
	pgs, err := componentutils.GetOwnerPodGangSet(ctx, r.client, pcsg.ObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errGetPodGangSet,
			component.OperationSync,
			fmt.Sprintf("failed to get owner PodGangSet for PodCliqueScalingGroup %s", client.ObjectKeyFromObject(pcsg)),
		)
	}
	existingPCLQs, err := r.getExistingPCLQs(ctx, pcsg)
	if err != nil {
		return err
	}
	// If there are excess PodCliques than expected, delete the ones that are no longer expected but existing.
	// This can happen when PCSG replicas have been scaled-in.
	if err = r.triggerDeletionOfExcessPCLQs(ctx, logger, pgs.ObjectMeta, pcsg, existingPCLQs, expectedPCLQs); err != nil {
		return err
	}
	// Identify PCSG replicas for which MinAvailable is breached for even a single constituent PCLQ. For such a PCSG replica terminate all PCLQs for that replica.
	shouldRequeue, err := r.triggerDeletionOfMinAvailableBreachedPCSGReplicas(ctx, logger, pgs, pcsg)
	if err != nil {
		return err
	}
	// Create or update the expected PodCliques as per the PodCliqueScalingGroup configurations defined in the PodGangSet.
	existingPCLQFQNs := lo.Map(existingPCLQs, func(pclq grovecorev1alpha1.PodClique, _ int) string { return pclq.Name })
	if err = r.createExpectedPCLQs(ctx, logger, pgs, pcsg, existingPCLQFQNs, expectedPCLQs); err != nil {
		return err
	}

	if shouldRequeue {
		return groveerr.New(groveerr.ErrCodeRequeueAfter,
			component.OperationSync,
			"Requeuing to re-process PCLQs that have breached MinAvailable but not crossed TerminationDelay",
		)
	}

	return nil
}

func (r _resource) getExistingPCLQs(ctx context.Context, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ([]grovecorev1alpha1.PodClique, error) {
	existingPCLQs, err := componentutils.GetPCLQsByOwner(ctx, r.client, grovecorev1alpha1.PodCliqueScalingGroupKind, client.ObjectKeyFromObject(pcsg), getPodCliqueSelectorLabels(pcsg.ObjectMeta))
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodCliquesForPCSG,
			component.OperationSync,
			fmt.Sprintf("Unable to fetch existing PodCliques for PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pcsg)),
		)
	}
	return existingPCLQs, nil
}

func (r _resource) triggerDeletionOfExcessPCLQs(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, existingPCLQs []grovecorev1alpha1.PodClique, numExpectedPCLQs int) error {
	// Check if the number of existing PodCliques is greater than expected, if so, we need to delete the extra ones.
	diff := len(existingPCLQs) - numExpectedPCLQs
	if diff > 0 {
		logger.Info("Found more PodCliques than expected, triggering deletion of excess PodCliques", "expected", numExpectedPCLQs, "existing", len(existingPCLQs), "diff", diff)
		// collect the names of the extra PodCliques to delete
		deletionCandidatePodGangNames := getExcessPCSGReplicaIndexesToDelete(pcsg, existingPCLQs)
		reason := fmt.Sprintf("Delete excess PodCliques associated to PodGangs %v for PodCliqueScalingGroup %v", deletionCandidatePodGangNames, client.ObjectKeyFromObject(pcsg))
		deletionTasks := r.createDeleteTasks(logger, pgsObjMeta, pcsg.Name, deletionCandidatePodGangNames, reason)
		return r.triggerDeletionOfPodCliques(ctx, logger, client.ObjectKeyFromObject(pcsg), deletionTasks)
	}
	return nil
}

func (r _resource) triggerDeletionOfMinAvailableBreachedPCSGReplicas(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) (shouldRequeue bool, err error) {
	terminationDelay := pgs.Spec.Template.TerminationDelay.Duration
	pcsgReplicaIndexesToDelete, pcsgReplicaIndexesRequiringRequeue, err := r.findPCSGReplicasToDelete(ctx, pcsg, terminationDelay, logger)
	if err != nil {
		return
	}
	numMinAvailableBreachedPCSGReplicas := len(pcsgReplicaIndexesToDelete) + len(pcsgReplicaIndexesRequiringRequeue)
	// If all the PCSG *replicas* are having MinAvailableBreached condition set to true, then that means that now PCSG.Status.Condition will have
	// MinAvailableBreached set to true. At this point of time, it is now the responsibility of PGS reconciler to ensure that it deletes the PGS replica.
	if numMinAvailableBreachedPCSGReplicas == int(pcsg.Spec.Replicas) {
		logger.Info("For each PCSG replica, MinAvailableBreached is true, delegating the responsibility to PGS reconciler to trigger the deletion of PGS replica.")
		return
	}
	if len(pcsgReplicaIndexesToDelete) > 0 {
		reason := fmt.Sprintf("Delete PodCliques %v for PodCliqueScalingGroup %v which have breached MinAvailable longer than TerminationDelay: %s", pcsgReplicaIndexesToDelete, client.ObjectKeyFromObject(pcsg), terminationDelay)
		pclqGangTerminationTasks := r.createDeleteTasks(logger, pgs.ObjectMeta, pcsg.Name, pcsgReplicaIndexesToDelete, reason)
		if err = r.triggerDeletionOfPodCliques(ctx, logger, client.ObjectKeyFromObject(pcsg), pclqGangTerminationTasks); err != nil {
			return
		}
	}
	if len(pcsgReplicaIndexesRequiringRequeue) > 0 {
		logger.Info("Found PCLQs with MinAvailable breached but which have not crossed TerminationDelay", "pclqs", pcsgReplicaIndexesRequiringRequeue, "terminationDelay", terminationDelay)
		shouldRequeue = true
		// return true to trigger a requeue
		return
	}
	return
}

func (r _resource) createExpectedPCLQs(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, existingPCLQNames []string, numExpectedPCLQs int) error {
	tasks := make([]utils.Task, 0, numExpectedPCLQs)
	for pcsgReplicaIndex := range pcsg.Spec.Replicas {
		pclqFQNs := lo.FilterMap(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, _ int) (string, bool) {
			pclqFQN := grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{
				Name:    pcsg.Name,
				Replica: int(pcsgReplicaIndex),
			}, pclqTemplateSpec.Name)
			return pclqFQN, slices.Contains(pcsg.Spec.CliqueNames, pclqTemplateSpec.Name)
		})

		for _, pclqFQN := range pclqFQNs {
			if slices.Contains(existingPCLQNames, pclqFQN) {
				continue
			}
			pclqObjectKey := client.ObjectKey{
				Name:      pclqFQN,
				Namespace: pcsg.Namespace,
			}
			createTask := utils.Task{
				Name: fmt.Sprintf("CreatePodClique-%s", pclqObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreate(ctx, logger, pgs, pcsg, int(pcsgReplicaIndex), pclqObjectKey)
				},
			}
			tasks = append(tasks, createTask)
		}
	}
	if runResult := utils.RunConcurrently(ctx, logger, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error Create of PodCliques for PodCliqueScalingGroup: %v, run summary: %s", client.ObjectKeyFromObject(pcsg), runResult.GetSummary()),
		)
	}
	return nil
}

func (r _resource) findPCSGReplicasToDelete(ctx context.Context, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, terminationDelay time.Duration, logger logr.Logger) (pcsgReplicaIndexesToTerminate []string, pcsgReplicaIndexToRequeue []string, err error) {
	existingPCLQs, err := componentutils.GetPCLQsByOwner(ctx, r.client, grovecorev1alpha1.PodCliqueScalingGroupKind, client.ObjectKeyFromObject(pcsg), getPodCliqueSelectorLabels(pcsg.ObjectMeta))
	if err != nil {
		err = groveerr.WrapError(err,
			errCodeListPodCliquesForPCSG,
			component.OperationSync,
			fmt.Sprintf("Unable to fetch existing PodCliques for PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pcsg)),
		)
		return
	}
	now := time.Now()
	// group existing PCLQs by PCSG replica index. These are PCLQs that belong to once replica of PCSG.
	pcsgReplicaIndexPCLQs := componentutils.GroupPCLQsByPCSGReplicaIndex(existingPCLQs)
	// For each PCSG replica check if minAvailable for any constituent PCLQ has been violated. Those PCSG replicas should be marked for termination.
	for pcsgReplicaIndex, pclqs := range pcsgReplicaIndexPCLQs {
		pclqNames, minWaitFor := componentutils.GetMinAvailableBreachedPCLQInfo(pclqs, terminationDelay, now)
		if len(pclqNames) > 0 {
			logger.Info("minAvailable breached for PCLQs", "pcsgReplicaIndex", pcsgReplicaIndex, "pclqNames", pclqNames, "minWaitFor", minWaitFor)
			if minWaitFor <= 0 {
				pcsgReplicaIndexesToTerminate = append(pcsgReplicaIndexesToTerminate, pcsgReplicaIndex)
			} else {
				pcsgReplicaIndexToRequeue = append(pcsgReplicaIndexToRequeue, pcsgReplicaIndex)
			}
		}
	}
	return
}

func (r _resource) triggerDeletionOfPodCliques(ctx context.Context, logger logr.Logger, pcsgObjectKey client.ObjectKey, deletionTasks []utils.Task) error {
	if runResult := utils.RunConcurrently(ctx, logger, deletionTasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error deleting PodCliques for PodCliqueScalingGroup: %v", pcsgObjectKey),
		)
	}
	logger.Info("Deleted PodCliques of PodCliqueScalingGroup", "pcsgObjectKey", pcsgObjectKey)
	return nil
}

func (r _resource) createDeleteTasks(logger logr.Logger, pgsObjMeta metav1.ObjectMeta, pcsgName string, pcsgReplicaIndexesToDelete []string, reason string) []utils.Task {
	deletionTasks := make([]utils.Task, 0, len(pcsgReplicaIndexesToDelete))
	for _, pcsgReplicaIndex := range pcsgReplicaIndexesToDelete {
		task := utils.Task{
			Name: "DeletePCSGReplicaPodCliques-" + pcsgReplicaIndex,
			Fn: func(ctx context.Context) error {
				if err := r.client.DeleteAllOf(ctx,
					&grovecorev1alpha1.PodClique{},
					client.InNamespace(pgsObjMeta.Namespace),
					client.MatchingLabels(getLabelsToDeletePCSGReplicaIndexPCLQs(pgsObjMeta.Name, pcsgName, pcsgReplicaIndex))); err != nil {
					logger.Error(err, "failed to delete PodCliques for PCSG replica index", "pcsgReplicaIndex", pcsgReplicaIndex, "reason", reason)
					return err
				}
				return nil
			},
		}
		deletionTasks = append(deletionTasks, task)
	}
	return deletionTasks
}

func getLabelsToDeletePCSGReplicaIndexPCLQs(pgsName, pcsgName, pcsgReplicaIndex string) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey:                      component.NamePCSGPodClique,
			grovecorev1alpha1.LabelPodCliqueScalingGroup:             pcsgName,
			grovecorev1alpha1.LabelPodCliqueScalingGroupReplicaIndex: pcsgReplicaIndex,
		},
	)
}

func getExcessPCSGReplicaIndexesToDelete(pcsg *grovecorev1alpha1.PodCliqueScalingGroup, existingPCLQs []grovecorev1alpha1.PodClique) []string {
	expectedPCLQNames := getExpectedPodCliqueFQNs(pcsg)
	return lo.FilterMap(existingPCLQs, func(existingPCLQ grovecorev1alpha1.PodClique, _ int) (string, bool) {
		if slices.Contains(expectedPCLQNames, existingPCLQ.Name) {
			return "", false
		}
		pcsgReplicaIndex, ok := existingPCLQ.GetLabels()[grovecorev1alpha1.LabelPodCliqueScalingGroupReplicaIndex]
		if !ok {
			return "", false
		}
		return pcsgReplicaIndex, true
	})
}

// Delete deletes all resources that the PodClique Operator manages.
func (r _resource) Delete(ctx context.Context, logger logr.Logger, pcsgObjectMeta metav1.ObjectMeta) error {
	logger.Info("Triggering deletion of PodCliques managed by PodCliqueScalingGroup")
	existingPCLQNames, err := r.GetExistingResourceNames(ctx, logger, pcsgObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errListPodClique,
			component.OperationDelete,
			fmt.Sprintf("Unable to fetch existing PodClique names for PodCliqueScalingGroup: %v", k8sutils.GetObjectKeyFromObjectMeta(pcsgObjectMeta)),
		)
	}
	deleteTasks := make([]utils.Task, 0, len(existingPCLQNames))
	for _, pclqName := range existingPCLQNames {
		pclqObjectKey := client.ObjectKey{Name: pclqName, Namespace: pcsgObjectMeta.Namespace}
		task := utils.Task{
			Name: "DeletePodClique-" + pclqName,
			Fn: func(ctx context.Context) error {
				if err := client.IgnoreNotFound(r.client.Delete(ctx, emptyPodClique(pclqObjectKey))); err != nil {
					return groveerr.WrapError(err,
						errDeletePodClique,
						component.OperationDelete,
						fmt.Sprintf("Failed to delete PodClique: %v for PodCliqueScalingGroup: %v", pclqObjectKey, k8sutils.GetObjectKeyFromObjectMeta(pcsgObjectMeta)),
					)
				}
				return nil
			},
		}
		deleteTasks = append(deleteTasks, task)
	}
	if runResult := utils.RunConcurrently(ctx, logger, deleteTasks); runResult.HasErrors() {
		logger.Error(runResult.GetAggregatedError(), "Error deleting PodCliques", "run summary", runResult.GetSummary())
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errDeletePodClique,
			component.OperationDelete,
			fmt.Sprintf("Error deleting PodCliques for PodCliqueScalingGroup: %v", k8sutils.GetObjectKeyFromObjectMeta(pcsgObjectMeta)),
		)
	}

	logger.Info("Deleted PodCliques belonging to PodCliqueScalingGroup")
	return nil
}

func (r _resource) getPCSGTemplateNumPods(pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) int {
	var pcsgTemplateNumPods int
	pcMap := make(map[string]*grovecorev1alpha1.PodCliqueTemplateSpec, len(pgs.Spec.Template.Cliques))
	for _, pclqTemplateSpec := range pgs.Spec.Template.Cliques {
		pcMap[pclqTemplateSpec.Name] = pclqTemplateSpec
	}
	for _, pclqTemplateName := range pcsg.Spec.CliqueNames {
		pclqTemplateSpec, ok := pcMap[pclqTemplateName]
		if !ok {
			continue
		}
		pcsgTemplateNumPods += int(pclqTemplateSpec.Spec.Replicas)
	}
	return pcsgTemplateNumPods
}

func (r _resource) doCreate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int, pclqObjectKey client.ObjectKey) error {
	logger.Info("Running CreateOrUpdate PodClique", "pclqObjectKey", pclqObjectKey)
	pclq := emptyPodClique(pclqObjectKey)
	pcsgObjKey := client.ObjectKeyFromObject(pclq)
	if err := r.buildResource(logger, pgs, pcsg, pcsgReplicaIndex, pclq); err != nil {
		return err
	}
	if err := r.client.Create(ctx, pclq); err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.Info("PodClique creation failed as it already exists", "pclq", pclqObjectKey)
			return nil
		}
		return groveerr.WrapError(err,
			errCodeCreatePodClique,
			component.OperationSync,
			fmt.Sprintf("Error creating PodClique: %v for PodCliqueScalingGroup: %v", pclqObjectKey, pcsgObjKey),
		)
	}
	logger.Info("triggered create of PodClique For PodCliqueScalingGroup", "pcsg", pcsgObjKey, "pclqObjectKey", pclqObjectKey)
	return nil
}

func (r _resource) buildResource(logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int, pclq *grovecorev1alpha1.PodClique) error {
	var err error
	pclqObjectKey, pgsObjectKey := client.ObjectKeyFromObject(pclq), client.ObjectKeyFromObject(pgs)
	pclqTemplateSpec, foundAtIndex, ok := lo.FindIndexOf(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
		return strings.HasSuffix(pclq.Name, pclqTemplateSpec.Name)
	})
	if !ok {
		logger.Info("Error building PodClique resource, PodClique template spec not found in PodGangSet", "podCliqueObjectKey", pclqObjectKey, "podGangSetObjectKey", pgsObjectKey)
		return groveerr.New(errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error building PodClique resource, PodCliqueTemplateSpec for PodClique: %v not found in PodGangSet: %v", pclqObjectKey, pgsObjectKey),
		)
	}
	// Set PodClique.ObjectMeta
	// ------------------------------------
	if err = controllerutil.SetControllerReference(pcsg, pclq, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for PodClique: %v", client.ObjectKeyFromObject(pclq)),
		)
	}
	pgsReplica, err := getPGSReplicaFromPCSG(pcsg)
	if err != nil {
		return err
	}
	pclq.Labels = getLabels(pgs.Name, pgsReplica, pcsg.Name, pcsgReplicaIndex, pclqObjectKey, pclqTemplateSpec)

	pclq.Annotations = pclqTemplateSpec.Annotations
	// set PodCliqueSpec
	// ------------------------------------
	pclq.Spec = pclqTemplateSpec.Spec
	pcsgTemplateNumPods := r.getPCSGTemplateNumPods(pgs, pcsg)
	r.addEnvironmentVariablesToPodContainerSpecs(pclq, pcsgTemplateNumPods)
	dependentPclqNames, err := identifyFullyQualifiedStartupDependencyNames(pgs, pclq, pgsReplica, foundAtIndex)
	if err != nil {
		return err
	}
	pclq.Spec.StartsAfter = dependentPclqNames
	return nil
}

func (r _resource) addEnvironmentVariablesToPodContainerSpecs(pclq *grovecorev1alpha1.PodClique, pcsgTemplateNumPods int) {
	pcsgEnvVars := []corev1.EnvVar{
		{
			Name: grovecorev1alpha1.EnvVarPCSGName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: fmt.Sprintf("metadata.labels['%s']", grovecorev1alpha1.LabelPodCliqueScalingGroup),
				},
			},
		},
		{
			Name:  grovecorev1alpha1.EnvVarPCSGTemplateNumPods,
			Value: strconv.Itoa(pcsgTemplateNumPods),
		},
		{
			Name: grovecorev1alpha1.EnvVarPCSGIndex,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: fmt.Sprintf("metadata.labels['%s']", grovecorev1alpha1.LabelPodCliqueScalingGroupReplicaIndex),
				},
			},
		},
	}
	pclqObjPodSpec := &pclq.Spec.PodSpec
	componentutils.AddEnvVarsToContainers(pclqObjPodSpec.Containers, pcsgEnvVars)
	componentutils.AddEnvVarsToContainers(pclqObjPodSpec.InitContainers, pcsgEnvVars)
}

func getExpectedPodCliqueFQNs(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) []string {
	expectedPCLQFQNs := make([]string, 0, int(pcsg.Spec.Replicas)*len(pcsg.Spec.CliqueNames))
	for pcsgReplicaIndex := range pcsg.Spec.Replicas {
		pclqFQNs := lo.Map(pcsg.Spec.CliqueNames, func(cliqueName string, _ int) string {
			return grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{
				Name:    pcsg.Name,
				Replica: int(pcsgReplicaIndex),
			}, cliqueName)
		})
		expectedPCLQFQNs = append(expectedPCLQFQNs, pclqFQNs...)
	}
	return expectedPCLQFQNs
}

func getPGSReplicaFromPCSG(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) (int, error) {
	pgsReplicaIndex, ok := pcsg.GetLabels()[grovecorev1alpha1.LabelPodGangSetReplicaIndex]
	if !ok {
		return 0, groveerr.New(errMissingPGSReplicaIndex, component.OperationSync, fmt.Sprintf("failed to get the PodGangSet replica ind value from the labels for PodCliqueScalingGroup %s", client.ObjectKeyFromObject(pcsg)))
	}
	pgsReplica, err := strconv.Atoi(pgsReplicaIndex)
	if err != nil {
		return 0, groveerr.WrapError(err,
			errReplicaIndexIntConversion,
			component.OperationSync,
			"failed to convert replica index value from string to integer",
		)
	}
	return pgsReplica, nil
}

func identifyFullyQualifiedStartupDependencyNames(pgs *grovecorev1alpha1.PodGangSet, pclq *grovecorev1alpha1.PodClique, pgsReplicaIndex, foundAtIndex int) ([]string, error) {
	cliqueStartupType := pgs.Spec.Template.StartupType
	if cliqueStartupType == nil {
		// Ideally this should never happen as the defaulting webhook should set it v1alpha1.CliqueStartupTypeInOrder as the default value.
		// If it is still nil, then by not returning an error we break the API contract. It is a bug that should be fixed.
		return nil, groveerr.New(errSyncPodClique, component.OperationSync, fmt.Sprintf("PodClique: %v has nil StartupType", client.ObjectKeyFromObject(pclq)))
	}
	switch *cliqueStartupType {
	case grovecorev1alpha1.CliqueStartupTypeInOrder:
		return getInOrderStartupDependencies(pgs, pgsReplicaIndex, foundAtIndex), nil
	case grovecorev1alpha1.CliqueStartupTypeExplicit:
		return getExplicitStartupDependencies(pgs.Name, pgsReplicaIndex, pclq), nil
	default:
		return nil, nil
	}
}

func getInOrderStartupDependencies(pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex, foundAtIndex int) []string {
	if foundAtIndex == 0 {
		return []string{}
	}
	previousClique := pgs.Spec.Template.Cliques[foundAtIndex-1]
	// get the name of the previous PodCliqueTemplateSpec
	previousPCLQName := grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex}, previousClique.Name)
	return []string{previousPCLQName}
}

func getExplicitStartupDependencies(pgsName string, pgsReplicaIndex int, pclq *grovecorev1alpha1.PodClique) []string {
	dependencies := make([]string, 0, len(pclq.Spec.StartsAfter))
	for _, dependency := range pclq.Spec.StartsAfter {
		dependencies = append(dependencies, grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: pgsReplicaIndex}, dependency))
	}
	return dependencies
}

func getPodCliqueSelectorLabels(pcsgObjectMeta metav1.ObjectMeta) map[string]string {
	pgsName := componentutils.GetPodGangSetName(pcsgObjectMeta)
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey:          component.NamePCSGPodClique,
			grovecorev1alpha1.LabelPodCliqueScalingGroup: pcsgObjectMeta.Name,
		},
	)
}

func getLabels(pgsName string, pgsReplicaIndex int, pcsgName string, pcsgReplicaIndex int, pclqObjectKey client.ObjectKey, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) map[string]string {
	podGangName := componentutils.CreatePodGangNameForPCSG(pgsName, pgsReplicaIndex, pcsgName, pcsgReplicaIndex)
	pclqComponentLabels := map[string]string{
		grovecorev1alpha1.LabelAppNameKey:            pclqObjectKey.Name,
		grovecorev1alpha1.LabelComponentKey:          component.NamePCSGPodClique,
		grovecorev1alpha1.LabelPodCliqueScalingGroup: pcsgName,
		grovecorev1alpha1.LabelPodGang:               podGangName, grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
		grovecorev1alpha1.LabelPodCliqueScalingGroupReplicaIndex: strconv.Itoa(pcsgReplicaIndex),
	}
	return lo.Assign(
		pclqTemplateSpec.Labels,
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		pclqComponentLabels,
	)
}

func emptyPodClique(objKey client.ObjectKey) *grovecorev1alpha1.PodClique {
	return &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
