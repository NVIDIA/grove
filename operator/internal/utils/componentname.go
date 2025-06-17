package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// GetPodGangSetReplicaIndexFromPodCliqueFQN extracts the PodGangSet replica index from a Pod Clique FQN name.
func GetPodGangSetReplicaIndexFromPodCliqueFQN(pgsName, pclqFQNName string) (int, error) {
	replicaStartIndex := len(pgsName) + 1 // +1 for the hyphen
	hyphenIndex := strings.Index(pclqFQNName[replicaStartIndex:], "-")
	if hyphenIndex == -1 {
		return -1, fmt.Errorf("PodClique FQN is not in the expected format of <pgs-name>-<pgs-replica-index>-<pclq-template-name>: %s", pclqFQNName)
	}
	replicaEndIndex := replicaStartIndex + hyphenIndex
	return strconv.Atoi(pclqFQNName[replicaStartIndex:replicaEndIndex])
}
