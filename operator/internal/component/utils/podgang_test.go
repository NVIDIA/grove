package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseScalingGroupInfo(t *testing.T) {
	tests := []struct {
		name       string
		pgsName    string
		pgsReplica int
		pclqName   string
		expected   *scalingGroupInfo
		expectNil  bool
	}{
		{
			name:       "scaling group PodClique with k8s suffix",
			pgsName:    "simple1",
			pgsReplica: 0,
			pclqName:   "simple1-0-sga-0-pcb-sqj25",
			expected: &scalingGroupInfo{
				scalingGroupName: "sga",
				sgReplicaIndex:   0,
				cliqueName:       "pcb",
			},
		},
		{
			name:       "scaling group PodClique with different replica",
			pgsName:    "simple1",
			pgsReplica: 0,
			pclqName:   "simple1-0-sga-2-pcc-abc123",
			expected: &scalingGroupInfo{
				scalingGroupName: "sga",
				sgReplicaIndex:   2,
				cliqueName:       "pcc",
			},
		},
		{
			name:       "standalone PodClique (not scaling group)",
			pgsName:    "simple1",
			pgsReplica: 0,
			pclqName:   "simple1-0-standalone-abc123",
			expectNil:  true,
		},
		{
			name:       "wrong PGS name",
			pgsName:    "simple1",
			pgsReplica: 0,
			pclqName:   "different-0-sga-0-pcb-abc123",
			expectNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseScalingGroupInfo(tt.pgsName, tt.pgsReplica, tt.pclqName)

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.scalingGroupName, result.scalingGroupName)
				assert.Equal(t, tt.expected.sgReplicaIndex, result.sgReplicaIndex)
				assert.Equal(t, tt.expected.cliqueName, result.cliqueName)
			}
		})
	}
}

func TestCreatePodGangNameForPCSGFromFQN(t *testing.T) {
	tests := []struct {
		name             string
		pcsgFQN          string
		pcsgReplicaIndex int
		expected         string
	}{
		{
			name:             "individual PodGang name from FQN",
			pcsgFQN:          "simple1-0-sga",
			pcsgReplicaIndex: 1,
			expected:         "simple1-0-sga-1",
		},
		{
			name:             "individual PodGang name from FQN with different replica",
			pcsgFQN:          "simple1-0-sga",
			pcsgReplicaIndex: 2,
			expected:         "simple1-0-sga-2",
		},
		{
			name:             "complex scaling group name",
			pcsgFQN:          "test-2-complex-sg",
			pcsgReplicaIndex: 0,
			expected:         "test-2-complex-sg-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreatePodGangNameForPCSGFromFQN(tt.pcsgFQN, tt.pcsgReplicaIndex)
			assert.Equal(t, tt.expected, result)
		})
	}
}
