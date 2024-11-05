package validation

import (
	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

// ValidateOperatorConfiguration validates the operator configuration.
func ValidateOperatorConfiguration(config *configv1alpha1.OperatorConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateLogConfiguration(config)...)
	allErrs = append(allErrs, validateControllerConfiguration(config.Controllers, field.NewPath("controllers"))...)
	return allErrs
}

func validateLogConfiguration(config *configv1alpha1.OperatorConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}
	if !utils.IsEmptyStringType(config.LogLevel) && !sets.New(configv1alpha1.AllLogLevels...).Has(config.LogLevel) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), config.LogLevel, configv1alpha1.AllLogLevels))
	}
	if !utils.IsEmptyStringType(config.LogFormat) && !sets.New(configv1alpha1.AllLogFormats...).Has(config.LogFormat) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), config.LogFormat, configv1alpha1.AllLogFormats))
	}
	return allErrs
}

func validateControllerConfiguration(controllerCfg configv1alpha1.ControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodGangSetControllerConfiguration(controllerCfg.PodGangSet, fldPath.Child("podGangSet"))...)
	return allErrs
}

func validatePodGangSetControllerConfiguration(pgsCfg configv1alpha1.PodGangSetControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateConcurrentSyncs(pgsCfg.ConcurrentSyncs, fldPath)...)
	return allErrs
}

func validateConcurrentSyncs(concurrentSyncs *int, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if ptr.Deref(concurrentSyncs, 0) <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("concurrentSyncs"), concurrentSyncs, "must be greater than 0"))
	}
	return allErrs
}
