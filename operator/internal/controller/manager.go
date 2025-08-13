// /*
// Copyright 2024 The Grove Authors.
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
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	groveclientscheme "github.com/NVIDIA/grove/operator/internal/client"
	"github.com/NVIDIA/grove/operator/internal/webhook"

	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmetricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	pprofBindAddress      = "127.0.0.1:2753"
	operatorNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// CreateManager creates the manager.
func CreateManager(operatorCfg *configv1alpha1.OperatorConfiguration) (ctrl.Manager, error) {
	return ctrl.NewManager(getRestConfig(operatorCfg), createManagerOptions(operatorCfg))
}

// InitializeManager adds all the controllers and webhooks to the controller-manager using the passed in Config.
func InitializeManager(mgr ctrl.Manager, operatorCfg *configv1alpha1.OperatorConfiguration) error {
	if operatorCfg.Server.InternalCertificateManagement != nil && *operatorCfg.Server.InternalCertificateManagement.Enabled {
		doneCh := make(chan struct{})
		namespace, err := getOperatorNamespace()
		if err != nil {
			return err
		}
		if err = RegisterCertificateManager(mgr, namespace, operatorCfg.Server.Webhooks.ServerCertDir, doneCh); err != nil {
			return err
		}
		// block here since the webhooks can not start without certificates
		<-doneCh
	}

	if err := RegisterControllers(mgr, operatorCfg.Controllers); err != nil {
		return err
	}
	if err := webhook.RegisterWebhooks(mgr); err != nil {
		return err
	}
	// TODO register readyz, healthz endpoints
	return nil
}

// SetupHealthAndReadinessEndpoints sets up the health and readiness endpoints for the operator.
func SetupHealthAndReadinessEndpoints(mgr ctrl.Manager) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}
	return nil
}

func createManagerOptions(operatorCfg *configv1alpha1.OperatorConfiguration) ctrl.Options {
	opts := ctrl.Options{
		Scheme:                  groveclientscheme.Scheme,
		GracefulShutdownTimeout: ptr.To(5 * time.Second),
		Metrics: ctrlmetricsserver.Options{
			BindAddress: net.JoinHostPort(operatorCfg.Server.Metrics.BindAddress, strconv.Itoa(operatorCfg.Server.Metrics.Port)),
		},
		HealthProbeBindAddress:        net.JoinHostPort(operatorCfg.Server.HealthProbes.BindAddress, strconv.Itoa(operatorCfg.Server.HealthProbes.Port)),
		LeaderElection:                operatorCfg.LeaderElection.Enabled,
		LeaderElectionID:              operatorCfg.LeaderElection.ResourceName,
		LeaderElectionResourceLock:    operatorCfg.LeaderElection.ResourceLock,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &operatorCfg.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:                 &operatorCfg.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                   &operatorCfg.LeaderElection.RetryPeriod.Duration,
		Controller: ctrlconfig.Controller{
			RecoverPanic: ptr.To(true),
		},
		WebhookServer: ctrlwebhook.NewServer(ctrlwebhook.Options{
			Host:    operatorCfg.Server.Webhooks.BindAddress,
			Port:    operatorCfg.Server.Webhooks.Port,
			CertDir: operatorCfg.Server.Webhooks.ServerCertDir,
		}),
	}
	if operatorCfg.Debugging != nil {
		if operatorCfg.Debugging.EnableProfiling != nil &&
			*operatorCfg.Debugging.EnableProfiling {
			opts.PprofBindAddress = pprofBindAddress
		}
	}
	return opts
}

func getRestConfig(operatorCfg *configv1alpha1.OperatorConfiguration) *rest.Config {
	restCfg := ctrl.GetConfigOrDie()
	if operatorCfg != nil {
		restCfg.Burst = operatorCfg.ClientConnection.Burst
		restCfg.QPS = operatorCfg.ClientConnection.QPS
		restCfg.AcceptContentTypes = operatorCfg.ClientConnection.AcceptContentTypes
		restCfg.ContentType = operatorCfg.ClientConnection.ContentType
	}
	return restCfg
}

func getOperatorNamespace() (string, error) {
	data, err := os.ReadFile(operatorNamespaceFile)
	if err != nil {
		return "", err
	}
	namespace := strings.TrimSpace(string(data))
	if len(namespace) == 0 {
		return "", fmt.Errorf("operator namespace is empty")
	}
	return namespace, nil
}
