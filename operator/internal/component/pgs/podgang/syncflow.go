package podgang

import (
	"context"
	"fmt"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"slices"
	"strconv"
)

type syncContext struct {
	ctx                  context.Context
	pgs                  *grovecorev1alpha1.PodGangSet
	logger               logr.Logger
	expectedPodGangs     []podGangInfo
	existingPodGangNames []string
	pclqs                []grovecorev1alpha1.PodClique
	podsByPCLQ           map[string][]corev1.Pod
}

func (sc syncContext) getPodGangsPendingCreation() []podGangInfo {
	diff := len(sc.existingPodGangNames) - len(sc.expectedPodGangs)
	if diff > 0 {
		return nil
	}
	return lo.Filter(sc.expectedPodGangs, func(podGang podGangInfo, _ int) bool {
		return !slices.Contains(sc.existingPodGangNames, podGang.fqn)
	})
}

// getTargetPodGangsForUpdate returns a slice of PodGangs where updates are expected.
// Only PodGangs that are created due to increase in PGS.Spec.Replicas can have their constituent PodCliques
// scaled since we only allow ScaleConfig to be defined for PodCliques that are not associated or part of a PodCliqueScalingGroup.
// If PodCliques.Spec.Replicas that are part of a PodCliqueScalingGroup are scaled then it will result in creation of new PodGangs.
// The existing PodGang will not be updated and those will be handled in PodGang creation flow.
func (sc syncContext) getTargetPodGangsForUpdate() []podGangInfo {
	return lo.Filter(sc.expectedPodGangs, func(pg podGangInfo, _ int) bool {
		return pg.creationReason == pgsReplicaIncrease
	})
}

type syncFlowResult struct {
	// podsGangsPendingCreation are the names of PodGangs that could not be created in this sync run.
	// It could be due to all PCLQs not present, or it could be due to presence of at least one PCLQ that is not ready.
	podsGangsPendingCreation []string
	createdPodGangNames      []string
	errs                     []error
}

func (sfr syncFlowResult) hasErrors() bool {
	return len(sfr.errs) > 0
}

type creationReason string

const (
	pgsReplicaIncrease  creationReason = "pgs-replica-increase"
	pcsgReplicaIncrease creationReason = "pcsg-replica-increase"
)

// podGangInfo is a convenience type that holds the information about
// its constituent PodClique names and expected replicas per PodClique for this PodGang.
// Each PodClique constituent is directly mapped to a groveschedulerv1alpha1.PodGroup.
// This struct will be used to check if all pods required by this PodGang are created and determine if this PodGang can be created.
type podGangInfo struct {
	// fqn is a fully qualified name of a PodGang.
	fqn string
	// creationReason is a reason the PodGang is created. It helps categorise PodGang.
	creationReason creationReason
	// pclqs holds the relevant information for all constituent PodCliques for this PodGang.
	pclqs []pclqInfo
}

// pclqInfo represents a groveschedulerv1alpha1.PodGroup and captures information relative to the PodGang of which
// this PodClique is a constituent.
type pclqInfo struct {
	// fqn is a fully qualified name for the PodClique
	fqn string
	// replicas is the number of Pods that are assigned to the PodGang for which this PodClique is a constituent.
	replicas int32
	// associatedPodNames are Pod names (having this PodClique as an owner) that have already been associated to this PodGang.
	// This will be updated as and when pods are either deleted or new pods are associated.
	associatedPodNames []string
}

// prepareSyncFlow
func (r _resource) prepareSyncFlow(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) (*syncContext, error) {
	pgsObjectKey := client.ObjectKeyFromObject(pgs)
	sc := &syncContext{
		ctx:    ctx,
		pgs:    pgs,
		logger: logger,
	}

	pclqs, err := r.getPCLQsForPGS(ctx, pgsObjectKey)
	if err != nil {
		groveerr.WrapError(err,
			errCodeListPodCliques,
			component.OperationSync,
			fmt.Sprintf("failed to list PodCliques for PodGangSet %v", pgsObjectKey),
		)
	}
	sc.pclqs = pclqs

	if err := r.computeExpectedPodGangs(sc); err != nil {
		return nil, groveerr.WrapError(err,
			errCodeComputeExistingPodGangs,
			component.OperationSync,
			fmt.Sprintf("failed to compute existing PodGangs for PodGangSet %v", pgsObjectKey),
		)
	}

	existingPodGangNames, err := r.GetExistingResourceNames(ctx, logger, pgs)
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodGangs,
			component.OperationSync,
			fmt.Sprintf("Failed to get existing PodGang names for PodGangSet: %v", client.ObjectKeyFromObject(sc.pgs)),
		)
	}
	sc.existingPodGangNames = existingPodGangNames

	podsByPCLQ, err := r.getPodsByPCLQForPGS(ctx, pgsObjectKey)
	if err != nil {
		groveerr.WrapError(err,
			errCodeListPods,
			component.OperationSync,
			fmt.Sprintf("failed to list Pods for PodGangSet %v", pgsObjectKey),
		)
	}
	sc.podsByPCLQ = podsByPCLQ
	return sc, nil
}

