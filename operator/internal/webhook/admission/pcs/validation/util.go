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
	"fmt"
	"strings"

	"github.com/NVIDIA/grove/operator/internal/utils"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateEnumType[T comparable](value *T, allowedValues sets.Set[T], fldPath *field.Path) field.ErrorList {
	allErrs := validateNonNilField(value, fldPath)
	if len(allErrs) != 0 {
		return allErrs
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

func sliceMustHaveUniqueElements(s []string, fldPath *field.Path, msg string) field.ErrorList {
	allErrs := field.ErrorList{}
	duplicates := lo.FindDuplicates(s)
	if len(duplicates) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, strings.Join(duplicates, ","), msg))
	}
	return allErrs
}
