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
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"
	"github.com/samber/lo"
)

func (v *validator) validatePodCliques(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}
	var warnings []string
	cliques := v.pgs.Spec.Template.Cliques
	isExplicit := (v.pgs.Spec.Template.StartupType != nil && *v.pgs.Spec.Template.StartupType == v1alpha1.CliqueStartupTypeExplicit)

	if len(cliques) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "at least on PodClique must be defined"))
	}

	cliqueNames := make([]string, 0, len(cliques))
	for _, clique := range cliques {
		cliqueNames = append(cliqueNames, clique.Name)
		warns, errs := v.validatePodClique(clique, fldPath)
		if len(errs) != 0 {
			allErrs = append(allErrs, errs...)
		}
		if len(warns) != 0 {
			warnings = append(warnings, warns...)
		}
	}

	duplicateCliqueNames := lo.FindDuplicates(cliqueNames)
	if len(duplicateCliqueNames) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"),
			strings.Join(duplicateCliqueNames, ","), "clique names must be unique"))
	}

	if isExplicit {
		allErrs = append(allErrs, validateCliqueDependencies(cliques, fldPath)...)
	}

	return warnings, allErrs
}

func (v *validator) validatePodClique(clique v1alpha1.PodClique, fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateNonEmptyStringField(clique.Name, fldPath.Child("name"))...)
	if clique.Size == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("size"), "field is required"))
	} else if *clique.Size <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("size"), *clique.Size, "must be greater than 0"))
	}

	// validate that the StartsAfter values are not empty strings
	if v.pgs.Spec.Template.StartupType != nil && *v.pgs.Spec.Template.StartupType == v1alpha1.CliqueStartupTypeExplicit {
		for _, dep := range clique.StartsAfter {
			if utils.IsEmptyStringType(dep) {
				allErrs = append(allErrs, field.Required(fldPath.Child("startsAfter"), "clique dependency must not be empty"))
			}
			if dep == clique.Name {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"), dep, "clique dependency cannot refer to itself"))
			}
		}
		duplicateStartAfterDeps := lo.FindDuplicates(clique.StartsAfter)
		if len(duplicateStartAfterDeps) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"),
				strings.Join(duplicateStartAfterDeps, ","), "clique dependencies must be unique"))
		}
	}

	warnings, cliquePodTemplateSpecErrs := v.validateCliquePodTemplateSpec(clique.Template, fldPath.Child("template"))
	if len(cliquePodTemplateSpecErrs) != 0 {
		allErrs = append(allErrs, cliquePodTemplateSpecErrs...)
	}

	return warnings, allErrs
}

func (v *validator) validateCliquePodTemplateSpec(template corev1.PodTemplateSpec, fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}
	var warnings []string

	allErrs = append(allErrs, metav1validation.ValidateLabels(template.ObjectMeta.Labels, fldPath.Child("metadata"))...)

	if !utils.IsEmptyStringType(template.Spec.RestartPolicy) {
		warnings = append(warnings, "restartPolicy will be ignored, it will be set to Never")
	}

	specFldPath := fldPath.Child("spec")
	if v.operation == admissionv1.Create {
		if template.Spec.TopologySpreadConstraints != nil {
			allErrs = append(allErrs, field.Invalid(specFldPath.Child("topologySpreadConstraints"), template.Spec.TopologySpreadConstraints, "must not be set"))
		}
		if !utils.IsEmptyStringType(template.Spec.NodeName) {
			allErrs = append(allErrs, field.Invalid(specFldPath.Child("nodeName"), template.Spec.NodeName, "must not be set"))
		}
	}
	if template.Spec.NodeSelector != nil {
		allErrs = append(allErrs, field.Invalid(specFldPath.Child("nodeSelector"), template.Spec.NodeSelector, "must not be set"))
	}

	return warnings, allErrs
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
	var parents []*parent
	var numCliquesHavingNoStartsAfter int
	// initialize adjacency matrix that describes "startsAfter" dependencies between cliques
	for i, clique := range cliques {
		if len(clique.StartsAfter) == 0 {
			numCliquesHavingNoStartsAfter++
		}
		adjMatrix[i] = make([]int, n)
		for j, startsAfter := range clique.StartsAfter {
			parents = append(parents, &parent{name: startsAfter, cliqueIndx: i, indx: j})
		}
	}

	if numCliquesHavingNoStartsAfter == 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath, "must contain at least one clique without startsAfter"))
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
			if child := cliques[p.cliqueIndx].StartsAfter[p.indx]; len(child) != 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"), child, "must have matching clique"))
			}
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
