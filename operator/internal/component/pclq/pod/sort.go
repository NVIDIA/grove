package pod

import corev1 "k8s.io/api/core/v1"

type DeletionSorter []*corev1.Pod

func (s DeletionSorter) Len() int {
	return len(s)
}

func (s DeletionSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s DeletionSorter) Less(i, j int) bool {
	// TODO implement me
	return false
}
