// /*
// Copyright 2025 The Grove Authors.
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

package podclique

import (
	"context"
	"fmt"
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	groveclientscheme "github.com/NVIDIA/grove/operator/internal/client"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testPGSetName    = "coyote"
	testPGSNamespace = "cobalt-ns"
)

func TestGetExistingResourceNames(t *testing.T) {
	testCases := []struct {
		description                 string
		pgsReplicas                 int32
		podCliqueTemplateNames      []string
		podCliqueNamesNotOwnedByPGS []string
		expectedPodCliqueNames      []string
		listErr                     *apierrors.StatusError
		expectedErr                 *groveerr.GroveError
	}{
		{
			description:            "PodCliqueSet has zero replicas and one PodClique",
			pgsReplicas:            0,
			podCliqueTemplateNames: []string{"howl"},
			expectedPodCliqueNames: []string{},
		},
		{
			description:            "PodCliqueSet has one replica and two PodCliques",
			pgsReplicas:            1,
			podCliqueTemplateNames: []string{"howl", "grin"},
			expectedPodCliqueNames: []string{"coyote-0-howl", "coyote-0-grin"},
		},
		{
			description:            "PodCliqueSet has two replicas and two PodCliques",
			pgsReplicas:            3,
			podCliqueTemplateNames: []string{"howl", "grin"},
			expectedPodCliqueNames: []string{"coyote-0-howl", "coyote-0-grin", "coyote-1-howl", "coyote-1-grin", "coyote-2-howl", "coyote-2-grin"},
		},
		{
			description:                 "PodCliqueSet has two replicas and two PodCliques with one not owned by the PodCliqueSet",
			pgsReplicas:                 2,
			podCliqueTemplateNames:      []string{"howl"},
			podCliqueNamesNotOwnedByPGS: []string{"bandit"},
			expectedPodCliqueNames:      []string{"coyote-0-howl", "coyote-1-howl"},
		},
		{
			description:            "should return error when list fails",
			pgsReplicas:            2,
			podCliqueTemplateNames: []string{"howl"},
			listErr:                testutils.TestAPIInternalErr,
			expectedErr: &groveerr.GroveError{
				Code:      errCodeListPodCliques,
				Cause:     testutils.TestAPIInternalErr,
				Operation: component.OperationGetExistingResourceNames,
			},
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			// Create a PodCliqueSet with the specified number of replicas and PodCliques
			pgsBuilder := testutils.NewPodGangSetBuilder(testPGSetName, testPGSNamespace, uuid.NewUUID()).
				WithReplicas(tc.pgsReplicas).
				WithCliqueStartupType(ptr.To(grovecorev1alpha1.CliqueStartupTypeAnyOrder))
			for _, pclqTemplateName := range tc.podCliqueTemplateNames {
				pgsBuilder.WithPodCliqueParameters(pclqTemplateName, 1, nil)
			}
			pgs := pgsBuilder.Build()
			// Create existing objects
			existingObjects := createExistingPodCliquesFromPGS(pgs, tc.podCliqueNamesNotOwnedByPGS)
			// Create a fake client with PodCliques
			cl := testutils.CreateFakeClientForObjectsMatchingLabels(nil, tc.listErr, pgs.Namespace, grovecorev1alpha1.SchemeGroupVersion.WithKind("PodClique"), getPodCliqueSelectorLabels(pgs.ObjectMeta), existingObjects...)
			operator := New(cl, groveclientscheme.Scheme, record.NewFakeRecorder(10))
			actualPCLQNames, err := operator.GetExistingResourceNames(context.Background(), logr.Discard(), pgs.ObjectMeta)
			if tc.expectedErr == nil {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tc.expectedPodCliqueNames, actualPCLQNames)
			} else {
				testutils.CheckGroveError(t, tc.expectedErr, err)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	testCases := []struct {
		description           string
		numExistingPodCliques int
		deleteError           *apierrors.StatusError
		expectedError         *groveerr.GroveError
	}{
		{
			description:           "no-op when there are no existing PodCliques",
			numExistingPodCliques: 0,
			expectedError:         nil,
		},
		{
			description:           "successfully delete all existing PodCliques",
			numExistingPodCliques: 2,
			expectedError:         nil,
		},
		{
			description:           "error when deleting existing PodCliques",
			numExistingPodCliques: 2,
			deleteError:           testutils.TestAPIInternalErr,
			expectedError: &groveerr.GroveError{
				Code:      errDeletePodClique,
				Cause:     testutils.TestAPIInternalErr,
				Operation: component.OperationDelete,
			},
		},
	}

	t.Parallel()
	pgsObjMeta := metav1.ObjectMeta{
		Name:      testPGSetName,
		Namespace: testPGSNamespace,
		UID:       uuid.NewUUID(),
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			existingPodCliques := createDefaultPodCliques(pgsObjMeta, "howl", tc.numExistingPodCliques)
			// Create a fake client with PodCliques
			cl := testutils.CreateFakeClientForObjectsMatchingLabels(tc.deleteError, nil, testPGSNamespace, grovecorev1alpha1.SchemeGroupVersion.WithKind("PodClique"), getPodCliqueSelectorLabels(pgsObjMeta), existingPodCliques...)
			operator := New(cl, groveclientscheme.Scheme, record.NewFakeRecorder(10))
			err := operator.Delete(context.Background(), logr.Discard(), pgsObjMeta)
			if tc.expectedError != nil {
				testutils.CheckGroveError(t, tc.expectedError, err)
			} else {
				assert.NoError(t, err)
				podCliquesPostDelete := getExistingPodCliques(t, cl, pgsObjMeta)
				assert.Empty(t, podCliquesPostDelete)
			}
		})
	}
}

