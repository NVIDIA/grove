package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ListExistingPartialObjectMetadata gets the PartialObjectMetadata for a GVK in a given namespace and matching labels.
func ListExistingPartialObjectMetadata(ctx context.Context, cl client.Client, gvk schema.GroupVersionKind, ownerObjMeta metav1.ObjectMeta, selectorLabels map[string]string) ([]metav1.PartialObjectMetadata, error) {
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(gvk)
	if err := cl.List(ctx,
		objMetaList,
		client.InNamespace(ownerObjMeta.Namespace),
		client.MatchingLabels(selectorLabels),
	); err != nil {
		return nil, err
	}
	return objMetaList.Items, nil
}
