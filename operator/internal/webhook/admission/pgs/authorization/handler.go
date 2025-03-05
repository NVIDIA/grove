package authorization

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Handler struct {
	Logger  logr.Logger
	Decoder admission.Decoder
}
