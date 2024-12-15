package validation

import (
	"fmt"
	"github.com/NVIDIA/grove/operator/internal/utils"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateEnumType[T comparable](value *T, allowedValues sets.Set[T], fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if errs := validateNonNilField(value, fldPath); len(errs) > 0 {
		return errs
	}
	if !allowedValues.Has(*value) {
		allErrs = append(allErrs, field.Invalid(fldPath, *value, fmt.Sprintf("can only be one of %v", allowedValues)))
	}
	return allErrs
}

func validateNonNilField[T any](value *T, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if value == nil {
		return append(allErrs, field.Required(fldPath, "field is required"))
	}
	return allErrs
}

func validateNonEmptyStringField(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if utils.IsEmptyStringType(value) {
		return append(allErrs, field.Required(fldPath, "field cannot be empty"))
	}
	return allErrs
}
