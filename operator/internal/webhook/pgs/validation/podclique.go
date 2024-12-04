// /*
// Copyright 2024 The Grove Authors.
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

package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"
)

func validateCliques(cliques []v1alpha1.PodClique, fldPath *field.Path, isInOrder bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(cliques) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "field is required"))
	}
	var noStartsAfterCount int
	nameMap := make(map[string]struct{})

	for _, clique := range cliques {
		if len(clique.Name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("name"), "field is required"))
		} else if _, ok := nameMap[clique.Name]; ok {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), clique.Name, "duplicate name"))
		} else {
			nameMap[clique.Name] = struct{}{}
		}

		// TODO: validate PodTemplateSpec
		//corevalidation.ValidatePodTemplateSpec(&clique.Template, fldPath.Child("template"), corevalidation.PodValidationOptions{})

		// validate Size
		if clique.Size == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("size"), "field is required"))
		} else if *clique.Size <= 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("size"), *clique.Size, "must be greater than 0"))
		}

		// validate StartsAfter
		if isInOrder {
			if len(clique.StartsAfter) == 0 {
				noStartsAfterCount++
			} else {
				for _, parent := range clique.StartsAfter {
					if len(parent) == 0 {
						allErrs = append(allErrs, field.Required(fldPath.Child("startsAfter"), "must not contain empty clique name"))
					}
				}
			}
		}
	}

	if isInOrder {
		if noStartsAfterCount == 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath, "must contain at least one clique without startsAfter"))
		}
		allErrs = append(allErrs, validateCliqueDependencies(cliques, fldPath)...)
	}

	return allErrs
}

func validateCliqueDependencies(cliques []v1alpha1.PodClique, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	n := len(cliques)
	adjMatrix := make([][]int, n)

	// parent represents a parent clique
	type parent struct {
		name       string // name of the parent clique
		indx       int    // index of the parent clique name in []startsAfter
		cliqueIndx int    // index of the clique in []cliques
		match      bool   // has a matching parent clique
	}
	parents := []*parent{}

	// initialize adjacency matrix that describes "startsAfter" dependencies between cliques
	for i, clique := range cliques {
		adjMatrix[i] = make([]int, n)
		for j, startsAfter := range clique.StartsAfter {
			if startsAfter == clique.Name {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"), startsAfter, "cannot match its own clique"))
			} else {
				parents = append(parents, &parent{name: startsAfter, cliqueIndx: i, indx: j})
			}
		}
	}

	// fill in adjacency matrix
	for _, p := range parents {
		for i, clique := range cliques {
			if i != p.cliqueIndx {
				if p.name == clique.Name {
					adjMatrix[i][p.cliqueIndx] = 1
					p.match = true
				}
			}
		}
	}

	// check that all parents have matching cliques
	for _, p := range parents {
		if !p.match {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"),
				cliques[p.cliqueIndx].StartsAfter[p.indx], "must have matching clique"))
		}
	}

	// check for circular dependencies
	if hasCycle(adjMatrix) {
		allErrs = append(allErrs, field.Forbidden(fldPath, "cannot have circular dependencies"))
	}

	return allErrs
}

func hasCycle(adjMatrix [][]int) bool {
	n := len(adjMatrix)
	visited := make([]bool, n)
	recStack := make([]bool, n)

	for i := 0; i < n; i++ {
		if !visited[i] {
			if dfs(i, adjMatrix, visited, recStack) {
				return true
			}
		}
	}
	return false
}

func dfs(v int, adjMatrix [][]int, visited, recStack []bool) bool {
	visited[v] = true
	recStack[v] = true

	for i := 0; i < len(adjMatrix); i++ {
		if adjMatrix[v][i] == 1 {
			if !visited[i] {
				if dfs(i, adjMatrix, visited, recStack) {
					return true
				}
			} else if recStack[i] {
				return true
			}
		}
	}

	recStack[v] = false
	return false
}
