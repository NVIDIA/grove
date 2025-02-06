package kubernetes

import (
	"github.com/NVIDIA/grove/operator/internal/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// CreateObjectKeyForCreateWebhooks creates an object key for an object handled by webhooks registered for CREATE verbs.
func CreateObjectKeyForCreateWebhooks(obj client.Object, req admission.Request) client.ObjectKey {
	namespace := obj.GetNamespace()
	// In webhooks the namespace is not always set in objects due to https://github.com/kubernetes/kubernetes/issues/88282,
	// so try to get the namespace information from the request directly, otherwise the object is presumably not namespaced.
	if utils.IsEmptyStringType(namespace) && len(req.Namespace) != 0 {
		namespace = req.Namespace
	}
	name := obj.GetName()
	if len(name) == 0 {
		name = obj.GetGenerateName()
	}
	return client.ObjectKey{Namespace: namespace, Name: name}
}
