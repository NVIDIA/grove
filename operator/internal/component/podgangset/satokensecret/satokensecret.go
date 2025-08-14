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

package satokensecret

import (
	"context"
	"fmt"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errCodeGetSecret              grovecorev1alpha1.ErrorCode = "ERR_GET_SECRET"
	errCodeSetControllerReference grovecorev1alpha1.ErrorCode = "ERR_SET_CONTROLLER_REFERENCE"
	errCodeCreateSecret           grovecorev1alpha1.ErrorCode = "ERR_CREATE_SECRET"
	errCodeDeleteSecret           grovecorev1alpha1.ErrorCode = "ERR_DELETE_SECRET"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of Secret component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pgsObjMeta metav1.ObjectMeta) ([]string, error) {
	secretNames := make([]string, 0, 1)
	objKey := getObjectKey(pgsObjMeta)
	partialObjMeta, err := k8sutils.GetExistingPartialObjectMetadata(ctx, r.client, corev1.SchemeGroupVersion.WithKind("Secret"), objKey)
	if err != nil {
		if errors.IsNotFound(err) {
			return secretNames, nil
		}
		return nil, groveerr.WrapError(err,
			errCodeGetSecret,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error getting Secret: %v for PodGangSet: %v", objKey, k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	if metav1.IsControlledBy(partialObjMeta, &pgsObjMeta) {
		secretNames = append(secretNames, partialObjMeta.Name)
	}
	return secretNames, nil
}

func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) error {
	pgsObjKey := client.ObjectKeyFromObject(pgs)
	existingSecretNames, err := r.GetExistingResourceNames(ctx, logger, pgs.ObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errCodeGetSecret,
			component.OperationSync,
			fmt.Sprintf("Error getting existing satokensecret names for PodGangSet: %v", pgsObjKey),
		)
	}
	if len(existingSecretNames) > 0 {
		logger.Info("Secret already exists, skipping creation", "existingSecret", existingSecretNames[0])
		return nil
	}
	objKey := getObjectKey(pgs.ObjectMeta)
	secret := emptySecret(objKey)
	if err = r.buildResource(pgs, secret); err != nil {
		return err
	}
	if err = client.IgnoreAlreadyExists(r.client.Create(ctx, secret)); err != nil {
		return groveerr.WrapError(err,
			errCodeCreateSecret,
			component.OperationSync,
			fmt.Sprintf("Error creating satokensecret: %v for PodGangSet: %v", objKey, pgsObjKey),
		)
	}
	logger.Info("Created Secret", "objectKey", objKey)
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta) error {
	objectKey := getObjectKey(pgsObjMeta)
	logger.Info("Triggering delete of Secret", "objectKey", objectKey)
	if err := r.client.Delete(ctx, emptySecret(objectKey)); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Secret not found", "objectKey", objectKey)
			return nil
		}
		return groveerr.WrapError(err,
			errCodeDeleteSecret,
			component.OperationDelete,
			fmt.Sprintf("Error deleting satokensecret: %v for PodGangSet: %v", objectKey, k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	logger.Info("Deleted Secret", "objectKey", objectKey)
	return nil
}

func (r _resource) buildResource(pgs *grovecorev1alpha1.PodGangSet, secret *corev1.Secret) error {
	secret.Labels = getLabels(pgs.Name, secret.Name)
	if err := controllerutil.SetControllerReference(pgs, secret, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errCodeSetControllerReference,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for satokensecret: %v", client.ObjectKeyFromObject(secret)),
		)
	}
	secret.Type = corev1.SecretTypeServiceAccountToken
	secret.Annotations = map[string]string{
		corev1.ServiceAccountNameKey: grovecorev1alpha1.GeneratePodServiceAccountName(pgs.Name),
	}
	return nil
}

func getLabels(pgsName, secretName string) map[string]string {
	secretLabels := map[string]string{
		grovecorev1alpha1.LabelComponentKey: component.NameServiceAccountTokenSecret,
		grovecorev1alpha1.LabelAppNameKey:   secretName,
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		secretLabels,
	)
}

func getObjectKey(pgsObjMeta metav1.ObjectMeta) client.ObjectKey {
	return client.ObjectKey{
		Name:      grovecorev1alpha1.GenerateInitContainerSATokenSecretName(pgsObjMeta.Name),
		Namespace: pgsObjMeta.Namespace,
	}
}

func emptySecret(objKey client.ObjectKey) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
