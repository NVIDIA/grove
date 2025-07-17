// /*
// Copyright 2025 The Grove Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestGetPodCliqueReplicaIndexFromPodCliqueFQN(t *testing.T) {
	testCases := []struct {
		description   string
		pgsName       string
		pclqFQNName   string
		expectedIndex int
		expectedErr   bool
	}{
		{
			description:   "Valid PodClique FQN",
			pgsName:       "inference",
			pclqFQNName:   "inference-0-prefill",
			expectedIndex: 0,
			expectedErr:   false,
		},
		{
			description:   "Valid PodClique FQN with hyphen in template name",
			pgsName:       "inference",
			pclqFQNName:   "inference-1-prefill-leader",
			expectedIndex: 0,
			expectedErr:   false,
		},
		{
			description:   "Valid PodClique FQN with hyphen in PGS name",
			pgsName:       "pgs-inference",
			pclqFQNName:   "pgs-inference-2-prefill-worker",
			expectedIndex: 0,
			expectedErr:   false,
		},
		{
			description: "Invalid PodClique FQN name",
			pgsName:     "inference",
			pclqFQNName: "inference-prefill",
			expectedErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			index, err := GetPodCliqueReplicaIndexFromPodCliqueFQN(tc.pgsName, tc.pclqFQNName)
			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedIndex, index)
			}
		})
	}
}

func TestGetPCSGReplicaIndexFromPCSGFQN(t *testing.T) {
	testCases := []struct {
		description   string
		pgsName       string
		pcsgFQNName   string
		expectedIndex int
		expectedErr   bool
	}{
		{
			description:   "Valid PCSG FQN",
			pgsName:       "inference",
			pcsgFQNName:   "inference-0-workers",
			expectedIndex: 0,
			expectedErr:   false,
		},
		{
			description:   "Valid PCSG FQN with higher replica index",
			pgsName:       "inference",
			pcsgFQNName:   "inference-1-workers",
			expectedIndex: 1,
			expectedErr:   false,
		},
		{
			description:   "Valid PCSG FQN with hyphen in PGS name",
			pgsName:       "pgs-inference",
			pcsgFQNName:   "pgs-inference-2-workers",
			expectedIndex: 2,
			expectedErr:   false,
		},
		{
			description:   "Valid PCSG FQN with hyphen in PCSG name",
			pgsName:       "inference",
			pcsgFQNName:   "inference-0-worker-group",
			expectedIndex: 0,
			expectedErr:   false,
		},
		{
			description: "Invalid PCSG FQN name",
			pgsName:     "inference",
			pcsgFQNName: "inference-workers",
			expectedErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			index, err := GetPCSGReplicaIndexFromPCSGFQN(tc.pgsName, tc.pcsgFQNName)
			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedIndex, index)
			}
		})
	}
}
