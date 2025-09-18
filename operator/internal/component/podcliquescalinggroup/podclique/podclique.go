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
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveevents "github.com/NVIDIA/grove/operator/internal/component/events"
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

// Error codes for PodClique component operations.
const (
	errCodeListPodClique                                 grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUE"
	errCodeMissingStartupType                            grovecorev1alpha1.ErrorCode = "ERR_UNDEFINED_STARTUP_TYPE"
	errCodeSetPodCliqueOwnerReference                    grovecorev1alpha1.ErrorCode = "ERR_SET_PODCLIQUE_OWNER_REFERENCE"
	errCodeBuildPodClique                                grovecorev1alpha1.ErrorCode = "ERR_BUILD_PODCLIQUE"
	errCodeCreatePodCliques                              grovecorev1alpha1.ErrorCode = "ERR_CREATE_PODCLIQUES"
	errCodeDeletePodClique                               grovecorev1alpha1.ErrorCode = "ERR_DELETE_PODCLIQUE"
	errCodeGetPodCliqueSet                               grovecorev1alpha1.ErrorCode = "ERR_GET_PODCLIQUESET"
	errCodeMissingPCSReplicaIndex                        grovecorev1alpha1.ErrorCode = "ERR_MISSING_PODCLIQUESET_REPLICA_INDEX"
	errCodePCSReplicaIndexIntConversion                  grovecorev1alpha1.ErrorCode = "ERR_PODCLIQUESET_REPLICA_INDEX_CONVERSION"
	errCodeListPodCliquesForPCSG                         grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUE_FOR_PCSG"
	errCodeCreatePodClique                               grovecorev1alpha1.ErrorCode = "ERR_CREATE_PODCLIQUE"
	errCodeParsePodCliqueScalingGroupReplicaIndex        grovecorev1alpha1.ErrorCode = "ERR_PARSE_PODCLIQUESCALINGGROUP_REPLICA_INDEX"
	errCodeUpdateStatus                                  grovecorev1alpha1.ErrorCode = "ERR_UPDATE_STATUS"
	errCodeComputePendingPodCliqueScalingGroupUpdateWork grovecorev1alpha1.ErrorCode = "ERR_COMPUTE_PENDINGUPDATE_WORK"
)

// Sentinel errors for PodClique operations.
var (
	// errPCCGMinAvailableBreached indicates that the minimum available replicas
	// threshold has been breached for a PodCliqueScalingGroup.
	errPCCGMinAvailableBreached = errors.New("minAvailable has been breached for PodCliqueScalingGroup")
)

// _resource implements the component.Operator interface for managing PodClique resources
// within a PodCliqueScalingGroup.
type _resource struct {
	client        client.Client        // Kubernetes client for API operations
	scheme        *runtime.Scheme      // Runtime scheme for object serialization
	eventRecorder record.EventRecorder // Event recorder for Kubernetes events
}

// New creates a new PodClique component operator for managing PodClique resources
// owned by PodCliqueScalingGroup resources.
func New(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) component.Operator[grovecorev1alpha1.PodCliqueScalingGroup] {
	return &_resource{
		client:        client,
		scheme:        scheme,
		eventRecorder: eventRecorder,
	}
}

// GetExistingResourceNames returns the names of all existing PodClique resources
// managed by the specified PodCliqueScalingGroup.
func (r _resource) GetExistingResourceNames(ctx context.Context, logger logr.Logger, pcsgObjMeta metav1.ObjectMeta) ([]string, error) {
	logger.Info("Looking for existing PodCliques managed by PodCliqueScalingGroup")

	// List all PodClique resources owned by this PodCliqueScalingGroup
	pclqPartialObjMetaList, err := k8sutils.ListExistingPartialObjectMetadata(ctx,
		r.client,
		grovecorev1alpha1.SchemeGroupVersion.WithKind("PodClique"),
		pcsgObjMeta,
		getPodCliqueSelectorLabels(pcsgObjMeta))
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodClique,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodCliques for PodCliqueScalingGroup: %v", k8sutils.GetObjectKeyFromObjectMeta(pcsgObjMeta)),
		)
	}
	return k8sutils.FilterMapOwnedResourceNames(pcsgObjMeta, pclqPartialObjMetaList), nil
}

