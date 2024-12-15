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
	"testing"

	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"
)

func ptrInt32(v int32) *int32 {
	ret := v
	return &ret
}

func TestValidateCliques(t *testing.T) {
	explicit := v1alpha1.CliqueStartupTypeExplicit

	testCases := []struct {
		name        string
		cliques     []v1alpha1.PodClique
		startupType *v1alpha1.CliqueStartupType
		errs        []string
	}{
		{
			name: "Case 1: no cliques",
			errs: []string{"spec.template.spec.cliques: Required value: at least on PodClique must be defined"},
		},
		{
			name: "Case 2: invalid cliques",
			cliques: []v1alpha1.PodClique{
				{
					Name: "clique1",
					Size: ptrInt32(2),
				},
				{
					Size: nil,
				},
				{
					Name: "clique1",
					Size: ptrInt32(-2),
				},
			},
			errs: []string{
				`spec.template.spec.cliques.name: Required value: field cannot be empty`,
				`spec.template.spec.cliques.size: Required value: field is required`,
				`spec.template.spec.cliques.size: Invalid value: -2: must be greater than 0`,
				`spec.template.spec.cliques.name: Invalid value: "clique1": clique names must be unique`,
			},
		},
		{
			name:        "Case 3: invalid 'startsAfter'",
			startupType: &explicit,
			cliques: []v1alpha1.PodClique{
				{
					Name:        "clique1",
					Size:        ptrInt32(2),
					StartsAfter: []string{"clique2"},
				},
				{
					Name:        "clique2",
					Size:        ptrInt32(1),
					StartsAfter: []string{"clique1"},
				},
				{
					Name:        "clique3",
					Size:        ptrInt32(1),
					StartsAfter: []string{"clique5"},
				},
				{
					Name:        "clique4",
					Size:        ptrInt32(1),
					StartsAfter: []string{"clique3", "clique3"},
				},
			},
			errs: []string{
				`spec.template.spec.cliques.startsAfter: Invalid value: "clique3": clique dependencies must be unique`,
				`spec.template.spec.cliques: Forbidden: must contain at least one clique without startsAfter`,
				`spec.template.spec.cliques.startsAfter: Invalid value: "clique5": must have matching clique`,
				`spec.template.spec.cliques: Forbidden: cannot have circular dependencies`,
			},
		},
		{
			name:        "Case 4: self-referencing",
			startupType: &explicit,
			cliques: []v1alpha1.PodClique{
				{
					Name:        "clique8",
					Size:        ptrInt32(2),
					StartsAfter: []string{"clique8"},
				},
				{
					Name:        "clique6",
					Size:        ptrInt32(1),
					StartsAfter: []string{""},
				},
			},
			errs: []string{
				`spec.template.spec.cliques.startsAfter: Invalid value: "clique8": clique dependency cannot refer to itself`,
				`spec.template.spec.cliques.startsAfter: Required value: clique dependency must not be empty`,
				`spec.template.spec.cliques: Forbidden: must contain at least one clique without startsAfter`,
				`spec.template.spec.cliques.startsAfter: Invalid value: "clique8": must have matching clique`,
			},
		},
		{
			name:        "Case 5: valid input",
			startupType: &explicit,
			//       clique1          clique2
			//       /  \
			// clique3    clique4
			//      \    /
			//     clique5
			//
			cliques: []v1alpha1.PodClique{
				{
					Name: "clique1",
					Size: ptrInt32(2),
				},
				{
					Name: "clique2",
					Size: ptrInt32(2),
				},
				{
					Name:        "clique3",
					Size:        ptrInt32(1),
					StartsAfter: []string{"clique1"},
				},
				{
					Name:        "clique4",
					Size:        ptrInt32(1),
					StartsAfter: []string{"clique1"},
				},
				{
					Name:        "clique5",
					Size:        ptrInt32(1),
					StartsAfter: []string{"clique3", "clique4"},
				},
			},
			errs: []string{},
		},
	}

	fldPath := field.NewPath("spec").Child("template", "spec", "cliques")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v := &validator{
				operation: admissionv1.Create,
				pgs: &v1alpha1.PodGangSet{
					Spec: v1alpha1.PodGangSetSpec{
						Template: v1alpha1.PodGangTemplateSpec{
							Cliques:     tc.cliques,
							StartupType: tc.startupType,
						},
					},
				},
			}

			_, errs := v.validatePodCliques(fldPath)
			errList := make([]string, len(errs))
			for i, err := range errs {
				errList[i] = err.Error()
			}
			require.Equal(t, tc.errs, errList)
		})
	}
}
