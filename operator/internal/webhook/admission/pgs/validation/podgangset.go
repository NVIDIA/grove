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
	"reflect"
	"strings"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"

	"github.com/samber/lo"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	allowedStartupTypes          = sets.New[v1alpha1.CliqueStartupType](v1alpha1.CliqueStartupTypeInOrder, v1alpha1.CliqueStartupTypeAnyOrder, v1alpha1.CliqueStartupTypeExplicit)
	allowedNetworkPackStrategies = sets.New[v1alpha1.NetworkPackStrategy](v1alpha1.BestEffort, v1alpha1.Strict)
)

type pgsValidator struct {
	operation admissionv1.Operation
	pgs       *v1alpha1.PodGangSet
}

func newPGSValidator(pgs *v1alpha1.PodGangSet, operation admissionv1.Operation) *pgsValidator {
	return &pgsValidator{
		operation: operation,
		pgs:       pgs,
	}
}

func (v *pgsValidator) validate() ([]string, error) {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&v.pgs.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	fldPath := field.NewPath("spec")
	warnings, errs := v.validatePodGangSetSpec(fldPath)
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return warnings, allErrs.ToAggregate()
}

func (v *pgsValidator) validateUpdate(oldPgs *v1alpha1.PodGangSet) error {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodGangSetSpecUpdate(field.NewPath("spec"), &v.pgs.Spec, &oldPgs.Spec)...)
	return allErrs.ToAggregate()
}

// validatePodGangSetSpec validates the specification of a PodGangSet object.
func (v *pgsValidator) validatePodGangSetSpec(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(v.pgs.Spec.Replicas), fldPath.Child("replicas"))...)
	allErrs = append(allErrs, v.validateUpdateStrategy(fldPath.Child("updateStrategy"))...)
	warnings, errs := v.validatePodGangTemplateSpec(fldPath.Child("template"))
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return warnings, allErrs
}

func (v *pgsValidator) validateUpdateStrategy(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	updateStrategy := v.pgs.Spec.UpdateStrategy
	if updateStrategy == nil {
		return append(errs, field.Required(fldPath, "field is required"))
	}
	return append(errs, v.validateRollingUpdateConfig(fldPath.Child("rollingUpdateConfig"))...)
}

func (v *pgsValidator) validateRollingUpdateConfig(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	rollingUpdateConfig := v.pgs.Spec.UpdateStrategy.RollingUpdateConfig
	replicas := int(v.pgs.Spec.Replicas)
	if rollingUpdateConfig == nil {
		return append(allErrs, field.Required(fldPath, "field is required"))
	}

	maxUnavailable, err := intstr.GetScaledValueFromIntOrPercent(rollingUpdateConfig.MaxUnavailable, replicas, false)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdateConfig.MaxUnavailable, err.Error()))
	}
	maxSurge, err := intstr.GetScaledValueFromIntOrPercent(rollingUpdateConfig.MaxSurge, replicas, false)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxSurge"), rollingUpdateConfig.MaxSurge, err.Error()))
	}

	// Ensure that MaxUnavailable and MaxSurge are non-negative.
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(maxUnavailable), fldPath.Child("maxUnavailable"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(maxSurge), fldPath.Child("maxSurge"))...)

	// Ensure that MaxUnavailable is not more than the replicas for the PodGangSet.
	if maxUnavailable > replicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdateConfig.MaxUnavailable, "cannot be greater than replicas"))
	}
	// Ensure that both MaxSurge and MaxUnavailable are not zero.
	if maxSurge == 0 && maxUnavailable == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdateConfig.MaxUnavailable, "cannot be 0 when maxSurge is 0"))
	}
	return allErrs
}

func (v *pgsValidator) validatePodGangTemplateSpec(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateEnumType(v.pgs.Spec.Template.StartupType, allowedStartupTypes, fldPath.Child("cliqueStartupType"))...)
	allErrs = append(allErrs, validateEnumType(v.pgs.Spec.Template.NetworkPackStrategy, allowedNetworkPackStrategies, fldPath.Child("networkPackStrategy"))...)

	// validate cliques
	warnings, errs := v.validatePodCliqueTemplates(fldPath.Child("cliques"))
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return warnings, allErrs
}