// Sync synchronizes all PodClique resources managed by the specified PodCliqueScalingGroup,
// ensuring the desired state matches the actual state in the cluster.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) error {
	// Prepare the synchronization context with current and expected state
	syncCtx, err := r.prepareSyncContext(ctx, logger, pcsg)
	if err != nil {
		return err
	}
	logger.Info("Starting PodCliqueScalingGroup Sync", "pcsgObjectKey", client.ObjectKeyFromObject(syncCtx.pcsg))

	// Execute the main synchronization logic
	if err = r.runSyncFlow(logger, syncCtx); err != nil {
		return err
	}
	return nil
}

// Delete removes all PodClique resources managed by the specified PodCliqueScalingGroup.
// This method is typically called during PodCliqueScalingGroup deletion or cleanup.
func (r _resource) Delete(ctx context.Context, logger logr.Logger, pcsgObjectMeta metav1.ObjectMeta) error {
	logger.Info("Triggering deletion of PodCliques managed by PodCliqueScalingGroup")

	// Get all PodClique resources owned by this PodCliqueScalingGroup
	existingPCLQNames, err := r.GetExistingResourceNames(ctx, logger, pcsgObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errCodeListPodClique,
			component.OperationDelete,
			fmt.Sprintf("Unable to fetch existing PodClique names for PodCliqueScalingGroup: %v", k8sutils.GetObjectKeyFromObjectMeta(pcsgObjectMeta)),
		)
	}

	// Create deletion tasks for all existing PodCliques
	deleteTasks := make([]utils.Task, 0, len(existingPCLQNames))
	for _, pclqName := range existingPCLQNames {
		pclqObjectKey := client.ObjectKey{Name: pclqName, Namespace: pcsgObjectMeta.Namespace}
		task := utils.Task{
			Name: "DeletePodClique-" + pclqName,
			Fn: func(ctx context.Context) error {
				if err := client.IgnoreNotFound(r.client.Delete(ctx, emptyPodClique(pclqObjectKey))); err != nil {
					return groveerr.WrapError(err,
						errCodeDeletePodClique,
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
			errCodeDeletePodClique,
			component.OperationDelete,
			fmt.Sprintf("Error deleting PodCliques for PodCliqueScalingGroup: %v", k8sutils.GetObjectKeyFromObjectMeta(pcsgObjectMeta)),
		)
	}

	logger.Info("Deleted PodCliques belonging to PodCliqueScalingGroup")
	return nil
}

// triggerDeletionOfPodCliques executes the provided deletion tasks concurrently
// to remove PodClique resources.
func (r _resource) triggerDeletionOfPodCliques(ctx context.Context, logger logr.Logger, pcsgObjectKey client.ObjectKey, deletionTasks []utils.Task) error {
	if len(deletionTasks) == 0 {
		return nil
	}

	// Execute all deletion tasks concurrently
	if runResult := utils.RunConcurrently(ctx, logger, deletionTasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errCodeDeletePodClique,
			component.OperationSync,
			fmt.Sprintf("Error deleting PodCliques for PodCliqueScalingGroup: %v", pcsgObjectKey),
		)
	}
	logger.Info("Deleted PodCliques of PodCliqueScalingGroup", "pcsgObjectKey", pcsgObjectKey)
	return nil
}

// createDeleteTasks creates a slice of deletion tasks for removing PodClique resources
// belonging to specific PodCliqueScalingGroup replica indices.
func (r _resource) createDeleteTasks(logger logr.Logger, pcs *grovecorev1alpha1.PodCliqueSet, pcsgName string, pcsgReplicasToDelete []string, reason string) []utils.Task {
	deletionTasks := make([]utils.Task, 0, len(pcsgReplicasToDelete))

	// Create a deletion task for each replica index
	for _, pcsgReplicaIndex := range pcsgReplicasToDelete {
		task := utils.Task{
			Name: "DeletePCSGReplicaPodCliques-" + pcsgName + "-" + pcsgReplicaIndex,
			Fn: func(ctx context.Context) error {
				// Delete all PodCliques belonging to this replica index
				if err := r.client.DeleteAllOf(ctx,
					&grovecorev1alpha1.PodClique{},
					client.InNamespace(pcs.Namespace),
					client.MatchingLabels(getLabelsToDeletePCSGReplicaIndexPCLQs(pcs.Name, pcsgName, pcsgReplicaIndex))); err != nil {
					r.eventRecorder.Eventf(pcs, corev1.EventTypeWarning, groveevents.ReasonPodCliqueScalingGroupReplicaDeleteFailed, "Error deleting PodCliqueScalingGroup %s ReplicaIndex %s : %v", pcsgName, pcsgReplicaIndex, err)
					logger.Error(err, "failed to delete PodCliques for PCSG replica index", "pcsgReplicaIndex", pcsgReplicaIndex, "reason", reason)
					return err
				}
				logger.Info("Deleting PodCliqueScalingGroup replica", "pcsgName", pcsgName, "pcsgReplicaIndex", pcsgReplicaIndex)
				r.eventRecorder.Eventf(pcs, corev1.EventTypeNormal, groveevents.ReasonPodCliqueScalingGroupReplicaDeleteSuccessful, "Deleted PodCliqueScalingGroup %s replicaIndex: %s", pcsgName, pcsgReplicaIndex)
				return nil
			},
		}
		deletionTasks = append(deletionTasks, task)
	}
	return deletionTasks
}

// getLabelsToDeletePCSGReplicaIndexPCLQs returns label selectors for identifying
// PodClique resources belonging to a specific PodCliqueScalingGroup replica index.
func getLabelsToDeletePCSGReplicaIndexPCLQs(pcsName, pcsgName, pcsgReplicaIndex string) map[string]string {
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsName),
		map[string]string{
			apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
			apicommon.LabelPodCliqueScalingGroup:             pcsgName,
			apicommon.LabelPodCliqueScalingGroupReplicaIndex: pcsgReplicaIndex,
		},
	)
}

