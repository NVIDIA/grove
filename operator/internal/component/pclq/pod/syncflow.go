package pod

import (
	"context"
	"fmt"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"slices"
	"strconv"
	"strings"
)

type syncContext struct {
	ctx                    context.Context
	pgs                    *grovecorev1alpha1.PodGangSet
	pclq                   *grovecorev1alpha1.PodClique
	existingPodGangNames   []string
	expectedPodGangNames   []string
	expectedPodsPerPodGang int
	numExistingPods        int
	existingPodGangToPods  map[string][]*corev1.Pod
	unassignedPods         []*corev1.Pod
	orphanedPods           map[string][]*corev1.Pod
}

func (r _resource) prepareSyncFlow(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) (*syncContext, error) {
	sc := &syncContext{ctx: ctx}
	pgs, err := r.getOwnerPodGangSet(ctx, pclq.ObjectMeta)
	if err != nil {
		return nil, err
	}
	sc.pgs = pgs

	existingPodGangNames, err := r.getExistingPodGangNamesAssociatedWithPCLQ(ctx, pgs, pclq.ObjectMeta)
	if err != nil {
		return nil, err
	}
	sc.existingPodGangNames = existingPodGangNames

	expectedPodGangNames, err := r.getExpectedPodGangNamesAssociatedWithPCLQ(ctx, pgs.Name, pclq.ObjectMeta)
	if err != nil {
		return nil, err
	}
	sc.expectedPodGangNames = expectedPodGangNames

	pclqPods, err := componentutils.GetPCLQPods(ctx, r.client, pgs.Name, pclq)
	if err != nil {
		logger.Error(err, "Failed to list pods that belong to PodClique %v", client.ObjectKeyFromObject(pclq))
		return nil, groveerr.WrapError(err,
			errCodeListPod,
			component.OperationSync,
			fmt.Sprintf("failed to list pods that belong to the PodClique %v", client.ObjectKeyFromObject(pclq)),
		)
	}
	sc.numExistingPods = len(pclqPods)
	sc.existingPodGangToPods, sc.unassignedPods, sc.orphanedPods = segregateExistingPodsByPodGang(existingPodGangNames, expectedPodGangNames, pclqPods)
	sc.expectedPodsPerPodGang = computeExpectedPodGangReplicas(pgs, pclq)

	return sc, nil
}

func (r _resource) runSyncFlow(sc *syncContext, logger logr.Logger) error {
	diff := sc.numExistingPods - int(sc.pclq.Spec.Replicas)
	if diff < 0 {
		r.syncExistingPodGangs()
	}
}

// getOwnerPodGangSet gets the owner PodGangSet object for the PodClique.
func (r _resource) getOwnerPodGangSet(ctx context.Context, pclqObjectMeta metav1.ObjectMeta) (*grovecorev1alpha1.PodGangSet, error) {
	pgsName := k8sutils.GetFirstOwnerName(pclqObjectMeta)
	pgs := &grovecorev1alpha1.PodGangSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgsName,
			Namespace: pclqObjectMeta.Namespace,
		},
	}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(pgs), pgs); err != nil {
		return nil, groveerr.WrapError(err,
			errCodeGetPodGangSet,
			component.OperationSync,
			fmt.Sprintf("failed to get owner PodGangSet %s ", pgsName),
		)
	}
	return pgs, nil
}

func (r _resource) getExpectedPodGangNamesAssociatedWithPCLQ(ctx context.Context, pgsName string, pclqObjectMeta metav1.ObjectMeta) ([]string, error) {
	var expectedPodGangNames []string
	pgsReplica, err := getPGSReplicaIndexForPCLQ(pclqObjectMeta)
	if err != nil {
		return nil, err
	}
	pgsReplicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: pgsReplica}, nil)
	expectedPodGangNames = append(expectedPodGangNames, pgsReplicaPodGangName)

	// check if the PCLQ is associated to a PCSG
	pcsgName, ok := pclqObjectMeta.GetLabels()[grovecorev1alpha1.LabelPodCliqueScalingGroup]
	if !ok {
		return expectedPodGangNames, nil
	}

	pcsgReplicas, err := r.getPCSGReplicasAssociatedWithPCLQ(ctx, pcsgName, pclqObjectMeta.Namespace)
	if err != nil {
		return nil, err
	}

	if pcsgReplicas > 1 {
		for replica := 2; replica <= int(pcsgReplicas); replica++ {
			pcsgReplicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: pgsReplica}, &grovecorev1alpha1.ResourceNameReplica{Name: pcsgName, Replica: replica})
			expectedPodGangNames = append(expectedPodGangNames, pcsgReplicaPodGangName)
		}
	}
	return expectedPodGangNames, nil
}

