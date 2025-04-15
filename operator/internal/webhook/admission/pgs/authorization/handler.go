package authorization

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Handler is the PodGangSet authorization admission webhook handler.
type Handler struct {
	Logger  logr.Logger
	Decoder admission.Decoder
}

// Handle handles requests and admits them if they are authorized.
func (h *Handler) Handle(_ context.Context, _ admission.Request) admission.Response {
	return admission.Response{}
}
