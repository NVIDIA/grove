package authorization

import (
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// Name is name of the authorizer webhook for all managed resources for a PCS managed by Grove operator.
	Name = "authorizer-webhook"
	// webhookPath is the path which the webhook handler is registered.
	webhookPath = "/webhooks/authorizer-webhook"
)

// RegisterWithManager registers the authorizer webhook handler with the manager.
func (h *Handler) RegisterWithManager(mgr manager.Manager) error {
	webhook := admission.Webhook{
		Handler:      h,
		RecoverPanic: ptr.To(true),
	}
	mgr.GetWebhookServer().Register(webhookPath, &webhook)
	return nil
}