func (r _resource) getPCSGReplicasAssociatedWithPCLQ(ctx context.Context, pcsgName, namespace string) (int32, error) {
	// Get the PCSG resource and check its current spec.replicas.
	pcsg := &grovecorev1alpha1.PodCliqueScalingGroup{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: pcsgName, Namespace: namespace}, pcsg); err != nil {
		return 0, groveerr.WrapError(err,
			errCodeGetPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("failed to get PodCliqueScalingGroup %s associated to PodClique", client.ObjectKey{Namespace: namespace, Name: pcsgName}),
		)
	}
	return pcsg.Spec.Replicas, nil
}

// getExistingPodGangNamesAssociatedWithPCLQ gets the existing PodGangs that correspond to the PodClique.
func (r _resource) getExistingPodGangNamesAssociatedWithPCLQ(ctx context.Context, pgs *grovecorev1alpha1.PodGangSet, pclqObjectMeta metav1.ObjectMeta) ([]string, error) {
	pgsReplica, err := getPGSReplicaIndexForPCLQ(pclqObjectMeta)
	if err != nil {
		return nil, err
	}
	pgsReplicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplica}, nil)
	pcsgName, associatedToPCSG := pclqObjectMeta.GetLabels()[grovecorev1alpha1.LabelPodCliqueScalingGroup]

	existingPGSPodGangNames, err := componentutils.GetExistingPodGangNames(ctx, r.client, pgs)
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodGang,
			component.OperationSync,
			fmt.Sprintf("failed to list existing PodGangs for PodGangSet %v", client.ObjectKeyFromObject(pgs)),
		)
	}

	existingPodGangsAssociatedWithPCLQ := lo.Filter(existingPGSPodGangNames, func(podGangName string, _ int) bool {
		return podGangName == pgsReplicaPodGangName || (associatedToPCSG && strings.HasPrefix(podGangName, pcsgName))
	})

	return existingPodGangsAssociatedWithPCLQ, nil
}

func computeExpectedPodGangReplicas(pgs *grovecorev1alpha1.PodGangSet, pclq *grovecorev1alpha1.PodClique) int {
	if metav1.HasLabel(pclq.ObjectMeta, grovecorev1alpha1.LabelPodCliqueScalingGroup) {
		cliqueTemplateSpec, _ := lo.Find(pgs.Spec.TemplateSpec.Cliques, func(cliqueTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return strings.Contains(pclq.Name, cliqueTemplateSpec.Name)
		})
		return int(cliqueTemplateSpec.Spec.Replicas)
	}
	return int(pclq.Spec.Replicas)
}

func segregateExistingPodsByPodGang(existingPodGangNames []string, expectedPodGangNames []string, pods []*corev1.Pod) (existingPodGangToPods map[string][]*corev1.Pod, unassignedPods []*corev1.Pod, orphanedPods map[string][]*corev1.Pod) {
	for _, pod := range pods {
		podGangName, ok := pod.GetLabels()[grovecorev1alpha1.LabelPodGangName]
		if !ok {
			unassignedPods = append(unassignedPods, pod)
		}
		if slices.Contains(existingPodGangNames, podGangName) {
			existingPodGangToPods[podGangName] = append(existingPodGangToPods[podGangName], pod)
		} else if !slices.Contains(expectedPodGangNames, podGangName) {
			orphanedPods[podGangName] = append(orphanedPods[podGangName], pod)
		}
	}
	return existingPodGangToPods, unassignedPods, orphanedPods
}

func getPGSReplicaIndexForPCLQ(pclqObjectMeta metav1.ObjectMeta) (int, error) {
	pgsReplicaLabelValue, ok := pclqObjectMeta.GetLabels()[grovecorev1alpha1.LabelPodGangSetReplicaIndex]
	if !ok {
		return 0, groveerr.New(
			errCodeMissingPodGangSetReplicaIndexLabel,
			component.OperationSync,
			fmt.Sprintf("PodClique %v is missing a required label :%s. This should ideally not happen.", k8sutils.GetObjectKeyFromObjectMeta(pclqObjectMeta), grovecorev1alpha1.LabelPodGangSetReplicaIndex))
	}
	pgsReplica, err := strconv.Atoi(pgsReplicaLabelValue)
	if err != nil {
		return 0, groveerr.WrapError(err,
			errCodeInvalidPodGangSetReplicaLabelValue,
			component.OperationSync,
			fmt.Sprintf("failed to convert label value %v to int for PodClique %v", pgsReplicaLabelValue, k8sutils.GetObjectKeyFromObjectMeta(pclqObjectMeta)),
		)
	}
	return pgsReplica, nil
}

