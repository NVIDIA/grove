package validation

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUnknownCliques(t *testing.T) {
	testCases := []struct {
		name              string
		discoveredCliques []string
		cliqueDeps        map[string][]string
		unknownCliques    []string
	}{
		{
			name:              "All clique dependencies have been defined in the PodGangSet",
			discoveredCliques: []string{"c1", "c2", "c3"},
			cliqueDeps: map[string][]string{
				"c2": {"c1"},
				"c3": {"c1"},
			},
			unknownCliques: []string{},
		},
		{
			name:              "Some clique dependencies have not been defined in the PodGangSet",
			discoveredCliques: []string{"c1", "c2", "c3"},
			cliqueDeps: map[string][]string{
				"c2": {"c1"},
				"c3": {"c1", "c4"},
			},
			unknownCliques: []string{"c4"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			depG := NewPodCliqueDependencyGraph()
			for c, deps := range tc.cliqueDeps {
				depG.AddDependencies(c, deps)
			}
			actualUnknownCliques := depG.GetUnknownCliques(tc.discoveredCliques)
			assert.ElementsMatch(t, tc.unknownCliques, actualUnknownCliques)
		})
	}
}

func TestGetStronglyConnectedCliques(t *testing.T) {
	testCases := []struct {
		name                             string
		cliqueDeps                       map[string][]string
		expectedStronglyConnectedCliques [][]string
	}{
		{
			name: "No strongly connected cliques",
			cliqueDeps: map[string][]string{
				"c3": {"c1"},
				"c4": {"c1"},
				"c5": {"c3", "c4"},
				"c2": {},
			},
			expectedStronglyConnectedCliques: [][]string{},
		},
		{
			name: "One cycle of strongly connected cliques",
			cliqueDeps: map[string][]string{
				"c1": {"c2"},
				"c2": {"c3", "c4"},
				"c3": {"c1"},
				"c4": {"c5"},
			},
			expectedStronglyConnectedCliques: [][]string{
				{"c1", "c2", "c3"},
			},
		},
		{
			name: "More than one cycle of strongly connected cliques",
			cliqueDeps: map[string][]string{
				"c1": {"c2"},
				"c2": {"c3"},
				"c3": {"c1", "c4"},
				"c4": {"c5"},
				"c6": {"c5", "c8"},
				"c7": {"c6"},
				"c8": {"c7", "c5"},
				"c9": {"c8", "c7"},
			},
			expectedStronglyConnectedCliques: [][]string{
				{"c1", "c2", "c3"},
				{"c6", "c7", "c8"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			depG := NewPodCliqueDependencyGraph()
			for c, deps := range tc.cliqueDeps {
				depG.AddDependencies(c, deps)
			}
			actualStronglyConnectedCliques := depG.GetStronglyConnectedCliques()
			// sort the expected strongly connected cliques
			for _, expectedScc := range tc.expectedStronglyConnectedCliques {
				slices.Sort(expectedScc)
			}
			// sort the actual strongly connected cliques
			for _, actualScc := range actualStronglyConnectedCliques {
				slices.Sort(actualScc)
			}
			assert.ElementsMatch(t, tc.expectedStronglyConnectedCliques, actualStronglyConnectedCliques)
		})
	}
}
