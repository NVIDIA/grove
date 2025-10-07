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

package authorization

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	apiconstants "github.com/NVIDIA/grove/operator/api/common/constants"
	groveconfigv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// allowedOperations are operations that will always be allowed irrespective of who is making this call.
// If the user who invokes Create/Connect have the required RBAC they can invoke these operations and this webhook
// will not disallow it.
var allowedOperations = []admissionv1.Operation{admissionv1.Connect}

// Handler is the PodCliqueSet authorization admission webhook handler.
type Handler struct {
	config  groveconfigv1alpha1.AuthorizerConfig
	client  client.Client
	decoder *requestDecoder
	logger  logr.Logger
}

// NewHandler creates a new handler for the auhtorizer webhook.
func NewHandler(mgr manager.Manager, config groveconfigv1alpha1.AuthorizerConfig) *Handler {
	return &Handler{
		config:  config,
		client:  mgr.GetClient(),
		decoder: newRequestDecoder(mgr),
		logger:  mgr.GetLogger().WithName("authorizer-webhook").WithName(Name),
	}
}

// Handle handles requests and admits them if they are authorized.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := h.logger.WithValues("user", req.UserInfo.Username, "operation", req.Operation, "resource", req.Resource, "subresource", req.SubResource, "name", req.Name, "namespace", req.Namespace)
	log.Info("Authorizer webhook invoked")
	// always allow 'connect' operations, irrespective of the user.
	if slices.Contains(allowedOperations, req.Operation) {
		log.Info("Connect operation requested, which is always allowed. Admitting.")
		return admission.Allowed(fmt.Sprintf("operation %s is allowed", req.Operation))
	}

	// Scale is only created for the podcliques and podcliquescalinggroups resources.
	if req.Kind.Kind == "Scale" {
		return h.handleScaleSubresource(req)
	}

	// Decode and convert the request object to `metav1.PartialObjectMeta`.
	resourcePartialObjMeta, err := h.decoder.decode(log, req)
	if err != nil {
		return admission.Errored(toAdmissionError(err), err)
	}
	resourceObjectKey := client.ObjectKeyFromObject(resourcePartialObjMeta)

	// Get the parent PodCliqueSet
	pcs, warnings, err := h.getParentPodCliqueSet(ctx, resourcePartialObjMeta.ObjectMeta)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err).WithWarnings(warnings...)
	}
	// PCS can be nil if the resource has no reference of a PodCliqueSet or the referenced PodCliqueSet is not found.
	if pcs == nil {
		return admission.Allowed(fmt.Sprintf("admission allowed, no PodCliqueSet could be determined for resource: %v", resourceObjectKey)).WithWarnings(warnings...)
	}

	// check if protection of PCS managed resources has been disabled
	if metav1.HasAnnotation(pcs.ObjectMeta, apiconstants.AnnotationDisableManagedResourceProtection) {
		log.Info("Resource has the \"grove.io/disable-managed-resource-protection\" annotation, authorized webhook bypassed. Admitting request.", "objectKey", resourceObjectKey)
		return admission.Allowed(fmt.Sprintf("admission allowed, resource protection is disabled for PodCliqueSet: %v", client.ObjectKeyFromObject(pcs)))
	}

	if req.Operation == admissionv1.Delete {
		return h.handleDelete(req, resourceObjectKey)
	}

	return h.handleUpdate(req.UserInfo, resourceObjectKey)
}

func (h *Handler) handleScaleSubresource(req admission.Request) admission.Response {
	if slices.Contains(h.config.ExemptServiceAccountUserNames, req.UserInfo.Username) {
		return admission.Allowed(fmt.Sprintf("Scale subresource for resource: %v is authorized to be modified.", req.Resource))
	}
	return admission.Denied(fmt.Sprintf("Scale subresource for resource: %v is not authorized to be modified.", req.Resource))
}

// handleDelete allows deletion of managed resources only if the request is either from the service account used by the reconcilers
// or the request is coming from one of the exempted service accounts.
func (h *Handler) handleDelete(req admission.Request, resourceObjectKey client.ObjectKey) admission.Response {
	if req.Kind.Kind == "Pod" {
		return admission.Allowed("admission allowed, deletion of resource: Pod is allowed for all users")
	}
	if req.UserInfo.Username == h.config.ReconcilerServiceAccountUserName {
		return admission.Allowed(fmt.Sprintf("admission allowed, deletion of resource: %v is initiated by the grove reconciler service account", resourceObjectKey))
	}
	if slices.Contains(h.config.ExemptServiceAccountUserNames, req.UserInfo.Username) {
		return admission.Allowed(fmt.Sprintf("admission allowed, deletion of resource: %v is initiated by exempt user account", resourceObjectKey))
	}

	return admission.Denied(fmt.Sprintf("admission denied, deletion of resource: %v is not allowed", resourceObjectKey))
}

func (h *Handler) handleUpdate(userInfo authenticationv1.UserInfo, resourceObjectKey client.ObjectKey) admission.Response {
	if userInfo.Username == h.config.ReconcilerServiceAccountUserName {
		return admission.Allowed(fmt.Sprintf("admission allowed, updation of resource: %v is initiated by the grove reconciler service account", resourceObjectKey))
	}
	if slices.Contains(h.config.ExemptServiceAccountUserNames, userInfo.Username) {
		return admission.Allowed(fmt.Sprintf("admission allowed, updation of resource: %v is initiated by exempt user account", resourceObjectKey))
	}

	return admission.Denied(fmt.Sprintf("admission denied: updation of resource: %v is not allowed", resourceObjectKey))
}

func (h *Handler) getParentPodCliqueSet(ctx context.Context, resourceObjMeta metav1.ObjectMeta) (*grovecorev1alpha1.PodCliqueSet, admission.Warnings, error) {
	resourceObjKey := k8sutils.GetObjectKeyFromObjectMeta(resourceObjMeta)
	pcsName, ok := resourceObjMeta.Labels[apicommon.LabelPartOfKey]
	if !ok {
		return nil, admission.Warnings{fmt.Sprintf("missing required label %s on resource %v, could not determine parent PodCliqueSet", apicommon.LabelPartOfKey, resourceObjKey)}, nil
	}
	pcs := &grovecorev1alpha1.PodCliqueSet{}
	if err := h.client.Get(ctx, client.ObjectKey{Name: pcsName, Namespace: resourceObjMeta.Namespace}, pcs); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, admission.Warnings{fmt.Sprintf("parent PodCliqueSet %s not found for resource: %v", pcsName, resourceObjKey)}, nil
		}
		return nil, nil, err
	}
	return pcs, nil, nil
}

func toAdmissionError(err error) int32 {
	if errors.Is(err, errDecodeRequestObject) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
