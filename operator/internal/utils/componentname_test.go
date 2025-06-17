package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetPodGangSetReplicaIndexFromPodCliqueFQN(t *testing.T) {
	testCases := []struct {
		description   string
		pgsName       string
		pclqFQNName   string
		expectedIndex int
		expectedErr   bool
	}{
		{
			description:   "PodClique and PGS name without hyphen",
			pgsName:       "inference",
			pclqFQNName:   "inference-0-prefill",
			expectedIndex: 0,
			expectedErr:   false,
		},
		{
			description:   "PodClique name with hyphen and PGS name without hyphen",
			pgsName:       "inference",
			pclqFQNName:   "inference-1-prefill-leader",
			expectedIndex: 1,
			expectedErr:   false,
		},
		{
			description:   "PodClique name with hyphen and PGS name with hyphen",
			pgsName:       "pgs-inference",
			pclqFQNName:   "pgs-inference-2-prefill-worker",
			expectedIndex: 2,
			expectedErr:   false,
		},
		{
			description: "Malformed PodClique FQN name",
			pgsName:     "inference",
			pclqFQNName: "inference-prefill",
			expectedErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			index, err := GetPodGangSetReplicaIndexFromPodCliqueFQN(tc.pgsName, tc.pclqFQNName)
			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedIndex, index)
			}
		})
	}
}
