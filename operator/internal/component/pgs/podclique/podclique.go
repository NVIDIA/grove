package podclique

import (
	"context"
	"errors"
	"fmt"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
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
	errSyncPodClique   v1alpha1.ErrorCode = "ERR_SYNC_PODCLIQUE"
	errDeletePodClique v1alpha1.ErrorCode = "ERR_DELETE_PODCLIQUE"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of PodClique component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[v1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) error {
	objectKeys := getObjectKeys(pgs)
	tasks := make([]utils.Task, 0, len(objectKeys))
	for _, objectKey := range objectKeys {
		createOrUpdateTask := utils.Task{
			Name: fmt.Sprintf("CreateOrUpdatePodClique-%s", objectKey),
			Fn: func(ctx context.Context) error {
				return r.doCreateOrUpdate(ctx, logger, pgs, objectKey)
			},
		}
		tasks = append(tasks, createOrUpdateTask)
	}
	if errs := utils.RunConcurrently(ctx, tasks); len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjectMeta metav1.ObjectMeta) error {
	logger.Info("Triggering deletion of PodClique", "objectKey", client.ObjectKey{Name: pgsObjectMeta.Name, Namespace: pgsObjectMeta.Namespace})
	if err := r.client.DeleteAllOf(ctx,
		&v1alpha1.PodClique{},
		client.InNamespace(pgsObjectMeta.Namespace),
		client.MatchingLabels(getPodCliqueSelectorLabels(pgsObjectMeta))); err != nil {
		return groveerr.WrapError(err,
			errDeletePodClique,
			component.OperationDelete,
			fmt.Sprintf("Failed to delete PodCliques for PodGangSet: %v", client.ObjectKey{Name: pgsObjectMeta.Name, Namespace: pgsObjectMeta.Namespace}),
		)
	}
	return nil
}

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet, pclqObjectKey client.ObjectKey) error {
	logger.Info("Running CreateOrUpdate PodClique", "pclqObjectKey", pclqObjectKey)
	pclq := emptyPodClique(pclqObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pclq, func() error {
		return r.buildResource(logger, pclq, pgs)
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

func (r _resource) buildResource(logger logr.Logger, pclq *v1alpha1.PodClique, pgs *v1alpha1.PodGangSet) error {
	pclqObjectKey, pgsObjectKey := client.ObjectKeyFromObject(pclq), client.ObjectKeyFromObject(pgs)
	pcTemplateSpec := findPodCliqueTemplateSpec(pclqObjectKey, pgs)
	if pcTemplateSpec == nil {
		logger.Info("PodClique template spec not found in PodGangSet", "podCliqueObjectKey", pclqObjectKey, "podGangSetObjectKey", pgsObjectKey)
		return groveerr.New(errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("PodCliqueTemplateSpec for PodClique: %v not found in PodGangSet: %v", pclqObjectKey, pgsObjectKey),
		)
	}
	// Set PodClique.ObjectMeta
	// ------------------------------------
	if err := controllerutil.SetControllerReference(pgs, pclq, r.scheme); err != nil {
		return err
	}
	pclq.ObjectMeta.Labels = getLabels(pgs.Name, pclqObjectKey, pcTemplateSpec)
	pclq.ObjectMeta.Annotations = pcTemplateSpec.Annotations
	// set PodCliqueSpec
	// ------------------------------------
	pclq.Spec = pcTemplateSpec.Spec
	return nil
}

func getPodCliqueSelectorLabels(pgsObjectMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjectMeta.Name),
		map[string]string{
			v1alpha1.LabelComponentKey: component.NamePodClique,
		},
	)
}

func getLabels(pgsName string, pcObjectKey client.ObjectKey, pcTemplateSpec *v1alpha1.PodCliqueTemplateSpec) map[string]string {
	pcComponentLabels := map[string]string{
		v1alpha1.LabelAppNameKey:   pcObjectKey.Name,
		v1alpha1.LabelComponentKey: component.NamePodClique,
	}
	return lo.Assign(
		pcTemplateSpec.Labels,
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		pcComponentLabels,
	)
}

func findPodCliqueTemplateSpec(pclqObjectKey client.ObjectKey, pgs *v1alpha1.PodGangSet) *v1alpha1.PodCliqueTemplateSpec {
	for _, pclqTemplate := range pgs.Spec.Template.Cliques {
		if createPodCliqueName(pgs.Name, pclqTemplate.Name) == pclqObjectKey.Name {
			return pclqTemplate
		}
	}
	return nil
}

func getObjectKeys(pgs *v1alpha1.PodGangSet) []client.ObjectKey {
	pcNames := getPodCliqueNames(pgs)
	pcObjKeys := make([]client.ObjectKey, 0, len(pcNames))
	for _, pcName := range pcNames {
		pcObjKeys = append(pcObjKeys, client.ObjectKey{
			Name:      pcName,
			Namespace: pgs.Namespace,
		})
	}
	return pcObjKeys
}

func getPodCliqueNames(pgs *v1alpha1.PodGangSet) []string {
	pcPrefix := pgs.Name
	pcNames := make([]string, 0, len(pgs.Spec.Template.Cliques))
	for _, pcTemplateSpec := range pgs.Spec.Template.Cliques {
		pcNames = append(pcNames, createPodCliqueName(pcPrefix, pcTemplateSpec.Name))
	}
	return pcNames
}

func createPodCliqueName(prefix, suffix string) string {
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

func emptyPodClique(objKey client.ObjectKey) *v1alpha1.PodClique {
	return &v1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
