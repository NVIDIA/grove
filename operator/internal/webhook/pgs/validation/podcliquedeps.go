package validation

import (
	"k8s.io/utils/strings/slices"
)

type PodCliqueDependencyGraph struct {
	adjacency map[string][]string
}

func NewPodCliqueDependencyGraph() *PodCliqueDependencyGraph {
	return &PodCliqueDependencyGraph{
		adjacency: make(map[string][]string),
	}
}

func (g *PodCliqueDependencyGraph) AddDependencies(from string, to []string) {
	for _, t := range to {
		g.adjacency[from] = append(g.adjacency[from], t)
	}
}

func (g *PodCliqueDependencyGraph) GetUnknownCliques(discoveredCliques []string) []string {
	unknownCliques := make([]string, 0, len(discoveredCliques))
	for _, toCliques := range g.adjacency {
		for _, toClique := range toCliques {
			if !slices.Contains(discoveredCliques, toClique) {
				unknownCliques = append(unknownCliques, toClique)
			}
		}
	}
	return nil
}

// GetStronglyConnectedCliques returns all strongly connected cliques in the graph representing the PodClique dependencies.
// A strongly connected node is a subgraph in which every node is reachable from every other node.
// Uses Tarjan's algorithm as it efficiently uses DFS to find all strongly connected components a.k.a cycles in a graph.
// See https://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm
// NOTE: For the use case of this operator, we do not consider sub-graphs with single node as strongly connected components.
func (g *PodCliqueDependencyGraph) GetStronglyConnectedCliques() [][]string {
	numNodes := len(g.adjacency)
	index := 0
	indices := make(map[string]int, numNodes)
	lowLink := make(map[string]int, numNodes)
	onStack := make(map[string]bool, numNodes)
	stack := make([]string, 0, numNodes)
	var sccs [][]string

	var strongConnect func(c string)
	strongConnect = func(c string) {
		indices[c] = index
		lowLink[c] = index
		index++
		stack = append(stack, c)
		onStack[c] = true
		// iterate over the adjacency list of the current clique
		for _, d := range g.adjacency[c] {
			if _, found := indices[d]; !found {
				strongConnect(d)
				lowLink[c] = min(lowLink[c], lowLink[d])
			} else if onStack[d] {
				lowLink[c] = min(lowLink[c], indices[d])
			}
		}

		if lowLink[c] == indices[c] {
			var scc []string
			var sccElem string
			for {
				sccElem, stack = stack[len(stack)-1], stack[:len(stack)-1]
				onStack[sccElem] = false
				scc = append(scc, sccElem)
				if sccElem == c {
					break
				}
			}
			if len(scc) > 1 {
				sccs = append(sccs, scc)
			}
		}
	}

	for c := range g.adjacency {
		if _, found := indices[c]; !found {
			strongConnect(c)
		}
	}

	return sccs
}
