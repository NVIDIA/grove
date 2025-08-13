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

package controller

import (
	"fmt"

	"github.com/NVIDIA/grove/operator/internal/webhook/admission/pgs/defaulting"
	"github.com/NVIDIA/grove/operator/internal/webhook/admission/pgs/validation"

	cert "github.com/open-policy-agent/cert-controller/pkg/rotator"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	serviceName                      = "grove-operator"
	certificateAuthorityName         = "grove-operator-ca"
	certificateAuthorityOrganization = "grove-operator"
)

// RegisterCertificateManager registers the cert-controller with the manager.
func RegisterCertificateManager(mgr ctrl.Manager, namespace string, certDir string, doneCh chan struct{}) error {
	rotator := &cert.CertRotator{
		SecretKey: types.NamespacedName{
			Namespace: namespace,
			Name:      "grove-webhook-server-cert",
		},
		CertDir:        certDir,
		CAName:         certificateAuthorityName,
		CAOrganization: certificateAuthorityOrganization,
		IsReady:        doneCh,
		DNSName:        fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
		ExtraDNSNames: []string{
			serviceName,
			fmt.Sprintf("%s.%s", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		},
		Webhooks: []cert.WebhookInfo{
			{
				Type: cert.Mutating,
				Name: defaulting.HandlerName,
			},
			{
				Type: cert.Validating,
				Name: validation.HandlerName,
			},
		},
		RestartOnSecretRefresh: true,
	}
	return cert.AddRotator(mgr, rotator)
}