// getPCSGTemplateNumPods calculates the total number of pods expected in a
// PodCliqueScalingGroup replica based on the PodClique template specifications.
func (r _resource) getPCSGTemplateNumPods(pcs *grovecorev1alpha1.PodCliqueSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) int {
	var pcsgTemplateNumPods int

	// Create a map for quick lookup of PodClique templates
	pcMap := make(map[string]*grovecorev1alpha1.PodCliqueTemplateSpec, len(pcs.Spec.Template.Cliques))
	for _, pclqTemplateSpec := range pcs.Spec.Template.Cliques {
		pcMap[pclqTemplateSpec.Name] = pclqTemplateSpec
	}

	// Sum up replicas for all PodCliques in this PodCliqueScalingGroup
	for _, pclqTemplateName := range pcsg.Spec.CliqueNames {
		pclqTemplateSpec, ok := pcMap[pclqTemplateName]
		if !ok {
			continue
		}
		pcsgTemplateNumPods += int(pclqTemplateSpec.Spec.Replicas)
	}
	return pcsgTemplateNumPods
}

// doCreate creates a new PodClique resource with the specified configuration.
// It builds the PodClique resource from templates and attempts to create it in the cluster.
func (r _resource) doCreate(ctx context.Context, logger logr.Logger, pcs *grovecorev1alpha1.PodCliqueSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int, pclqObjectKey client.ObjectKey) error {
	logger.Info("Running CreateOrUpdate PodClique", "pclqObjectKey", pclqObjectKey)

	// Create empty PodClique resource and build its specification
	pclq := emptyPodClique(pclqObjectKey)
	pcsgObjKey := client.ObjectKeyFromObject(pclq)
	if err := r.buildResource(logger, pcs, pcsg, pcsgReplicaIndex, pclq); err != nil {
		return err
	}

	// Attempt to create the PodClique resource
	if err := r.client.Create(ctx, pclq); err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.Info("PodClique creation failed as it already exists", "pclq", pclqObjectKey)
			return nil
		}
		r.eventRecorder.Eventf(pcsg, corev1.EventTypeWarning, groveevents.ReasonPodCliqueCreateFailed, "PodClique %v creation failed: %v", pclqObjectKey, err)
		return groveerr.WrapError(err,
			errCodeCreatePodClique,
			component.OperationSync,
			fmt.Sprintf("Error creating PodClique: %v for PodCliqueScalingGroup: %v", pclqObjectKey, pcsgObjKey),
		)
	}
	r.eventRecorder.Eventf(pcsg, corev1.EventTypeNormal, groveevents.ReasonPodCliqueCreateSuccessful, "PodClique %v created successfully", pclqObjectKey)
	logger.Info("Successfully created PodClique", "pclqObjectKey", pclqObjectKey)
	return nil
}

