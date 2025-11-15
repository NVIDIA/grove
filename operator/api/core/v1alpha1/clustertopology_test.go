/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"testing"
)

func TestTopologyDomainComparison(t *testing.T) {
	tests := []struct {
		name                 string
		domain               TopologyDomain
		other                TopologyDomain
		expectedCompare      int
		expectedBroaderThan  bool
		expectedNarrowerThan bool
	}{
		{
			name:                 "region vs zone",
			domain:               TopologyDomainRegion,
			other:                TopologyDomainZone,
			expectedCompare:      -1,
			expectedBroaderThan:  true,
			expectedNarrowerThan: false,
		},
		{
			name:                 "zone vs region",
			domain:               TopologyDomainZone,
			other:                TopologyDomainRegion,
			expectedCompare:      1,
			expectedBroaderThan:  false,
			expectedNarrowerThan: true,
		},
		{
			name:                 "region vs region (equal)",
			domain:               TopologyDomainRegion,
			other:                TopologyDomainRegion,
			expectedCompare:      0,
			expectedBroaderThan:  false,
			expectedNarrowerThan: false,
		},
		{
			name:                 "host vs numa",
			domain:               TopologyDomainHost,
			other:                TopologyDomainNuma,
			expectedCompare:      -1,
			expectedBroaderThan:  true,
			expectedNarrowerThan: false,
		},
		{
			name:                 "region vs numa (multiple levels apart)",
			domain:               TopologyDomainRegion,
			other:                TopologyDomainNuma,
			expectedCompare:      -6,
			expectedBroaderThan:  true,
			expectedNarrowerThan: false,
		},
		{
			name:                 "numa vs region (multiple levels apart)",
			domain:               TopologyDomainNuma,
			other:                TopologyDomainRegion,
			expectedCompare:      6,
			expectedBroaderThan:  false,
			expectedNarrowerThan: true,
		},
		{
			name:                 "datacenter vs rack",
			domain:               TopologyDomainDataCenter,
			other:                TopologyDomainRack,
			expectedCompare:      -2,
			expectedBroaderThan:  true,
			expectedNarrowerThan: false,
		},
		{
			name:                 "rack vs datacenter",
			domain:               TopologyDomainRack,
			other:                TopologyDomainDataCenter,
			expectedCompare:      2,
			expectedBroaderThan:  false,
			expectedNarrowerThan: true,
		},
		{
			name:                 "rack vs zone (narrower vs broader)",
			domain:               TopologyDomainRack,
			other:                TopologyDomainZone,
			expectedCompare:      3,
			expectedBroaderThan:  false,
			expectedNarrowerThan: true,
		},
		{
			name:                 "zone vs rack (broader vs narrower)",
			domain:               TopologyDomainZone,
			other:                TopologyDomainRack,
			expectedCompare:      -3,
			expectedBroaderThan:  true,
			expectedNarrowerThan: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Compare()
			gotCompare := tt.domain.Compare(tt.other)
			if gotCompare != tt.expectedCompare {
				t.Errorf("Compare() = %v, want %v", gotCompare, tt.expectedCompare)
			}

			// Test BroaderThan()
			gotBroaderThan := tt.domain.BroaderThan(tt.other)
			if gotBroaderThan != tt.expectedBroaderThan {
				t.Errorf("BroaderThan() = %v, want %v", gotBroaderThan, tt.expectedBroaderThan)
			}

			// Test NarrowerThan()
			gotNarrowerThan := tt.domain.NarrowerThan(tt.other)
			if gotNarrowerThan != tt.expectedNarrowerThan {
				t.Errorf("NarrowerThan() = %v, want %v", gotNarrowerThan, tt.expectedNarrowerThan)
			}
		})
	}
}
