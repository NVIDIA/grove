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
	"errors"
	"fmt"
	"strconv"
	"strings"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errListPodClique   grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUE"
	errSyncPodClique   grovecorev1alpha1.ErrorCode = "ERR_SYNC_PODCLIQUE"
	errDeletePodClique grovecorev1alpha1.ErrorCode = "ERR_DELETE_PODCLIQUE"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of PodClique component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the PodClique Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) ([]string, error) {
	logger.Info("Looking for existing PodCliques for PodGangSet", "objectKey", client.ObjectKeyFromObject(pgs))
	existingPclqNames := make([]string, 0, int(pgs.Spec.Replicas)*len(pgs.Spec.TemplateSpec.Cliques))
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(grovecorev1alpha1.SchemeGroupVersion.WithKind("PodClique"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pgs.Namespace),
		client.MatchingLabels(getPodCliqueSelectorLabels(pgs.ObjectMeta)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errListPodClique,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodCliques for PodGangSet: %v", client.ObjectKeyFromObject(pgs)),
		)
	}
	for _, pclqObjMeta := range objMetaList.Items {
		if metav1.IsControlledBy(&pclqObjMeta, &pgs.ObjectMeta) {
			existingPclqNames = append(existingPclqNames, pclqObjMeta.Name)
		}
	}
	return existingPclqNames, nil
}

// Sync synchronizes all resources that the PodClique Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) error {
	numTasks := int(pgs.Spec.Replicas) * len(pgs.Spec.TemplateSpec.Cliques)
	tasks := make([]utils.Task, 0, numTasks)

	for replicaIndex := range pgs.Spec.Replicas {
		podGangName := grovecorev1alpha1.GeneratePodGangName(pgs.Name, replicaIndex)
		for _, pclqTemplateSpec := range pgs.Spec.TemplateSpec.Cliques {
			pclqObjectKey := client.ObjectKey{
				Name:      grovecorev1alpha1.GeneratePodCliqueName(pgs.Name, replicaIndex, pclqTemplateSpec.Name),
				Namespace: pgs.Namespace,
			}
			createOrUpdateTask := utils.Task{
				Name: fmt.Sprintf("CreateOrUpdatePodClique-%s", pclqObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreateOrUpdate(ctx, logger, pgs, pclqObjectKey, podGangName)
				},
			}
			tasks = append(tasks, createOrUpdateTask)
		}
	}
	if errs := utils.RunConcurrently(ctx, tasks); len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjectMeta metav1.ObjectMeta) error {
	logger.Info("Triggering deletion of PodClique", "objectKey", client.ObjectKey{Name: pgsObjectMeta.Name, Namespace: pgsObjectMeta.Namespace})
	if err := r.client.DeleteAllOf(ctx,
		&grovecorev1alpha1.PodClique{},
		client.InNamespace(pgsObjectMeta.Namespace),
		client.MatchingLabels(getPodCliqueSelectorLabels(pgsObjectMeta))); err != nil {
		return groveerr.WrapError(err,
			errDeletePodClique,
			component.OperationDelete,
			fmt.Sprintf("Failed to delete PodCliques for PodGangSet: %v", client.ObjectKey{Name: pgsObjectMeta.Name, Namespace: pgsObjectMeta.Namespace}),
		)
	}
	logger.Info("Deleted PodClique", "name", pgsObjectMeta.Name)
	return nil
}

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pclqObjectKey client.ObjectKey, podGangName string) error {
	logger.Info("Running CreateOrUpdate PodClique", "pclqObjectKey", pclqObjectKey)
	pclq := emptyPodClique(pclqObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pclq, func() error {
		return r.buildResource(logger, pclq, pgs, podGangName)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error syncing PodClique: %v for PodGangSet: %v", pclqObjectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("triggered create or update of PodClique", "pclqObjectKey", pclqObjectKey, "result", opResult)
	return nil
}

func (r _resource) buildResource(logger logr.Logger, pclq *grovecorev1alpha1.PodClique, pgs *grovecorev1alpha1.PodGangSet, podGangName string) error {
	var err error
	pclqObjectKey, pgsObjectKey := client.ObjectKeyFromObject(pclq), client.ObjectKeyFromObject(pgs)
	pclqTemplateSpec := findPodCliqueTemplateSpec(pclqObjectKey, pgs)
	if pclqTemplateSpec == nil {
		logger.Info("PodClique template spec not found in PodGangSet", "podCliqueObjectKey", pclqObjectKey, "podGangSetObjectKey", pgsObjectKey)
		return groveerr.New(errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("PodCliqueTemplateSpec for PodClique: %v not found in PodGangSet: %v", pclqObjectKey, pgsObjectKey),
		)
	}
	// Set PodClique.ObjectMeta
	// ------------------------------------
	if err = controllerutil.SetControllerReference(pgs, pclq, r.scheme); err != nil {
		return err
	}
	pclq.Labels = getLabels(pgs.Name, pclqObjectKey, pclqTemplateSpec, podGangName)
	pclq.Annotations = pclqTemplateSpec.Annotations
	// set PodCliqueSpec
	// ------------------------------------
	pclq.Spec = pclqTemplateSpec.Spec
	var dependentPclqNames []string
	if dependentPclqNames, err = identifyStartupDeps(pclq, pgs); err != nil {
		return err
	}
	pclq.Spec.StartsAfter = dependentPclqNames
	return nil
}

func identifyStartupDeps(pclq *grovecorev1alpha1.PodClique, pgs *grovecorev1alpha1.PodGangSet) ([]string, error) {
	cliqueStartupType := pgs.Spec.TemplateSpec.StartupType
	if cliqueStartupType == nil {
		// Ideally this should never happen as the defaulting webhook should set it v1alpha1.CliqueStartupTypeInOrder as the default value.
		// If it is still nil, then by not returning an error we break the API contract. It is a bug that should be fixed.
		return nil, groveerr.New(errSyncPodClique, component.OperationSync, fmt.Sprintf("PodClique: %v has nil StartupType", client.ObjectKeyFromObject(pclq)))
	}

	pclqTemplateSpec, foundAtIndex, ok := lo.FindIndexOf(pgs.Spec.TemplateSpec.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
		return pclqTemplateSpec.Name == extractPodCliqueTemplateName(pclq.Name)
	})
	// this should ideally never happen that matching PodCliqueTemplateSpec is not found. If it happens then it is a bug as
	// this should have been checked in the validating webhook. Therefore, we should return an error.
	if !ok {
		return nil, groveerr.New(errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Could not find PodCliqueTemplateSpec for PodClique: %v in PodGangSet: %v while setting startsAfter", pclq.Name, client.ObjectKeyFromObject(pgs)),
		)
	}

	pclqNameTuple, err := getPodCliqueNameParts(pclq.Name)
	if err != nil {
		return nil, groveerr.WrapError(err,
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Failed to extract parts from a fully qualified name: %s", pclq.Name),
		)
	}

	switch *cliqueStartupType {
	case grovecorev1alpha1.CliqueStartupTypeInOrder:
		return getInOrderStartupDependencies(pclqNameTuple, foundAtIndex), nil
	case grovecorev1alpha1.CliqueStartupTypeExplicit:
		return getExplicitStartupDependencies(pclqNameTuple, pclqTemplateSpec), nil
	default:
		return nil, nil
	}
}

func getInOrderStartupDependencies(pclqNameTuple lo.Tuple3[string, int, string], foundAtIndex int) []string {
	if foundAtIndex == 0 {
		return []string{}
	}
	// get the name of the previous PodCliqueTemplateSpec
	previousPCLQName := grovecorev1alpha1.GeneratePodCliqueName(pclqNameTuple.A, int32(pclqNameTuple.B-1), pclqNameTuple.C)
	return []string{previousPCLQName}
}

func getExplicitStartupDependencies(pclqNameTuple lo.Tuple3[string, int, string], pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) []string {
	dependencies := make([]string, 0, len(pclqTemplateSpec.Spec.StartsAfter))
	pgsName := pclqNameTuple.A
	replicaIndex := pclqNameTuple.B
	for _, dependency := range pclqTemplateSpec.Spec.StartsAfter {
		dependencies = append(dependencies, grovecorev1alpha1.GeneratePodCliqueName(pgsName, int32(replicaIndex), dependency))
	}
	return dependencies
}

func getPodCliqueSelectorLabels(pgsObjectMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjectMeta.Name),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey: component.NamePodClique,
		},
	)
}

