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
	"slices"
	"strings"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"

	"github.com/samber/lo"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var allowedStartupTypes = sets.New(grovecorev1alpha1.CliqueStartupTypeInOrder, grovecorev1alpha1.CliqueStartupTypeAnyOrder, grovecorev1alpha1.CliqueStartupTypeExplicit)

type pgsValidator struct {
	operation admissionv1.Operation
	pgs       *grovecorev1alpha1.PodGangSet
}

func newPGSValidator(pgs *grovecorev1alpha1.PodGangSet, operation admissionv1.Operation) *pgsValidator {
	return &pgsValidator{
		operation: operation,
		pgs:       pgs,
	}
}

// validate validates the PodGangSet object.
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

// validateUpdate validates the update to a PodGangSet object. It compares the old and new PodGangSet objects and validates that the changes done are allowed/valid.
func (v *pgsValidator) validateUpdate(oldPgs *grovecorev1alpha1.PodGangSet) error {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("spec")
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(v.pgs.Spec.ReplicaSpreadConstraints, oldPgs.Spec.ReplicaSpreadConstraints, fldPath.Child("replicaSpreadConstraints"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(v.pgs.Spec.Template.PriorityClassName, oldPgs.Spec.Template.PriorityClassName, fldPath.Child("priorityClassName"))...)
	allErrs = append(allErrs, validatePodGangSetSpecUpdate(&v.pgs.Spec, &oldPgs.Spec, fldPath)...)
	return allErrs.ToAggregate()
}

// validatePodGangSetSpec validates the specification of a PodGangSet object.
func (v *pgsValidator) validatePodGangSetSpec(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(v.pgs.Spec.Replicas), fldPath.Child("replicas"))...)
	warnings, errs := v.validatePodGangTemplateSpec(fldPath.Child("template"))
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}

	return warnings, allErrs
}

func (v *pgsValidator) validatePodGangTemplateSpec(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateEnumType(v.pgs.Spec.Template.StartupType, allowedStartupTypes, fldPath.Child("cliqueStartupType"))...)
	// validate cliques
	warnings, errs := v.validatePodCliqueTemplates(fldPath.Child("cliques"))
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	allErrs = append(allErrs, v.validatePodGangSchedulingPolicyConfig(v.pgs.Spec.Template.SchedulingPolicyConfig, fldPath.Child("schedulingPolicyConfig"))...)
	allErrs = append(allErrs, v.validatePodCliqueScalingGroupConfigs(fldPath.Child("podCliqueScalingGroups"))...)

	return warnings, allErrs
}

func (v *pgsValidator) validatePodCliqueTemplates(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	var warnings []string
	cliqueTemplateSpecs := v.pgs.Spec.Template.Cliques
	if len(cliqueTemplateSpecs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "at least one PodClique must be defined"))
	}

	cliqueNames := make([]string, 0, len(cliqueTemplateSpecs))
	schedulerNames := make([]string, 0, len(cliqueTemplateSpecs))
	for _, cliqueTemplateSpec := range cliqueTemplateSpecs {
		cliqueNames = append(cliqueNames, cliqueTemplateSpec.Name)
		warns, errs := v.validatePodCliqueTemplateSpec(cliqueTemplateSpec, fldPath)
		if len(errs) != 0 {
			allErrs = append(allErrs, errs...)
		}
		if len(warns) != 0 {
			warnings = append(warnings, warns...)
		}
		schedulerNames = append(schedulerNames, cliqueTemplateSpec.Spec.PodSpec.SchedulerName)
	}

	duplicateCliqueNames := lo.FindDuplicates(cliqueNames)
	if len(duplicateCliqueNames) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"),
			strings.Join(duplicateCliqueNames, ","), "cliqueTemplateSpec names must be unique"))
	}

	uniqueSchedulerNames := lo.Uniq(lo.Map(schedulerNames, func(item string, _ int) string {
		if item == "" {
			return "default-scheduler"
		}
		return item
	}))
	if len(uniqueSchedulerNames) > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("spec").Child("podSpec").Child("schedulerName"), uniqueSchedulerNames[0], "the schedulerName for all pods have to be the same"))
	}

	if v.isStartupTypeExplicit() {
		allErrs = append(allErrs, validateCliqueDependencies(cliqueTemplateSpecs, fldPath)...)
	}

	return warnings, allErrs
}

