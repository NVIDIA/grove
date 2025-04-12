package utils

import "fmt"

// GeneratePodGangName generates a PodGang name based on the PodGangSet name and the replica index.
func GeneratePodGangName(pgsName string, replicaIndex int32) string {
	return fmt.Sprintf("%s-%d", pgsName, replicaIndex)
}
