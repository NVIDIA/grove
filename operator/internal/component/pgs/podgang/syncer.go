package podgang

import (
	"context"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"slices"
	"strconv"
)

type pendingPodGang struct {
	name         string
	constituents []constituent
	affinityTo   string
}
type constituent struct {
	podCliqueFQN string
	replicas     int32
}

func (r _resource) computeColocatedPodGangsPendingCreation(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, existingPodGangNames []string) ([]pendingPodGang, error) {
	pendingPodGangs := make([]pendingPodGang, 0, 50) // preallocate to avoid multiple allocations

	// For each PodGangSet replica there is going to be a PodGang. Add all pending PodGangs that should be created for each replica.
	pendingPodGangs = append(pendingPodGangs, getPendingPodGangForPGSReplicas(pgs, existingPodGangNames)...)
	// For each replica of PodGangSet, get the pending PodGangs for each PodCliqueScalingGroup.
	for pgsReplica := range pgs.Spec.Replicas {
		pendingPodGangsForPCSG, err := r.getPendingPodGangsForPCSG(ctx, logger, pgs, pgsReplica, existingPodGangNames)
		if err != nil {
			return nil, err
		}
		pendingPodGangs = append(pendingPodGangs, pendingPodGangsForPCSG...)
	}

	return pendingPodGangs, nil
}

func getPendingPodGangForPGSReplicas(pgs *grovecorev1alpha1.PodGangSet, existingPodGangNames []string) []pendingPodGang {
	pendingPodGangs := make([]pendingPodGang, 0, int(pgs.Spec.Replicas))
	for pgsReplica := range pgs.Spec.Replicas {
		replicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: int(pgsReplica)}, nil)
		if !slices.Contains(existingPodGangNames, replicaPodGangName) {
			pendingPodGangs = append(pendingPodGangs, pendingPodGang{
				name:         replicaPodGangName,
				constituents: getConstituentPCLQsForPGSReplicaPodGang(pgs.Name, pgsReplica, pgs.Spec.TemplateSpec.Cliques),
			})
		}
	}
	return pendingPodGangs
}

func getConstituentPCLQsForPGSReplicaPodGang(pgsName string, pgsReplica int32, cliques []*grovecorev1alpha1.PodCliqueTemplateSpec) []constituent {
	constituents := make([]constituent, 0, len(cliques))
	for _, pclq := range cliques {
		constituents = append(constituents, constituent{
			podCliqueFQN: grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: int(pgsReplica)}, pclq.Name),
			replicas:     pclq.Spec.Replicas,
		})
	}
	return constituents
}

// TODO: Is there a possibility that PodCliqueScalingGroupConfigs contains a PCSG that is still not reflecting when queried?
func (r _resource) getPendingPodGangsForPCSG(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pgsReplica int32, existingPodGangNames []string) ([]pendingPodGang, error) {
	if len(pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs) == 0 {
		return []pendingPodGang{}, nil
	}
	existingPCSGs, err := r.getExistingPodCliqueScalingGroups(ctx, pgs.Name, pgsReplica)
	if err != nil {
		return nil, err
	}
	pendingPodGangs := make([]pendingPodGang, 0, 50) // preallocate to avoid multiple allocations
	for _, pcsg := range existingPCSGs {
		if pcsg.Spec.Replicas == 1 {
			continue // First replica of PodCliqueScalingGroup is the first PodGang created for the PodGangSet replica, so it is already accounted for.
		}
		for i := 1; i < int(pcsg.Spec.Replicas); i++ {
			pcsgReplicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: int(pgsReplica)}, &grovecorev1alpha1.ResourceNameReplica{Name: pcsg.Name, Replica: i - 1})
			if !slices.Contains(existingPodGangNames, pcsgReplicaPodGangName) {
				pendingPodGangs = append(pendingPodGangs, pendingPodGang{
					name:         pcsgReplicaPodGangName,
					constituents: getConstituentPCLQsForPCSGPodGang(&pcsg, pgs.Spec.TemplateSpec.Cliques, logger),
					affinityTo:   grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: int(pgsReplica)}, nil),
				})
			}
		}
	}
	return pendingPodGangs, nil
}

func getConstituentPCLQsForPCSGPodGang(pcsg *grovecorev1alpha1.PodCliqueScalingGroup, cliques []*grovecorev1alpha1.PodCliqueTemplateSpec, logger logr.Logger) []constituent {
	constituents := make([]constituent, 0, len(pcsg.Spec.CliqueNames))
	for _, pclqName := range pcsg.Spec.CliqueNames {
		pclqTemplate, ok := lo.Find(cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return pclqTemplateSpec.Name == pclqName
		})
		if !ok {
			logger.Info("[WARN]: PodCliqueScalingGroup references a PodClique that does not exist in the PodGangSet. This should never happen.", "podCliqueName", pclqName)
			continue
		}
		constituents = append(constituents, constituent{
			podCliqueFQN: pclqName,
			replicas:     pclqTemplate.Spec.Replicas,
		})
	}
	return constituents
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

//func (r _resource) GetExistingPodGangs(ctx context.Context, pgsObjMeta metav1.ObjectMeta) ([]groveschedulerv1alpha1.PodGang, error) {
//	podGangList := &groveschedulerv1alpha1.PodGangList{}
//	if err := r.client.List(ctx,
//		podGangList,
//		client.InNamespace(pgsObjMeta.Namespace),
//		client.MatchingLabels(getPodGangSelectorLabels(pgsObjMeta)),
//	); err != nil {
//		return nil, err
//	}
//	return podGangList.Items, nil
//}
