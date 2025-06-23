package podgang

//import (
//	"context"
//	"fmt"
//	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
//	"github.com/NVIDIA/grove/operator/internal/component"
//	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
//	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
//	"github.com/go-logr/logr"
//	"github.com/samber/lo"
//	corev1 "k8s.io/api/core/v1"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"sigs.k8s.io/controller-runtime/pkg/client"
//	"strings"
//)
//
////func (pgi podGangInfo) getPodCliqueFQNs() []string {
////	return lo.Map(pgi.constituents, func(c constituent, _ int) string {
////		return c.podCliqueFQN
////	})
////}
////
////func (pgi podGangInfo) getExpectedPCLQReplicas(pclqFQN string) int {
////	matchingPCLQConstituent, ok := lo.Find(pgi.constituents, func(c constituent) bool {
////		return c.podCliqueFQN == pclqFQN
////	})
////	return lo.Ternary(ok, int(matchingPCLQConstituent.replicas), 0)
////}
////
////func (pgi podGangInfo) allPodsCreated() bool {
////	return lo.Reduce(pgi.constituents, func(agg bool, c constituent, _ int) bool {
////		return agg && (int(c.replicas) == len(c.associatedPodNames))
////	}, true)
////}
////
////func (pgi podGangInfo) getAssociatedPods() []string {
////	allAssociatedPodNames := make([]string, 0, 50)
////	for _, c := range pgi.constituents {
////		allAssociatedPodNames = append(allAssociatedPodNames, c.associatedPodNames...)
////	}
////	return allAssociatedPodNames
////}
////
////// computeExpectedPodGangs computes the expected PodGangs for the PodGangSet.
////// It inspects the defined PodCliqueScalingGroup's and PodGangSet replicas to identify PodGangs.
////func (r _resource) computeExpectedPodGangs(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) ([]podGangInfo, error) {
////	expectedPodGangs := make([]podGangInfo, 0, 50) // preallocate to avoid multiple allocations
////
////	// For each PodGangSet replica there is going to be a PodGang. Add all pending PodGangs that should be created for each replica.
////	expectedPodGangs = append(expectedPodGangs, getExpectedPodGangForPGSReplicas(pgs)...)
////	// For each replica of PodGangSet, get the pending PodGangs for each PodCliqueScalingGroup.
////	for pgsReplica := range pgs.Spec.Replicas {
////		expectedPodGangsForPCSG, err := r.getExpectedPodGangsForPCSG(ctx, logger, pgs, pgsReplica)
////		if err != nil {
////			return nil, err
////		}
////		expectedPodGangs = append(expectedPodGangs, expectedPodGangsForPCSG...)
////	}
////
////	return expectedPodGangs, nil
////}
//
//// createPendingPodGangs checks if expected PodGangs are ready to be created. For each such PodGang this function does the folowing:
//// 1. It identifies unassigned Pods across PodCliques that make this PodGang and labels these Pods with the PodGang name.
//// 2. It creates the PodGang.
//func (r _resource) createPendingPodGangs(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pendingPodGangs []podGangInfo) ([]string, error) {
//	pgsObjectKey := client.ObjectKeyFromObject(pgs)
//	skippedPodGangNames := make([]string, 0, len(pendingPodGangs))
//	podsByPCLQ, err := r.getPodsByPCLQForPGS(ctx, pgsObjectKey)
//	if err != nil {
//		return nil, err
//	}
//	for _, pendingPodGang := range pendingPodGangs {
//		podGroups := make([]groveschedulerv1alpha1.PodGroup, 0, len(pendingPodGang.constituents))
//		pclqs, err := r.getPCLQsForPodGang(ctx, pgsObjectKey.Namespace, pendingPodGang.name, pendingPodGang.getPodCliqueFQNs())
//		if err != nil {
//			return nil, err
//		}
//		for _, pclq := range pclqs {
//			pods, ok := podsByPCLQ[pclq.Name]
//
//			matchingPCLQTemplateSpec, ok := lo.Find(pgs.Spec.TemplateSpec.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
//				return strings.HasSuffix(pclq.Name, pclqTemplateSpec.Name)
//			})
//			if !ok {
//				logger.Info("[WARN] Found a PCLQ that is not defined in PGS spec. This should ideally never happen.", "pclqName", pclq.Name)
//				continue
//			}
//			allAssociatedPodNames := make([]string, 0, 50)
//			pclqObjectKey := client.ObjectKeyFromObject(pclq)
//			// check if PodClique reconciler has created all expected Pods, this will be reflected in PodClique.Status.ReadyReplicas
//			if pclq.Spec.Replicas == pclq.Status.ReadyReplicas {
//				logger.Info("PodClique has created all desired replicas", "pclqObjectKey", pclqObjectKey)
//				associatedPodNames, unassignedPods := segregatePCLQPods(pendingPodGang.name, pclq, pods)
//				numPendingPodsToAssociate := pendingPodGang.getExpectedPCLQReplicas(pclq.Name) - len(associatedPodNames)
//				if numPendingPodsToAssociate > 0 {
//					if err := r.assignPodsToPodGang(ctx, pendingPodGang.name, unassignedPods, numPendingPodsToAssociate); err != nil {
//						return nil, err
//					}
//				}
//				allAssociatedPodNames = append(allAssociatedPodNames, associatedPodNames...)
//				allAssociatedPodNames = append(allAssociatedPodNames, lo.Map(unassignedPods, func(pod corev1.Pod, _ int) string {
//					return pod.Name
//				})...)
//				podGroups = append(podGroups, createPodGroup(pclq.Namespace, allAssociatedPodNames, *matchingPCLQTemplateSpec.Spec.MinReplicas))
//			} else {
//				logger.Info("PodClique has not created all desired replicas yet, skipping creation of PodGang", "podGang", pendingPodGang.name, "pclqObjectKey", pclqObjectKey, "desiredReplicas", pclq.Spec.Replicas, "readyReplicas", pclq.Status.ReadyReplicas)
//				skippedPodGangNames = append(skippedPodGangNames, pendingPodGang.name)
//			}
//		}
//		//if err := r.createPodGang(ctx, pgs, pendingPodGang.name, podGroups); err != nil {
//		//	return groveerr.WrapError(err,
//		//	)
//		//}
//	}
//	return skippedPodGangNames, nil
//}
//
////func getExpectedPodGangForPGSReplicas(pgs *grovecorev1alpha1.PodGangSet) []podGangInfo {
////	expectedPodGangs := make([]podGangInfo, 0, int(pgs.Spec.Replicas))
////	for pgsReplica := range pgs.Spec.Replicas {
////		replicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: int(pgsReplica)}, nil)
////		expectedPodGangs = append(expectedPodGangs, podGangInfo{
////			name:         replicaPodGangName,
////			constituents: getConstituentPCLQsForPGSReplicaPodGang(pgs.Name, pgsReplica, pgs.Spec.TemplateSpec.Cliques),
////		})
////	}
////	return expectedPodGangs
////}
//
////func (r _resource) getExpectedPodGangsForPCSG(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pgsReplica int32) ([]podGangInfo, error) {
////	if len(pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs) == 0 {
////		return []podGangInfo{}, nil
////	}
////	existingPCSGs, err := r.getExistingPodCliqueScalingGroups(ctx, pgs.Name, pgsReplica)
////	if err != nil {
////		return nil, err
////	}
////	expectedPodGangs := make([]podGangInfo, 0, 50) // preallocate to avoid multiple allocations
////	for _, pcsg := range existingPCSGs {
////		if pcsg.Spec.Replicas == 1 {
////			continue // First replica of PodCliqueScalingGroup is the first PodGang created for the PodGangSet replica, so it is already accounted for.
////		}
////		for i := 1; i < int(pcsg.Spec.Replicas); i++ {
////			pcsgReplicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: int(pgsReplica)}, &grovecorev1alpha1.ResourceNameReplica{Name: pcsg.Name, Replica: i - 1})
////			expectedPodGangs = append(expectedPodGangs, podGangInfo{
////				name:         pcsgReplicaPodGangName,
////				constituents: identifyConstituentPCLQsForPCSGPodGang(&pcsg, pgs.Spec.TemplateSpec.Cliques, logger),
////			})
////		}
////	}
////	return expectedPodGangs, nil
////}
//
////func getConstituentPCLQsForPGSReplicaPodGang(pgsName string, pgsReplica int32, cliques []*grovecorev1alpha1.PodCliqueTemplateSpec) []constituent {
////	constituents := make([]constituent, 0, len(cliques))
////	for _, pclq := range cliques {
////		constituents = append(constituents, constituent{
////			podCliqueFQN: grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: int(pgsReplica)}, pclq.Name),
////			replicas:     pclq.Spec.Replicas,
////		})
////	}
////	return constituents
////}
//
////func (r _resource) createPodGang(ctx context.Context, pgs *grovecorev1alpha1.PodGangSet, podGangName string, podGroups []podGangInfo) error {
////	podGang := groveschedulerv1alpha1.PodGang{
////		ObjectMeta: metav1.ObjectMeta{
////			Name:      podGangName,
////			Namespace: pgs.Namespace,
////			Labels:    getLabels(),
////		},
////	}
////	metav1.
////}
//
//func createPodGroup(namespace string, podNames []string, minReplicas int32) groveschedulerv1alpha1.PodGroup {
//	podReferences := lo.Map(podNames, func(podName string, _ int) groveschedulerv1alpha1.NamespacedName {
//		return groveschedulerv1alpha1.NamespacedName{
//			Name:      podName,
//			Namespace: namespace,
//		}
//	})
//	return groveschedulerv1alpha1.PodGroup{
//		PodReferences: podReferences,
//		MinReplicas:   minReplicas,
//	}
//}
//
//// segregatePCLQPods creates two slices. One slice is Pods that have already been labeled with the passed in podGangName.
//// The second slice is of Pods that do not have this label set yet. This needs to be done as it is possible that in the previous run
//// only part of the pods could be labeled. It is required that all pods that should be associated to this PodGang be labeled before
//// creating the PodGang resource.
//func segregatePCLQPods(podGangName string, pclq *grovecorev1alpha1.PodClique, pgsPods []corev1.Pod) (assignedPodNames []string, unassignedPods []corev1.Pod) {
//	for _, pod := range pgsPods {
//		if !metav1.IsControlledBy(&pod, pclq) {
//			continue
//		}
//		if metav1.HasLabel(pod.ObjectMeta, grovecorev1alpha1.LabelPodGangName) {
//			if pod.GetLabels()[grovecorev1alpha1.LabelPodGangName] == podGangName {
//				assignedPodNames = append(assignedPodNames, pod.Name)
//			}
//		} else {
//			unassignedPods = append(unassignedPods, pod)
//		}
//	}
//	return
//}
//
//func (r _resource) assignPodsToPodGang(ctx context.Context, podGangName string, unassignedPods []corev1.Pod, pendingPodsToAssociate int) error {
//	maxCount := min(len(unassignedPods), pendingPodsToAssociate)
//	for _, pod := range unassignedPods[0:maxCount] {
//		podClone := pod.DeepCopy()
//		pod.Labels[grovecorev1alpha1.LabelPodGangName] = podGangName
//		if err := r.client.Patch(ctx, &pod, client.MergeFrom(podClone)); err != nil {
//			return groveerr.WrapError(err,
//				errCodePatchPodLabel,
//				component.OperationSync,
//				fmt.Sprintf("failed to patch pod %v with pod gang label: [%s:%s]", client.ObjectKeyFromObject(&pod), grovecorev1alpha1.LabelPodGangName, podGangName),
//			)
//		}
//	}
//	return nil
//}
//
//func (r _resource) getPCLQsForPodGang(ctx context.Context, namespace, podGangName string, pclqFQNs []string) ([]*grovecorev1alpha1.PodClique, error) {
//	pclqs := make([]*grovecorev1alpha1.PodClique, 0, len(pclqFQNs))
//	for _, pclqFQN := range pclqFQNs {
//		pclq := &grovecorev1alpha1.PodClique{}
//		pclqObjKey := client.ObjectKey{Namespace: namespace, Name: pclqFQN}
//		if err := r.client.Get(ctx, pclqObjKey, pclq); err != nil {
//			return nil, groveerr.WrapError(err,
//				errCodeGetPodClique,
//				component.OperationGetExistingResourceNames,
//				fmt.Sprintf("Failed to get PodClique %v for PodGang %v", pclqObjKey, client.ObjectKey{Namespace: namespace, Name: podGangName}),
//			)
//		}
//		pclqs = append(pclqs, pclq)
//	}
//	return pclqs, nil
//}
