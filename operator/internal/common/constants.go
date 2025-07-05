package common

const (
	// PodNamespaceFileName is the name of the file that contains the namespace in which the pod is running
	PodNamespaceFileName = "namespace"
	// VolumeMountPathPodInfo contains the file path at which the downward API volume is attached
	VolumeMountPathPodInfo = "/var/grove/pod-info"
)