func (v *pgsValidator) validatePodCliqueTemplates(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}
	var warnings []string
	cliqueTemplateSpecs := v.pgs.Spec.Template.Cliques
	isExplicit := v.pgs.Spec.Template.StartupType != nil && *v.pgs.Spec.Template.StartupType == v1alpha1.CliqueStartupTypeExplicit

	if len(cliqueTemplateSpecs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "at least one PodClique must be defined"))
	}

	cliqueNames := make([]string, 0, len(cliqueTemplateSpecs))
	for _, cliqueTemplateSpec := range cliqueTemplateSpecs {
		cliqueNames = append(cliqueNames, cliqueTemplateSpec.Name)
		warns, errs := v.validatePodCliqueTemplateSpec(cliqueTemplateSpec, fldPath)
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
			strings.Join(duplicateCliqueNames, ","), "cliqueTemplateSpec names must be unique"))
	}

	if isExplicit {
		allErrs = append(allErrs, validateCliqueDependencies(cliqueTemplateSpecs, fldPath)...)
	}

	return warnings, allErrs
}

func (v *pgsValidator) validatePodCliqueTemplateSpec(cliqueTemplateSpec *v1alpha1.PodCliqueTemplateSpec, fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateNonEmptyStringField(cliqueTemplateSpec.Name, fldPath.Child("name"))...)
	allErrs = append(allErrs, metav1validation.ValidateLabels(cliqueTemplateSpec.Labels, fldPath.Child("labels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(cliqueTemplateSpec.Annotations, fldPath.Child("annotations"))...)

	warnings, errs := v.validatePodCliqueSpec(cliqueTemplateSpec.Name, cliqueTemplateSpec.Spec, fldPath.Child("spec"))
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return warnings, allErrs
}

func (v *pgsValidator) validatePodCliqueSpec(name string, cliqueSpec v1alpha1.PodCliqueSpec, fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	if cliqueSpec.Replicas <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), cliqueSpec.Replicas, "must be greater than 0"))
	}

	isStartupTypeExplicit := v.pgs.Spec.Template.StartupType != nil && *v.pgs.Spec.Template.StartupType == v1alpha1.CliqueStartupTypeExplicit
	if isStartupTypeExplicit && len(cliqueSpec.StartsAfter) > 0 {
		for _, dep := range cliqueSpec.StartsAfter {
			if utils.IsEmptyStringType(dep) {
				allErrs = append(allErrs, field.Required(fldPath.Child("startsAfter"), "clique dependency must not be empty"))
			}
			if dep == name {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"), dep, "clique dependency cannot refer to itself"))
			}
		}
		duplicateStartAfterDeps := lo.FindDuplicates(cliqueSpec.StartsAfter)
		if len(duplicateStartAfterDeps) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"),
				strings.Join(duplicateStartAfterDeps, ","), "clique dependencies must be unique"))
		}
	}

	if cliqueSpec.ScaleConfig != nil {
		allErrs = append(allErrs, validateScaleConfig(cliqueSpec.ScaleConfig, fldPath.Child("autoScalingConfig"), cliqueSpec.Replicas)...)
	}

	warnings, cliquePodSpecErrs := v.validatePodSpec(cliqueSpec.PodSpec, fldPath.Child("podSpec"))
	if len(cliquePodSpecErrs) != 0 {
		allErrs = append(allErrs, cliquePodSpecErrs...)
	}

	return warnings, allErrs
}

func (v *pgsValidator) validatePodSpec(spec corev1.PodSpec, fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}
	var warnings []string

	if !utils.IsEmptyStringType(spec.RestartPolicy) {
		warnings = append(warnings, "restartPolicy will be ignored, it will be set to Always")
	}

	specFldPath := fldPath.Child("spec")
	if v.operation == admissionv1.Create {
		if spec.TopologySpreadConstraints != nil {
			allErrs = append(allErrs, field.Invalid(specFldPath.Child("topologySpreadConstraints"), spec.TopologySpreadConstraints, "must not be set"))
		}
		if !utils.IsEmptyStringType(spec.NodeName) {
			allErrs = append(allErrs, field.Invalid(specFldPath.Child("nodeName"), spec.NodeName, "must not be set"))
		}
	}
	if spec.NodeSelector != nil {
		allErrs = append(allErrs, field.Invalid(specFldPath.Child("nodeSelector"), spec.NodeSelector, "must not be set"))
	}

	return warnings, allErrs
}

func validateCliqueDependencies(cliques []*v1alpha1.PodCliqueTemplateSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	depG := NewPodCliqueDependencyGraph()
	var discoveredCliqueNames []string
	for _, clique := range cliques {
		discoveredCliqueNames = append(discoveredCliqueNames, clique.Name)
		depG.AddDependencies(clique.Name, clique.Spec.StartsAfter)
	}

	unknownCliquesDeps := depG.GetUnknownCliques(discoveredCliqueNames)
	if len(unknownCliquesDeps) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"),
			strings.Join(unknownCliquesDeps, ","), "unknown clique names found, all clique dependencies must be defined as cliques"))
	}

	// check for strongly connected components a.k.a cycles in the directed graph of clique dependencies
	cycles := depG.GetStronglyConnectedCliques()
	if len(cycles) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, cycles, "clique must not have circular dependencies"))
	}

	return allErrs
}

