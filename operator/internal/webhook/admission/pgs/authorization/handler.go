package authorization

import (
	"context"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Handler struct {
	Logger  logr.Logger
	Decoder admission.Decoder
}

func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	return admission.Response{}
}