func getExistingPodCliques(t *testing.T, cl client.Client, pgsObjMeta metav1.ObjectMeta) []grovecorev1alpha1.PodClique {
	podCliqueList := &grovecorev1alpha1.PodCliqueList{}
	assert.NoError(t, cl.List(context.Background(), podCliqueList, client.InNamespace(pgsObjMeta.Namespace), client.MatchingLabels(getPodCliqueSelectorLabels(pgsObjMeta))))
	return podCliqueList.Items
}

func createDefaultPodCliques(pgsObjMeta metav1.ObjectMeta, pclqNamePrefix string, numPodCliques int) []client.Object {
	podCliqueNames := make([]client.Object, 0, numPodCliques)
	for i := range numPodCliques {
		pclq := testutils.NewPodCliqueBuilder(pgsObjMeta.Name, pgsObjMeta.GetUID(), fmt.Sprintf("%s-%d", pclqNamePrefix, i), pgsObjMeta.Namespace, 0).
			WithLabels(getPodCliqueSelectorLabels(pgsObjMeta)).
			Build()
		podCliqueNames = append(podCliqueNames, pclq)
	}
	return podCliqueNames
}

func createExistingPodCliquesFromPGS(pgs *grovecorev1alpha1.PodCliqueSet, podCliqueNamesNotOwnedByPGS []string) []client.Object {
	existingPodCliques := make([]client.Object, 0, len(pgs.Spec.Template.Cliques)*int(pgs.Spec.Replicas)+len(podCliqueNamesNotOwnedByPGS))
	for replicaIndex := range pgs.Spec.Replicas {
		for _, pclqTemplate := range pgs.Spec.Template.Cliques {
			pclq := testutils.NewPodCliqueBuilder(pgs.Name, pgs.UID, pclqTemplate.Name, pgs.Namespace, replicaIndex).
				WithLabels(getPodCliqueSelectorLabels(pgs.ObjectMeta)).
				WithStartsAfter(pclqTemplate.Spec.StartsAfter).
				Build()
			existingPodCliques = append(existingPodCliques, pclq)
		}
	}
	// Add additional PodCliques not owned by the PodCliqueSet
	nonExistingPGSName := "ebony"
	for _, podCliqueName := range podCliqueNamesNotOwnedByPGS {
		pclq := testutils.NewPodCliqueBuilder(nonExistingPGSName, uuid.NewUUID(), podCliqueName, pgs.Namespace, 0).
			WithOwnerReference("PodCliqueSet", nonExistingPGSName, uuid.NewUUID()).Build()
		existingPodCliques = append(existingPodCliques, pclq)
	}
	return existingPodCliques
}