func (r _resource) runSyncFlow(syncCtx *syncContext) syncFlowResult {
	return syncFlowResult{}
}

func (r _resource) creatOrUpdatePodGangsTask(sc *syncContext) utils.Task {
	return utils.Task{
		Name: fmt.Sprintf("CreateOrUpdatePodGangs-%s", client.ObjectKeyFromObject(sc.pgs)),
		Fn: func(ctx context.Context) error {
			r.createNonExistingPodGangs(sc)
			//podGangUpdateCandidates := sc.getTargetPodGangsForUpdate()
			//r.updatePodGangs(podGangUpdateCandidates)
			return nil
		},
	}
}

func (r _resource) createNonExistingPodGangs(sc *syncContext) syncFlowResult {
	podGangsPendingCreation := sc.getPodGangsPendingCreation()
	if len(podGangsPendingCreation) == 0 {
		return syncFlowResult{}
	}

	return syncFlowResult{}
}

func (r _resource) deleteExcessPodGangsTask(sc *syncContext) utils.Task {
	return utils.Task{
		Name: fmt.Sprintf("DeleteExcessPodGangs-%s", client.ObjectKeyFromObject(sc.pgs)),
		Fn: func(ctx context.Context) error {
			expectedPodGangNames := lo.Map(sc.expectedPodGangs, func(pg podGangInfo, _ int) string {
				return pg.fqn
			})
			excessPodGangs, _ := lo.Difference(sc.existingPodGangNames, expectedPodGangNames)
			namespace := sc.pgs.Namespace
			for _, podGangToDelete := range excessPodGangs {
				pgObjectKey := client.ObjectKey{Namespace: namespace, Name: podGangToDelete}
				pg := emptyPodGang(pgObjectKey)
				if err := client.IgnoreNotFound(r.client.Delete(sc.ctx, pg)); err != nil {
					return groveerr.WrapError(err,
						errCodeDeleteExcessPodGang,
						component.OperationSync,
						fmt.Sprintf("failed to delete PodGang %v", pgObjectKey),
					)
				}
			}
			return nil
		},
	}
}

// computeExpectedPodGangs computes the expected PodGangs for the PodGangSet.
// It inspects the defined PodCliqueScalingGroup's and PodGangSet replicas to identify PodGangs.
func (r _resource) computeExpectedPodGangs(sc *syncContext) error {
	expectedPodGangs := make([]podGangInfo, 0, 50) // preallocate to avoid multiple allocations

	// For each PodGangSet replica there is going to be a PodGang. Add all pending PodGangs that should be created for each replica.
	expectedPodGangs = append(expectedPodGangs, getExpectedPodGangForPGSReplicas(sc)...)
	// For each replica of PodGangSet, get the pending PodGangs for each PodCliqueScalingGroup.
	for pgsReplica := range sc.pgs.Spec.Replicas {
		expectedPodGangsForPCSG, err := r.getExpectedPodGangsForPCSG(sc.ctx, sc.logger, sc.pgs, pgsReplica)
		if err != nil {
			return err
		}
		expectedPodGangs = append(expectedPodGangs, expectedPodGangsForPCSG...)
	}
	sc.expectedPodGangs = expectedPodGangs
	return nil
}

func getExpectedPodGangForPGSReplicas(sc *syncContext) []podGangInfo {
	expectedPodGangs := make([]podGangInfo, 0, int(sc.pgs.Spec.Replicas))
	for pgsReplica := range sc.pgs.Spec.Replicas {
		replicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: sc.pgs.Name, Replica: int(pgsReplica)}, nil)
		expectedPodGangs = append(expectedPodGangs, podGangInfo{
			fqn:            replicaPodGangName,
			creationReason: pgsReplicaIncrease,
			pclqs:          identifyConstituentPCLQsForPGSReplicaPodGang(sc, pgsReplica),
		})
	}
	return expectedPodGangs
}