func (r _resource) syncExistingPodGangs(sc *syncContext, logger logr.Logger) error {
	for existingPodGang, existingPods := range sc.existingPodGangToPods {
		// check if these existing existingPods are schedule gated. If yes, then remove the schedule gates as the PodGang now exists.
		if err := r.removeSchedulingGatesFromPods(sc.ctx, logger, client.ObjectKeyFromObject(sc.pclq), existingPods); err != nil {
			return err
		}
		diff := len(existingPods) - sc.expectedPodsPerPodGang
		if diff < 0 {

		} else {
			// TODO: code the logic to select pods to delete first.
		}
	}

	if runResult := utils.RunConcurrentlyWithSlowStart(sc.ctx, logger, 1, syncTasks); runResult.HasErrors() {

	}
}

func (r _resource) removeSchedulingGatesFromPods(ctx context.Context, logger logr.Logger, pclqObjectKey client.ObjectKey, pods []*corev1.Pod) error {
	tasks := make([]utils.Task, 0, len(pods))
	for _, pod := range pods {
		if hasPodGangSchedulingGate(pod) {
			task := utils.Task{
				Name: fmt.Sprintf("RemoveSchedulingGate-%s", pod.Name),
				Fn: func(ctx context.Context) error {
					podClone := pod.DeepCopy()
					pod.Spec.SchedulingGates = nil
					if err := client.IgnoreNotFound(r.client.Patch(ctx, pod, client.MergeFrom(podClone))); err != nil {
						return err
					}
					return nil
				},
			}
			tasks = append(tasks, task)
		}
	}
	if len(tasks) == 0 {
		return nil
	}
	if runResult := utils.RunConcurrentlyWithSlowStart(ctx, logger, 1, tasks); runResult.HasErrors() {
		err := runResult.GetAggregatedError()
		logger.Error(err, "failed to remove scheduling gates from pods for PCLQ", "pclqObjectKey", pclqObjectKey, "runSummary", runResult.GetSummary())
		return groveerr.WrapError(err,
			errCodeRemovePodSchedulingGate,
			component.OperationSync,
			fmt.Sprintf("failed to remove scheduling gates from Pods for PodClique %v", pclqObjectKey),
		)
	}
	return nil
}

func (r _resource) getPodCreationTasks(logger logr.Logger, pclq *grovecorev1alpha1.PodClique, podGangName *string, numPods int) []utils.Task {
	createTasks := make([]utils.Task, 0, numPods)
	for i := range numPods {
		createTask := utils.Task{
			Name: fmt.Sprintf("CreatePod-%s-%d", pclq.Name, i),
			Fn: func(ctx context.Context) error {
				pod := &corev1.Pod{}
				if err := r.buildResource(pclq, pod, podGangName); err != nil {
					return groveerr.WrapError(err,
						errCodeSyncPod,
						component.OperationSync,
						fmt.Sprintf("failed to build Pod resource for PodClique %v", client.ObjectKeyFromObject(pclq)),
					)
				}
				if err := r.client.Create(ctx, pod); err != nil {
					r.eventRecorder.Eventf(pclq, corev1.EventTypeWarning, reasonPodCreationFailed, "Error creating pod: %v", err)
					return groveerr.WrapError(err,
						errCodeSyncPod,
						component.OperationSync,
						fmt.Sprintf("failed to create Pod: %v for PodClique %v", client.ObjectKeyFromObject(pod), client.ObjectKeyFromObject(pclq)),
					)
				}
				logger.Info("Created pod for PodClique", "pclqName", pclq.Name, "podName", pod.Name)
				r.eventRecorder.Eventf(pclq, corev1.EventTypeNormal, reasonPodCreationSuccessful, "Created Pod: %s", pod.Name)
				return nil
			},
		}
		createTasks = append(createTasks, createTask)
	}
	return createTasks
}

func hasPodGangSchedulingGate(pod *corev1.Pod) bool {
	return slices.ContainsFunc(pod.Spec.SchedulingGates, func(schedulingGate corev1.PodSchedulingGate) bool {
		return podGangSchedulingGate == schedulingGate.Name
	})
}
