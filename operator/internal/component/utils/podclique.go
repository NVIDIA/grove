package utils

import (
	"context"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"slices"
	"time"
)

// GetPodCliquesByOwner retrieves PodClique objects that are owned by the specified owner kind and object key, and match the provided selector labels.
func GetPodCliquesByOwner(ctx context.Context, cl client.Client, ownerKind string, ownerObjectKey client.ObjectKey, selectorLabels map[string]string) ([]grovecorev1alpha1.PodClique, error) {
	podCliqueList := &grovecorev1alpha1.PodCliqueList{}
	if err := cl.List(ctx,
		podCliqueList,
		client.InNamespace(ownerObjectKey.Namespace),
		client.MatchingLabels(selectorLabels)); err != nil {
		return nil, err
	}

	filteredPCLQs := lo.Filter(podCliqueList.Items, func(pclq grovecorev1alpha1.PodClique, _ int) bool {
		if len(pclq.OwnerReferences) == 0 {
			return false
		}
		return pclq.OwnerReferences[0].Kind == ownerKind && pclq.OwnerReferences[0].Name == ownerObjectKey.Name
	})
	return filteredPCLQs, nil
}

// GroupPCLQsByPodGangName filters PCLQs that have a PodGang label and groups them by the PodGang name.
func GroupPCLQsByPodGangName(pclqs []grovecorev1alpha1.PodClique) map[string][]grovecorev1alpha1.PodClique {
	podGangPCLQs := make(map[string][]grovecorev1alpha1.PodClique, len(pclqs))
	for _, pclq := range pclqs {
		podGangName, ok := pclq.GetLabels()[grovecorev1alpha1.LabelPodGangName]
		if !ok {
			continue
		}
		podGangPCLQs[podGangName] = append(podGangPCLQs[podGangName], pclq)
	}
	return podGangPCLQs
}

// GetMinAvailableBreachedPCLQInfo filters PodCliques that have grovecorev1alpha1.ConditionTypeMinAvailableBreached set to true.
// For each such PodClique it returns the name of the PodClique a duration to wait for before terminationDelay is breached.
func GetMinAvailableBreachedPCLQInfo(pclqs []grovecorev1alpha1.PodClique, terminationDelay time.Duration, since time.Time) ([]string, time.Duration) {
	pclqCandidateNames := make([]string, 0, len(pclqs))
	waitForDurations := make([]time.Duration, 0, len(pclqs))
	for _, pclq := range pclqs {
		cond := meta.FindStatusCondition(pclq.Status.Conditions, grovecorev1alpha1.ConditionTypeMinAvailableBreached)
		if cond == nil {
			continue
		}
		if cond.Status == metav1.ConditionTrue {
			pclqCandidateNames = append(pclqCandidateNames, pclq.Name)
			waitFor := terminationDelay - since.Sub(cond.LastTransitionTime.Time)
			waitForDurations = append(waitForDurations, waitFor)
		}
	}
	slices.Sort(waitForDurations)
	if len(waitForDurations) == 0 {
		return pclqCandidateNames, 0
	}
	return pclqCandidateNames, waitForDurations[0]
}

func FilterPCLQsWithMinAvailableBreached(pclqs []grovecorev1alpha1.PodClique) []grovecorev1alpha1.PodClique {
	return lo.Filter(pclqs, func(pclq grovecorev1alpha1.PodClique, _ int) bool {
		cond := meta.FindStatusCondition(pclq.Status.Conditions, grovecorev1alpha1.ConditionTypeMinAvailableBreached)
		return cond != nil && cond.Status == metav1.ConditionTrue
	})
}

func GetPCLQNames(pclqs []grovecorev1alpha1.PodClique) []string {
	return lo.Map(pclqs, func(pclq grovecorev1alpha1.PodClique, _ int) string {
		return pclq.Name
	})
}