// buildResource constructs a complete PodClique resource specification from the
// PodCliqueSet template and PodCliqueScalingGroup configuration.
func (r _resource) buildResource(logger logr.Logger, pcs *grovecorev1alpha1.PodCliqueSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int, pclq *grovecorev1alpha1.PodClique) error {
	var err error
	pclqObjectKey, pcsObjectKey := client.ObjectKeyFromObject(pclq), client.ObjectKeyFromObject(pcs)

	// Find the corresponding PodClique template specification
	pclqTemplateSpec, foundAtIndex, ok := lo.FindIndexOf(pcs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
		return strings.HasSuffix(pclq.Name, pclqTemplateSpec.Name)
	})
	if !ok {
		logger.Info("Error building PodClique resource, PodClique template spec not found in PodCliqueSet", "podCliqueObjectKey", pclqObjectKey, "podCliqueSetObjectKey", pcsObjectKey)
		return groveerr.New(errCodeBuildPodClique,
			component.OperationSync,
			fmt.Sprintf("Error building PodClique resource, PodCliqueTemplateSpec for PodClique: %v not found in PodCliqueSet: %v", pclqObjectKey, pcsObjectKey),
		)
	}

	// Set owner reference to establish parent-child relationship
	if err = controllerutil.SetControllerReference(pcsg, pclq, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errCodeSetPodCliqueOwnerReference,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for PodClique: %v", client.ObjectKeyFromObject(pclq)),
		)
	}

	// Extract PodCliqueSet replica index from PodCliqueScalingGroup labels
	pcsReplicaIndex, err := getPCSReplicaFromPCSG(pcsg)
	if err != nil {
		return err
	}

	// Generate PodGang name for this PodClique
	podGangName := apicommon.GeneratePodGangNameForPodCliqueOwnedByPCSG(pcs, pcsReplicaIndex, pcsg, pcsgReplicaIndex)

	// Set labels and annotations from template
	pclq.Labels = getLabels(pcs, pcsReplicaIndex, pcsg, pcsgReplicaIndex, pclqObjectKey, pclqTemplateSpec, podGangName)
	pclq.Annotations = pclqTemplateSpec.Annotations

	// Build PodClique specification from template
	pclq.Spec = *pclqTemplateSpec.Spec.DeepCopy()
	pcsgTemplateNumPods := r.getPCSGTemplateNumPods(pcs, pcsg)
	r.addEnvironmentVariablesToPodContainerSpecs(pclq, pcsgTemplateNumPods)

	// Configure startup dependencies based on startup ordering
	dependentPCLQNames, err := identifyFullyQualifiedStartupDependencyNames(pcs, pcsReplicaIndex, pcsg, pcsgReplicaIndex, pclq, foundAtIndex)
	if err != nil {
		return err
	}
	pclq.Spec.StartsAfter = dependentPCLQNames
	return nil
}

