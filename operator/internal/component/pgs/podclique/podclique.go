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
	numTasks := int(pgs.Spec.Replicas) * len(pgs.Spec.TemplateSpec.Cliques)
	tasks := make([]utils.Task, 0, numTasks)

	for replicaIndex := range pgs.Spec.Replicas {
		podGangName := utils.GeneratePodGangName(pgs.Name, replicaIndex)
		for _, pclqTemplateSpec := range pgs.Spec.TemplateSpec.Cliques {
			pclqObjectKey := client.ObjectKey{
				Name:      createPodCliqueName(pgs.Name, replicaIndex, pclqTemplateSpec.Name),
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
		&v1alpha1.PodClique{},
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

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet, pclqObjectKey client.ObjectKey, podGangName string) error {
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

func (r _resource) buildResource(logger logr.Logger, pclq *v1alpha1.PodClique, pgs *v1alpha1.PodGangSet, podGangName string) error {
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
	if err := controllerutil.SetControllerReference(pgs, pclq, r.scheme); err != nil {
		return err
	}
	pclq.ObjectMeta.Labels = getLabels(pgs.Name, pclqObjectKey, pclqTemplateSpec, podGangName)
	pclq.ObjectMeta.Annotations = pclqTemplateSpec.Annotations
	// set PodCliqueSpec
	// ------------------------------------
	pclq.Spec = pclqTemplateSpec.Spec
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

func getLabels(pgsName string, pclqObjectKey client.ObjectKey, pclqTemplateSpec *v1alpha1.PodCliqueTemplateSpec, podGangName string) map[string]string {
	pclqComponentLabels := map[string]string{
		v1alpha1.LabelAppNameKey:     pclqObjectKey.Name,
		v1alpha1.LabelComponentKey:   component.NamePodClique,
		v1alpha1.LabelPodGangNameKey: podGangName,
	}
	return lo.Assign(
		pclqTemplateSpec.Labels,
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		pclqComponentLabels,
	)
}

func findPodCliqueTemplateSpec(pclqObjectKey client.ObjectKey, pgs *v1alpha1.PodGangSet) *v1alpha1.PodCliqueTemplateSpec {
	for replicaIndex := range pgs.Spec.Replicas {
		for _, pclqTemplate := range pgs.Spec.TemplateSpec.Cliques {
			if createPodCliqueName(pgs.Name, replicaIndex, pclqTemplate.Name) == pclqObjectKey.Name {
				return pclqTemplate
			}
		}
	}
	return nil
}

// createPodCliqueName creates a PodClique name based on the PodGangSet name, replica index, and PodClique name.
// PodCliqueName convention is <PGS.Name>-<PGS.ReplicaIndex>-<PCLQ.Name>
func createPodCliqueName(prefix string, replicaIndex int32, suffix string) string {
	return fmt.Sprintf("%s-%d-%s", prefix, replicaIndex, suffix)
}

func emptyPodClique(objKey client.ObjectKey) *v1alpha1.PodClique {
	return &v1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
