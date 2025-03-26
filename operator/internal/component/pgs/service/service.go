package service

import (
	"context"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type _resource struct {
	client client.Client
}

func New(client client.Client) component.Operator[v1alpha1.PodGangSet] {
	return &_resource{
		client: client,
	}
}

func (_ _resource) Sync(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) error {
	//TODO implement me
	return nil
}

func (_ _resource) Delete(ctx context.Context, logger logr.Logger, objMeta metav1.ObjectMeta) error {
	//TODO implement me
	return nil
}