// addEnvironmentVariablesToPodContainerSpecs adds PodCliqueScalingGroup-specific
// environment variables to all containers and init containers in the PodClique.
func (r _resource) addEnvironmentVariablesToPodContainerSpecs(pclq *grovecorev1alpha1.PodClique, pcsgTemplateNumPods int) {
	// Define PodCliqueScalingGroup-specific environment variables
	pcsgEnvVars := []corev1.EnvVar{
		{
			Name: constants.EnvVarPodCliqueScalingGroupName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: fmt.Sprintf("metadata.labels['%s']", apicommon.LabelPodCliqueScalingGroup),
				},
			},
		},
		{
			Name:  constants.EnvVarPodCliqueScalingGroupTemplateNumPods,
			Value: strconv.Itoa(pcsgTemplateNumPods),
		},
		{
			Name: constants.EnvVarPodCliqueScalingGroupIndex,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: fmt.Sprintf("metadata.labels['%s']", apicommon.LabelPodCliqueScalingGroupReplicaIndex),
				},
			},
		},
	}

	// Add environment variables to all containers and init containers
	pclqObjPodSpec := &pclq.Spec.PodSpec
	componentutils.AddEnvVarsToContainers(pclqObjPodSpec.Containers, pcsgEnvVars)
	componentutils.AddEnvVarsToContainers(pclqObjPodSpec.InitContainers, pcsgEnvVars)
}

// getPCSReplicaFromPCSG extracts the PodCliqueSet replica index from the
// PodCliqueScalingGroup's labels.
func getPCSReplicaFromPCSG(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) (int, error) {
	// Get the PodCliqueSet replica index from labels
	pcsReplicaIndex, ok := pcsg.GetLabels()[apicommon.LabelPodCliqueSetReplicaIndex]
	if !ok {
		return 0, groveerr.New(errCodeMissingPCSReplicaIndex, component.OperationSync, fmt.Sprintf("failed to get the PodCliqueSet replica ind value from the labels for PodCliqueScalingGroup %s", client.ObjectKeyFromObject(pcsg)))
	}

	// Convert string index to integer
	pcsReplica, err := strconv.Atoi(pcsReplicaIndex)
	if err != nil {
		return 0, groveerr.WrapError(err,
			errCodePCSReplicaIndexIntConversion,
			component.OperationSync,
			"failed to convert replica index value from string to integer",
		)
	}
	return pcsReplica, nil
}

// identifyFullyQualifiedStartupDependencyNames determines the startup dependencies
// for a PodClique based on the startup type configuration.
func identifyFullyQualifiedStartupDependencyNames(pcs *grovecorev1alpha1.PodCliqueSet, pcsReplicaIndex int, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int, pclq *grovecorev1alpha1.PodClique, foundAtIndex int) ([]string, error) {
	cliqueStartupType := pcs.Spec.Template.StartupType
	if cliqueStartupType == nil {
		// This should never happen as the defaulting webhook should set CliqueStartupTypeInOrder as default
		return nil, groveerr.New(errCodeMissingStartupType, component.OperationSync, fmt.Sprintf("PodClique: %v has nil StartupType", client.ObjectKeyFromObject(pclq)))
	}

	// Determine dependencies based on startup type
	switch *cliqueStartupType {
	case grovecorev1alpha1.CliqueStartupTypeInOrder:
		return getInOrderStartupDependencies(pcs, pcsReplicaIndex, pcsg, pcsgReplicaIndex, foundAtIndex), nil
	case grovecorev1alpha1.CliqueStartupTypeExplicit:
		return getExplicitStartupDependencies(pcs, pcsReplicaIndex, pcsg, pcsgReplicaIndex, pclq), nil
	default:
		return nil, nil
	}
}

// getInOrderStartupDependencies generates startup dependencies for in-order startup type,
// where PodCliques start in the order they are defined in the template.
func getInOrderStartupDependencies(pcs *grovecorev1alpha1.PodCliqueSet, pcsReplicaIndex int, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex, foundAtIndex int) []string {
	// First PodClique in the sequence has no dependencies
	if foundAtIndex == 0 {
		return nil
	}

	// Get the name of the previous PodClique in the sequence
	parentCliqueName := pcs.Spec.Template.Cliques[foundAtIndex-1].Name

	// Handle base PodGang dependencies (replicas within minAvailable)
	if pcsgReplicaIndex < int(*pcsg.Spec.MinAvailable) {
		return componentutils.GenerateDependencyNamesForBasePodGang(pcs, pcsReplicaIndex, parentCliqueName)
	}

	// For scaled PodGangs, startup ordering is only enforced within the same PodGang
	// PodCliques from the base PodGang are not considered as dependencies
	if !slices.Contains(pcsg.Spec.CliqueNames, parentCliqueName) {
		return nil
	}

	// Return dependency on the previous PodClique in the same scaled replica
	return []string{
		apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{Name: pcsg.Name, Replica: pcsgReplicaIndex}, parentCliqueName),
	}
}