func (v *pgsValidator) validatePodCliqueTemplateSpec(cliqueTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, fldPath *field.Path) ([]string, field.ErrorList) {
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

func (v *pgsValidator) validatePodCliqueSpec(name string, cliqueSpec grovecorev1alpha1.PodCliqueSpec, fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	if cliqueSpec.Replicas <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), cliqueSpec.Replicas, "must be greater than 0"))
	}

	// Ideally this should never happen, the defaulting webhook will always set the default value for minAvailable.
	if cliqueSpec.MinAvailable == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("minAvailable"), "field is required"))
	}
	if *cliqueSpec.MinAvailable <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minAvailable"), *cliqueSpec.MinAvailable, "must be greater than 0"))
	}
	if *cliqueSpec.MinAvailable > cliqueSpec.Replicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minAvailable"), *cliqueSpec.MinAvailable, "minAvailable must not be greater than replicas"))
	}

	if v.isStartupTypeExplicit() && len(cliqueSpec.StartsAfter) > 0 {
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
		allErrs = append(allErrs, validateScaleConfig(cliqueSpec.ScaleConfig, *cliqueSpec.MinAvailable, fldPath.Child("autoScalingConfig"))...)
		if cliqueSpec.ScaleConfig.MaxReplicas < cliqueSpec.Replicas {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("autoScalingConfig", "maxReplicas"), cliqueSpec.ScaleConfig.MaxReplicas, "must be greater than or equal to replicas"))
		}
	}

	warnings, cliquePodSpecErrs := v.validatePodSpec(cliqueSpec.PodSpec, fldPath.Child("podSpec"))
	if len(cliquePodSpecErrs) != 0 {
		allErrs = append(allErrs, cliquePodSpecErrs...)
	}

	return warnings, allErrs
}

func validateScaleConfig(scaleConfig *grovecorev1alpha1.AutoScalingConfig, minAvailable int32, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// This should ideally not happen, the defaulting webhook will always set the default value for minReplicas.
	if scaleConfig.MinReplicas == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("minReplicas"), "field is required"))
	}
	// scaleConfig.MinReplicas should be greater than or equal to minAvailable else it will trigger a PodGang termination.
	if *scaleConfig.MinReplicas < minAvailable {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minReplicas"), *scaleConfig.MinReplicas, "must be greater than or equal to podCliqueSpec.minAvailable"))
	}
	if scaleConfig.MaxReplicas < *scaleConfig.MinReplicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxReplicas"), scaleConfig.MaxReplicas, "must be greater than or equal to podCliqueSpec.minReplicas"))
	}
	return allErrs
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

func validateCliqueDependencies(cliques []*grovecorev1alpha1.PodCliqueTemplateSpec, fldPath *field.Path) field.ErrorList {
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

func (v *pgsValidator) validatePodGangSchedulingPolicyConfig(schedulingPolicyConfig *grovecorev1alpha1.SchedulingPolicyConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if schedulingPolicyConfig == nil {
		return allErrs
	}
	if schedulingPolicyConfig.TerminationDelay != nil {
		allErrs = append(allErrs, mustBeEqualToOrGreaterThanZeroDuration(*schedulingPolicyConfig.TerminationDelay, fldPath.Child("terminationDelay"))...)
	}
	if len(schedulingPolicyConfig.NetworkPackGroupConfigs) > 0 {
		allErrs = append(allErrs, v.validateNetworkPackGroupConfigs(fldPath.Child("networkPackGroupConfigs"))...)
	}
	return allErrs
}

func (v *pgsValidator) validateNetworkPackGroupConfigs(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, v.checkNetworkPackGroupConfigsForDuplicates(fldPath)...)
	allErrs = append(allErrs, v.checkNetworkPackGroupConfigsForPartialPCSGInclusions(fldPath)...)
	return allErrs
}

func (v *pgsValidator) checkNetworkPackGroupConfigsForDuplicates(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	networkPackGroupConfigs := v.pgs.Spec.Template.SchedulingPolicyConfig.NetworkPackGroupConfigs
	var allCliqueNames []string
	for _, nwPackGrpConfig := range networkPackGroupConfigs {
		allCliqueNames = append(allCliqueNames, nwPackGrpConfig.CliqueNames...)
	}

	// validate that a clique cannot be present in more than one NetworkPackGroupConfig.
	duplicateCliqueNames := lo.FindDuplicates(allCliqueNames)
	if len(duplicateCliqueNames) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("cliqueNames"), strings.Join(duplicateCliqueNames, ","), "A PodClique cannot belong to more than one NetworkPackGroupConfig"))
	}

	return allErrs
}

func (v *pgsValidator) checkNetworkPackGroupConfigsForPartialPCSGInclusions(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	pcsgConfigs := v.pgs.Spec.Template.PodCliqueScalingGroupConfigs
	if len(pcsgConfigs) == 0 {
		return allErrs
	}
	for _, nwPackGrpConfig := range v.pgs.Spec.Template.SchedulingPolicyConfig.NetworkPackGroupConfigs {
		for _, cliqueName := range nwPackGrpConfig.CliqueNames {
			matchingPCSG, ok := lo.Find(pcsgConfigs, func(pcsg grovecorev1alpha1.PodCliqueScalingGroupConfig) bool {
				return slices.Contains(pcsg.CliqueNames, cliqueName)
			})
			if !ok {
				continue
			}
			absentPCSGCliqueNames, _ := lo.Difference(nwPackGrpConfig.CliqueNames, matchingPCSG.CliqueNames)
			if len(absentPCSGCliqueNames) > 0 {
				return append(allErrs, field.Invalid(fldPath.Child("cliqueName"), strings.Join(absentPCSGCliqueNames, ","), "NetworkPackGroupConfig cannot partially include PodCliques that are a part of a PodCliqueScalingGroup"))
			}
		}
	}
	return allErrs
}

