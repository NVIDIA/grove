package kubernetes

import (
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"testing"
)

const (
	testPgsName   = "test-pgs"
	testNamespace = "test-ns"
)

func TestGetDefaultLabelsForPodGangSetManagedResources(t *testing.T) {
	labels := GetDefaultLabelsForPodGangSetManagedResources(testPgsName)
	assert.Equal(t, labels, map[string]string{
		"app.kubernetes.io/managed-by": "grove-operator",
		"app.kubernetes.io/part-of":    testPgsName,
	})
}

func TestFilterMapOwnedResourceNames(t *testing.T) {
	testOwnerObjMeta := metav1.ObjectMeta{
		Name:      testPgsName,
		Namespace: testNamespace,
		UID:       uuid.NewUUID(),
	}
	testCases := []struct {
		description           string
		ownerObjMeta          metav1.ObjectMeta
		candidateResources    []metav1.PartialObjectMetadata
		expectedResourceNames []string
	}{
		{
			description:  "None of the resources are owned by the owner object",
			ownerObjMeta: testOwnerObjMeta,
			candidateResources: []metav1.PartialObjectMetadata{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "resource1",
						Namespace: testNamespace,
						UID:       uuid.NewUUID(),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PodGangSet",
								Name:       "other-pgs",
								UID:        uuid.NewUUID(),
								Controller: ptr.To(true),
							},
						},
					},
				},
			},
			expectedResourceNames: []string{},
		},
		{
			description:  "Some resources are owned by the owner object",
			ownerObjMeta: testOwnerObjMeta,
			candidateResources: []metav1.PartialObjectMetadata{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "resource1",
						Namespace: testNamespace,
						UID:       uuid.NewUUID(),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PodGangSet",
								Name:       testPgsName,
								UID:        testOwnerObjMeta.UID,
								Controller: ptr.To(true),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "resource2",
						Namespace: testNamespace,
						UID:       uuid.NewUUID(),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PodGangSet",
								Name:       "other-pgs",
								UID:        uuid.NewUUID(),
								Controller: ptr.To(true),
							},
						},
					},
				},
			},
			expectedResourceNames: []string{"resource1"},
		},
		{
			description:  "All resources are owned by the owner object",
			ownerObjMeta: testOwnerObjMeta,
			candidateResources: []metav1.PartialObjectMetadata{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "resource1",
						Namespace: testNamespace,
						UID:       uuid.NewUUID(),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PodGangSet",
								Name:       testPgsName,
								UID:        testOwnerObjMeta.UID,
								Controller: ptr.To(true),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "resource2",
						Namespace: testNamespace,
						UID:       uuid.NewUUID(),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PodGangSet",
								Name:       testPgsName,
								UID:        testOwnerObjMeta.UID,
								Controller: ptr.To(true),
							},
						},
					},
				},
			},
			expectedResourceNames: []string{"resource1", "resource2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			resourceNames := FilterMapOwnedResourceNames(tc.ownerObjMeta, tc.candidateResources)
			assert.Equal(t, tc.expectedResourceNames, resourceNames)
		})
	}
}