// getExplicitStartupDependencies generates startup dependencies for explicit startup type,
// where dependencies are explicitly defined in the PodClique template.
func getExplicitStartupDependencies(pcs *grovecorev1alpha1.PodCliqueSet, pcsReplicaIndex int, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int, pclq *grovecorev1alpha1.PodClique) []string {
	parentCliqueNames := make([]string, 0, len(pclq.Spec.StartsAfter))

	// Handle base PodGang dependencies (replicas within minAvailable)
	if pcsgReplicaIndex < int(*pcsg.Spec.MinAvailable) {
		for _, dependency := range pclq.Spec.StartsAfter {
			parentCliqueNames = append(parentCliqueNames, componentutils.GenerateDependencyNamesForBasePodGang(pcs, pcsReplicaIndex, dependency)...)
		}
		return parentCliqueNames
	}

	// For scaled PodGangs, only consider dependencies within the same PodCliqueScalingGroup
	for _, dependency := range pclq.Spec.StartsAfter {
		// Skip dependencies not part of this PodCliqueScalingGroup
		if !slices.Contains(pcsg.Spec.CliqueNames, dependency) {
			continue
		}
		parentCliqueNames = append(parentCliqueNames, apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{Name: pcsg.Name, Replica: pcsgReplicaIndex}, dependency))
	}
	return parentCliqueNames
}

// getPodCliqueSelectorLabels returns label selectors for finding PodClique resources
// managed by a specific PodCliqueScalingGroup.
func getPodCliqueSelectorLabels(pcsgObjectMeta metav1.ObjectMeta) map[string]string {
	pcsName := componentutils.GetPodCliqueSetName(pcsgObjectMeta)
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsName),
		map[string]string{
			apicommon.LabelComponentKey:          apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
			apicommon.LabelPodCliqueScalingGroup: pcsgObjectMeta.Name,
		},
	)
}

// getLabels constructs the complete set of labels for a PodClique resource,
// combining template labels, default labels, and component-specific labels.
func getLabels(pcs *grovecorev1alpha1.PodCliqueSet, pcsReplicaIndex int, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int, pclqObjectKey client.ObjectKey, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, podGangName string) map[string]string {
	// Define component-specific labels
	pclqComponentLabels := map[string]string{
		apicommon.LabelAppNameKey:                        pclqObjectKey.Name,
		apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
		apicommon.LabelPodCliqueScalingGroup:             pcsg.Name,
		apicommon.LabelPodGang:                           podGangName,
		apicommon.LabelPodCliqueSetReplicaIndex:          strconv.Itoa(pcsReplicaIndex),
		apicommon.LabelPodCliqueScalingGroupReplicaIndex: strconv.Itoa(pcsgReplicaIndex),
		apicommon.LabelPodTemplateHash:                   componentutils.ComputePCLQPodTemplateHash(pclqTemplateSpec, pcs.Spec.Template.PriorityClassName),
	}

	// Add base PodGang label for scaled PodGang resources (beyond minAvailable)
	basePodGangName := apicommon.GenerateBasePodGangName(
		apicommon.ResourceNameReplica{Name: pcs.Name, Replica: pcsReplicaIndex},
	)
	if podGangName != basePodGangName {
		// This PodClique belongs to a scaled PodGang - add the base PodGang reference
		pclqComponentLabels[apicommon.LabelBasePodGang] = basePodGangName
	}

	// Merge template labels, default labels, and component labels
	return lo.Assign(
		pclqTemplateSpec.Labels,
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcs.Name),
		pclqComponentLabels,
	)
}

// emptyPodClique creates a new PodClique resource with only the name and namespace set.
func emptyPodClique(objKey client.ObjectKey) *grovecorev1alpha1.PodClique {
	return &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