func (v *pgsValidator) validatePodCliqueScalingGroupConfigs(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allPodGangSetCliqueNames := lo.Map(v.pgs.Spec.Template.Cliques, func(cliqueTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, _ int) string {
		return cliqueTemplateSpec.Name
	})
	pclqScalingGroupNames := make([]string, 0, len(v.pgs.Spec.Template.PodCliqueScalingGroupConfigs))
	var cliqueNamesAcrossAllScalingGroups []string

	for _, scalingGroupConfig := range v.pgs.Spec.Template.PodCliqueScalingGroupConfigs {
		pclqScalingGroupNames = append(pclqScalingGroupNames, scalingGroupConfig.Name)
		cliqueNamesAcrossAllScalingGroups = append(cliqueNamesAcrossAllScalingGroups, scalingGroupConfig.CliqueNames...)
		// validate that scaling groups only contains clique names that are defined in the PodGangSet.
		allErrs = append(allErrs, validateScalingGroupPodCliqueNames(allPodGangSetCliqueNames, scalingGroupConfig.CliqueNames, fldPath.Child("cliqueNames"))...)
	}

	// validate that the scaling group names are unique
	duplicateScalingGroupNames := lo.FindDuplicates(pclqScalingGroupNames)
	if len(duplicateScalingGroupNames) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), strings.Join(duplicateScalingGroupNames, ","), "PodCliqueScalingGroupConfig names must be unique"))
	}

	// validate that there should not be any overlapping clique names across scaling groups.
	overlappingCliqueNames := lo.FindDuplicates(cliqueNamesAcrossAllScalingGroups)
	if len(overlappingCliqueNames) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("cliqueNames"), strings.Join(overlappingCliqueNames, ","), "clique names must not overlap across scaling groups, every scaling group should have unique clique names"))
	}

	// validate that for all pod cliques that are part of defined scaling groups, separate AutoScalingConfig is not defined for them.
	scalingGroupCliqueNames := lo.Uniq(cliqueNamesAcrossAllScalingGroups)
	for _, cliqueTemplateSpec := range v.pgs.Spec.Template.Cliques {
		if slices.Contains(scalingGroupCliqueNames, cliqueTemplateSpec.Name) && cliqueTemplateSpec.Spec.ScaleConfig != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, cliqueTemplateSpec.Name, "AutoScalingConfig is not allowed to be defined for PodClique that is part of scaling group"))
		}
	}

	return allErrs
}

// checks if the PodClique names specified in PodCliqueScalingGroupConfig refer to a defined clique in the PodGangSet.
func validateScalingGroupPodCliqueNames(allPclqNames, pclqNameInScalingGrp []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	_, unidentifiedPclqNames := lo.Difference(allPclqNames, lo.Uniq(pclqNameInScalingGrp))
	if len(unidentifiedPclqNames) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, strings.Join(unidentifiedPclqNames, ","), "unidentified PodClique names found"))
	}

	return allErrs
}

func validatePodGangSetSpecUpdate(newSpec, oldSpec *grovecorev1alpha1.PodGangSetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodGangTemplateSpecUpdate(&newSpec.Template, &oldSpec.Template, fldPath.Child("template"))...)
	return allErrs
}

func validatePodGangTemplateSpecUpdate(newSpec, oldSpec *grovecorev1alpha1.PodGangSetTemplateSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validatePodCliqueUpdate(newSpec.Cliques, oldSpec.Cliques, fldPath.Child("cliques"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.StartupType, oldSpec.StartupType, fldPath.Child("cliqueStartupType"))...)
	allErrs = append(allErrs, validatePodGangSchedulingPolicyConfigUpdate(newSpec.SchedulingPolicyConfig, newSpec.SchedulingPolicyConfig, fldPath.Child("schedulingPolicyConfig"))...)

	return allErrs
}

func validatePodGangSchedulingPolicyConfigUpdate(newConfig, oldConfig *grovecorev1alpha1.SchedulingPolicyConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig, oldConfig, fldPath.Child("networkPackStrategy"))...)
	return allErrs
}

func validatePodCliqueUpdate(newCliques, oldCliques []*grovecorev1alpha1.PodCliqueTemplateSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(newCliques) != len(oldCliques) {
		allErrs = append(allErrs, field.Forbidden(fldPath, "not allowed to change clique composition"))
	}
	for i := range newCliques {
		// TODO: check name
		allErrs = append(allErrs, validatePodSpecUpdate(&newCliques[i].Spec.PodSpec, &oldCliques[i].Spec.PodSpec, fldPath.Child("spec", "podSpec"))...)
	}

	return allErrs
}

func validatePodSpecUpdate(newSpec, oldSpec *corev1.PodSpec, fldPath *field.Path) field.ErrorList {
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

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(spec1, spec2, fldPath)...)

	return allErrs
}

func clearContainerImages(containers []corev1.Container) {
	for i := range containers {
		containers[i].Image = ""
	}
}

func (v *pgsValidator) isStartupTypeExplicit() bool {
	return v.pgs.Spec.Template.StartupType != nil && *v.pgs.Spec.Template.StartupType == grovecorev1alpha1.CliqueStartupTypeExplicit
}