func getLabels(pgsName string, pclqObjectKey client.ObjectKey, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, podGangName string) map[string]string {
	pclqComponentLabels := map[string]string{
		grovecorev1alpha1.LabelAppNameKey:     pclqObjectKey.Name,
		grovecorev1alpha1.LabelComponentKey:   component.NamePodClique,
		grovecorev1alpha1.LabelPodGangNameKey: podGangName,
	}
	return lo.Assign(
		pclqTemplateSpec.Labels,
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		pclqComponentLabels,
	)
}

func findPodCliqueTemplateSpec(pclqObjectKey client.ObjectKey, pgs *grovecorev1alpha1.PodGangSet) *grovecorev1alpha1.PodCliqueTemplateSpec {
	pclqTemplateName := extractPodCliqueTemplateName(pclqObjectKey.Name)
	foundPCLQTemplate, ok := lo.Find(pgs.Spec.TemplateSpec.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
		return pclqTemplateName == pclqTemplateSpec.Name
	})
	return lo.Ternary(ok, foundPCLQTemplate, nil)
}

// extractPodCliqueName extracts the PodClique name from the qualified PodClique name.
// A fully qualified PodClique name is made of 3 parts: <PGS.Name>-<PGS.ReplicaIndex>-<PCLQ.Name>
func extractPodCliqueTemplateName(qualifiedPCLQName string) string {
	parts := strings.Split(qualifiedPCLQName, "-")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}

func getPodCliqueNameParts(qualifiedPCLQName string) (lo.Tuple3[string, int, string], error) {
	parts := strings.Split(qualifiedPCLQName, "-")
	errTuple := lo.T3("", -1, "")
	if len(parts) != 3 {
		return errTuple, fmt.Errorf("invalid PodClique name. Failed to extract parts from a fully qualified name: %s", qualifiedPCLQName)
	}
	replicaIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return errTuple, err
	}
	return lo.T3[string, int, string](parts[0], replicaIndex, parts[2]), nil
}

func emptyPodClique(objKey client.ObjectKey) *grovecorev1alpha1.PodClique {
	return &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