func (r _resource) getExpectedPodGangsForPCSG(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pgsReplica int32) ([]podGangInfo, error) {
	if len(pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs) == 0 {
		return []podGangInfo{}, nil
	}
	existingPCSGs, err := r.getExistingPodCliqueScalingGroups(ctx, pgs.Name, pgsReplica)
	if err != nil {
		return nil, err
	}
	expectedPodGangs := make([]podGangInfo, 0, 50) // preallocate to avoid multiple allocations
	for _, pcsg := range existingPCSGs {
		if pcsg.Spec.Replicas == 1 {
			continue // First replica of PodCliqueScalingGroup is the first PodGang created for the PodGangSet replica, so it is already accounted for.
		}
		for i := 1; i < int(pcsg.Spec.Replicas); i++ {
			pcsgReplicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: int(pgsReplica)}, &grovecorev1alpha1.ResourceNameReplica{Name: pcsg.Name, Replica: i - 1})
			expectedPodGangs = append(expectedPodGangs, podGangInfo{
				fqn:            pcsgReplicaPodGangName,
				creationReason: pcsgReplicaIncrease,
				pclqs:          identifyConstituentPCLQsForPCSGPodGang(&pcsg, pgs.Spec.TemplateSpec.Cliques, logger),
			})
		}
	}
	return expectedPodGangs, nil
}

func (r _resource) getExistingPodCliqueScalingGroups(ctx context.Context, pgsName string, pgsReplica int32) ([]grovecorev1alpha1.PodCliqueScalingGroup, error) {
	pcsgList := &grovecorev1alpha1.PodCliqueScalingGroupList{}
	if err := r.client.List(ctx,
		pcsgList,
		client.InNamespace(pgsName),
		client.MatchingLabels(
			lo.Assign(
				k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
				map[string]string{
					grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(int(pgsReplica)),
				},
			),
		),
	); err != nil {
		return nil, err
	}
	return pcsgList.Items, nil
}

func identifyConstituentPCLQsForPGSReplicaPodGang(sc *syncContext, pgsReplica int32) []pclqInfo {
	constituentPCLQs := make([]pclqInfo, 0, len(sc.pgs.Spec.TemplateSpec.Cliques))
	for _, pclqTemplateSpec := range sc.pgs.Spec.TemplateSpec.Cliques {
		pclqFQN := grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: sc.pgs.Name, Replica: int(pgsReplica)}, pclqTemplateSpec.Name)
		var replicas int32
		if pclqTemplateSpec.Spec.ScaleConfig != nil {
			matchingPCLQ, _ := lo.Find(sc.pclqs, func(pclq grovecorev1alpha1.PodClique) bool {
				return pclqFQN == pclq.Name
			})
			replicas = matchingPCLQ.Spec.Replicas
		} else {
			replicas = sc.pgs.Spec.Replicas
		}
		constituentPCLQs = append(constituentPCLQs, pclqInfo{
			fqn:      pclqFQN,
			replicas: replicas,
		})
	}
	return constituentPCLQs
}

func identifyConstituentPCLQsForPCSGPodGang(pcsg *grovecorev1alpha1.PodCliqueScalingGroup, cliques []*grovecorev1alpha1.PodCliqueTemplateSpec, logger logr.Logger) []pclqInfo {
	constituentPCLQs := make([]pclqInfo, 0, len(pcsg.Spec.CliqueNames))
	for _, pclqName := range pcsg.Spec.CliqueNames {
		pclqTemplate, ok := lo.Find(cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return pclqTemplateSpec.Name == pclqName
		})
		if !ok {
			logger.Info("[WARN]: PodCliqueScalingGroup references a PodClique that does not exist in the PodGangSet. This should never happen.", "podCliqueName", pclqName)
			continue
		}
		constituentPCLQs = append(constituentPCLQs, pclqInfo{
			fqn:      pclqName,
			replicas: pclqTemplate.Spec.Replicas,
		})
	}
	return constituentPCLQs
}

func (r _resource) getPCLQsForPGS(ctx context.Context, pgsObjectKey client.ObjectKey) ([]grovecorev1alpha1.PodClique, error) {
	pclqList := &grovecorev1alpha1.PodCliqueList{}
	if err := r.client.List(ctx, pclqList,
		client.InNamespace(pgsObjectKey.Namespace),
		client.MatchingLabels(k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjectKey.Name))); err != nil {
		return nil, err
	}
	return pclqList.Items, nil
}

func (r _resource) getPodsByPCLQForPGS(ctx context.Context, pgsObjectKey client.ObjectKey) (map[string][]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := r.client.List(ctx,
		podList,
		client.InNamespace(pgsObjectKey.Namespace),
		client.MatchingLabels(k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjectKey.Name)),
	); err != nil {
		return nil, err
	}
	podsByPCLQ := lo.GroupByMap(podList.Items, func(pod corev1.Pod) (string, corev1.Pod) {
		return k8sutils.GetFirstOwnerName(pod.ObjectMeta), pod
	})
	return podsByPCLQ, nil
}
