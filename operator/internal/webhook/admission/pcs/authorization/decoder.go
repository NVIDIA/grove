package authorization

import (
	"context"
	"errors"
	"fmt"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/scale/scheme/autoscalingv1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	pcsgGVK               = grovecorev1alpha1.SchemeGroupVersion.WithKind(constants.KindPodCliqueScalingGroup)
	pclqGVK               = grovecorev1alpha1.SchemeGroupVersion.WithKind(constants.KindPodClique)
	podGVK                = corev1.SchemeGroupVersion.WithKind("Pod")
	secretGVK             = corev1.SchemeGroupVersion.WithKind("Secret")
	roleGVK               = rbacv1.SchemeGroupVersion.WithKind("Role")
	roleBindingGVK        = rbacv1.SchemeGroupVersion.WithKind("RoleBinding")
	serviceGVK            = corev1.SchemeGroupVersion.WithKind("Service")
	serviceAccountGVK     = corev1.SchemeGroupVersion.WithKind("ServiceAccount")
	scaleSubResourceV2GVK = autoscalingv2.SchemeGroupVersion.WithKind("Scale")
	scaleSubResourceV1GVK = autoscalingv1.SchemeGroupVersion.WithKind("Scale")
	hpaV2GVK              = autoscalingv2.SchemeGroupVersion.WithKind("HorizontalPodAutoscaler")
	hpaV1GVK              = autoscalingv1.SchemeGroupVersion.WithKind("HorizontalPodAutoscaler")
	podgangGVK            = groveschedulerv1alpha1.SchemeGroupVersion.WithKind("PodGang")

	errDecodeRequestObject  = errors.New("failed to decode request")
	errUnsupportedOperation = errors.New("unsupported operation")
)

// requestDecoder decodes the admission requests.
// It optimizes only gets PartialObjectMetadata for the resource
// as there is no need to get the full resource.
type requestDecoder struct {
	decoder admission.Decoder
	client  client.Client
}

func newRequestDecoder(mgr manager.Manager) *requestDecoder {
	return &requestDecoder{
		decoder: admission.NewDecoder(mgr.GetScheme()),
		client:  mgr.GetClient(),
	}
}

func (d *requestDecoder) decode(ctx context.Context, logger logr.Logger, req admission.Request) (*metav1.PartialObjectMetadata, error) {
	reqGVK := schema.GroupVersionKind{
		Group:   req.Kind.Group,
		Version: req.Kind.Version,
		Kind:    req.Kind.Kind,
	}
	switch reqGVK {
	case pcsgGVK, pclqGVK, podGVK, serviceAccountGVK, serviceGVK, secretGVK, roleGVK, roleBindingGVK, hpaV2GVK, hpaV1GVK, podgangGVK:
		return d.decodeAsPartialObjectMetadata(ctx, req, false)
	case scaleSubResourceV2GVK, scaleSubResourceV1GVK:
		return d.decodeAsPartialObjectMetadata(ctx, req, true)
	default:
		logger.Info("Skipping decoding, unknown GVK", "GVK", reqGVK)
		return nil, nil
	}
}

func (d *requestDecoder) decodeAsPartialObjectMetadata(ctx context.Context, req admission.Request, isScaleSubResource bool) (partialObjMeta *metav1.PartialObjectMetadata, err error) {
	var obj *unstructured.Unstructured
	switch req.Operation {
	case admissionv1.Connect:
		return
	case admissionv1.Create:
		obj, err = d.asUnstructured(req.Object)
		if err != nil {
			return
		}
	case admissionv1.Delete:
		// OldObject contains the object being deleted
		//https://github.com/kubernetes/kubernetes/pull/76346
		obj, err = d.asUnstructured(req.OldObject)
		if err != nil {
			return
		}
	case admissionv1.Update:
		// OldObject is used since labels are used to check if a resource is managed by grove.
		// If these labels are changed in an update, it might cause the authorizer webhook to not work as intended.
		obj, err = d.asUnstructured(req.OldObject)
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("%w: operation %s", errUnsupportedOperation, req.Operation)
		return
	}
	return meta.AsPartialObjectMetadata(obj), nil
}

func (d *requestDecoder) asUnstructured(rawObj runtime.RawExtension) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	if err := d.decoder.DecodeRaw(rawObj, obj); err != nil {
		return nil, fmt.Errorf("%w: %w", errDecodeRequestObject, err)
	}
	return obj, nil
}