func validateScaleConfig(scaleConfig *v1alpha1.AutoScalingConfig, fldPath *field.Path, replicas int32) field.ErrorList {
	allErrs := field.ErrorList{}

	if scaleConfig.MinReplicas == nil {
		return append(allErrs, field.Required(fldPath.Child("minReplicas"), "field is required"))
	}

	if *scaleConfig.MinReplicas <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minReplicas"), *scaleConfig.MinReplicas, "must be greater than 0"))
	}

	if scaleConfig.MaxReplicas < *scaleConfig.MinReplicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxReplicas"), scaleConfig.MaxReplicas, "must be greater than or equal to minReplicas"))
	}

	if scaleConfig.MaxReplicas < replicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxReplicas"), scaleConfig.MaxReplicas, "must be greater than or equal to replicas"))
	}

	return allErrs
}

func validatePodGangSetSpecUpdate(fldPath *field.Path, newSpec, oldSpec *v1alpha1.PodGangSetSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validatePodGangTemplateSpecUpdate(fldPath.Child("template"), &newSpec.Template, &oldSpec.Template)...)

	if !reflect.DeepEqual(newSpec.GangSpreadConstraints, oldSpec.GangSpreadConstraints) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("gangSpreadConstraints"), "field is immutable"))
	}

	if newSpec.PriorityClassName != oldSpec.PriorityClassName {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("priorityClassName"), "field is immutable"))
	}

	return allErrs
}

func validatePodGangTemplateSpecUpdate(fldPath *field.Path, newSpec, oldSpec *v1alpha1.PodGangTemplateSpec) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodCliqueUpdate(fldPath.Child("cliques"), newSpec.Cliques, oldSpec.Cliques)...)
	if *newSpec.StartupType != *oldSpec.StartupType {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("cliqueStartupType"), "field is immutable"))
	}
	if *newSpec.NetworkPackStrategy != *oldSpec.NetworkPackStrategy {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("networkPackStrategy"), "field is immutable"))
	}
	return allErrs
}

func validatePodCliqueUpdate(fldPath *field.Path, newCliques, oldCliques []*v1alpha1.PodCliqueTemplateSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(newCliques) != len(oldCliques) {
		allErrs = append(allErrs, field.Forbidden(fldPath, "not allowed to change clique composition"))
	}
	for i := range newCliques {
		// TODO: check name
		allErrs = append(allErrs, validatePodSpecUpdate(fldPath.Child("spec", "podSpec"), &newCliques[i].Spec.PodSpec, &oldCliques[i].Spec.PodSpec)...)
	}

	return allErrs
}

func validatePodSpecUpdate(fldPath *field.Path, newSpec, oldSpec *corev1.PodSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	// spec: Forbidden: pod updates may not change fields other than:
	//  `spec.containers[*].image`,
	//  `spec.initContainers[*].image`,
	//  `spec.activeDeadlineSeconds`,
	//  `spec.tolerations` (only additions to existing tolerations),
	//  `spec.terminationGracePeriodSeconds` (allow it to be set to 1 if it was previously negative)
	if len(newSpec.Tolerations) < len(oldSpec.Tolerations) || !reflect.DeepEqual(oldSpec.Tolerations, newSpec.Tolerations[:len(oldSpec.Tolerations)]) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("tolerations"), "not allowed to change immutable pod fields"))
	}
	if *oldSpec.TerminationGracePeriodSeconds < 0 {
		// The only change that is allowed is to set this value to 1. All other modifications should be rejected.
		if *newSpec.TerminationGracePeriodSeconds != 1 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("terminationGracePeriodSeconds"), "value can only be set to 1 if previously negative"))
		}
	}
	// hide mutable fields
	spec1 := newSpec.DeepCopy()
	spec2 := oldSpec.DeepCopy()

	clearContainerImages(spec1.Containers)
	clearContainerImages(spec2.Containers)
	clearContainerImages(spec1.InitContainers)
	clearContainerImages(spec2.InitContainers)
	spec1.ActiveDeadlineSeconds, spec2.ActiveDeadlineSeconds = nil, nil
	spec1.Tolerations, spec2.Tolerations = []corev1.Toleration{}, []corev1.Toleration{}
	spec1.TerminationGracePeriodSeconds, spec2.TerminationGracePeriodSeconds = nil, nil

	if !reflect.DeepEqual(spec1, spec2) {
		allErrs = append(allErrs, field.Forbidden(fldPath, "not allowed to change immutable pod fields"))
	}

	return allErrs
}

func clearContainerImages(containers []corev1.Container) {
	for i := range containers {
		containers[i].Image = ""
	}
}
