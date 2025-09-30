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

package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

// HelmInstallConfig holds configuration for Helm chart installations.
type HelmInstallConfig struct {
	// RestConfig is the Kubernetes REST configuration. If nil, uses default kubeconfig.
	RestConfig *rest.Config
	// ReleaseName is the name of the Helm release. Required unless GenerateName is true.
	ReleaseName string
	// ChartRef is the chart reference (path or OCI reference). Required.
	ChartRef string
	// ChartVersion is the version of the chart to install. Required.
	ChartVersion string
	// Namespace is the Kubernetes namespace to install into. Required.
	Namespace string
	// CreateNamespace creates the namespace if it doesn't exist.
	CreateNamespace bool
	// Wait blocks until all resources are ready.
	Wait bool
	// GenerateName generates a random release name with ReleaseName as prefix.
	GenerateName bool
	// Values are the chart values to use for the installation.
	Values map[string]interface{}
	// HelmLoggerFunc is called for Helm operation logging.
	HelmLoggerFunc func(format string, v ...interface{})
	// Logger is the full logger for component operations.
	Logger *logrus.Logger
}

// Validate validates and sets defaults for the configuration.
func (c *HelmInstallConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("config cannot be nil")
	}

	var missing []string
	if c.ReleaseName == "" && !c.GenerateName {
		missing = append(missing, "release name (or enable GenerateName)")
	}
	if c.ChartRef == "" {
		missing = append(missing, "chart reference")
	}
	if c.ChartVersion == "" {
		missing = append(missing, "chart version")
	}
	if c.Namespace == "" {
		missing = append(missing, "namespace")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	// Set defaults
	if c.Values == nil {
		c.Values = make(map[string]interface{})
	}
	if c.HelmLoggerFunc == nil {
		c.HelmLoggerFunc = func(_ string, _ ...interface{}) {}
	}
	return nil
}

// InstallHelmChart installs a Helm chart with the given configuration.
func InstallHelmChart(config *HelmInstallConfig) (*release.Release, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	config.HelmLoggerFunc("Setting up Helm and Kubernetes configuration for %s...", config.ReleaseName)
	actionConfig, err := setupHelmAction(config)
	if err != nil {
		return nil, err
	}

	// Create the install client using a helper to keep this function's workflow clean and readable.
	installClient := newInstallClient(actionConfig, config)

	config.HelmLoggerFunc("Locating and pulling chart %s version %s...", config.ChartRef, config.ChartVersion)
	chartPath, err := installClient.LocateChart(config.ChartRef, cli.New())
	if err != nil {
		return nil, fmt.Errorf("failed to locate chart: %w", err)
	}
	config.HelmLoggerFunc("Chart located at: %s", chartPath)

	// Load the chart from the located path.
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %w", err)
	}

	// Run the installation.
	rel, err := installClient.Run(chart, config.Values)
	if err != nil {
		return nil, fmt.Errorf("helm install failed: %w", err)
	}

	config.HelmLoggerFunc("Success! Release '%s' installed in namespace '%s'. Status: %s", rel.Name, rel.Namespace, rel.Info.Status)
	return rel, nil
}

// setupHelmAction sets up Helm action configuration.
func setupHelmAction(config *HelmInstallConfig) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)

	// Create a RESTClientGetter that can handle both a custom rest.Config and the default kubeconfig path.
	restClientGetter := newRESTClientGetter(config.Namespace, config.RestConfig)

	// Initialize the action configuration with the REST client, namespace, driver, and logger.
	if err := actionConfig.Init(restClientGetter, config.Namespace, os.Getenv("HELM_DRIVER"), config.HelmLoggerFunc); err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action configuration: %w", err)
	}

	// Initialize the OCI registry client for pulling charts from OCI registries.
	regClient, err := registry.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Helm registry client: %w", err)
	}
	actionConfig.RegistryClient = regClient

	return actionConfig, nil
}

// newInstallClient creates and configures a Helm install action client from the provided configuration.
func newInstallClient(actionConfig *action.Configuration, config *HelmInstallConfig) *action.Install {
	// Create a new install action client.
	client := action.NewInstall(actionConfig)

	// Set fields from the config struct.
	client.ReleaseName = config.ReleaseName
	client.GenerateName = config.GenerateName
	client.Namespace = config.Namespace
	client.CreateNamespace = config.CreateNamespace
	client.Wait = config.Wait
	// This single field assignment populates both client.Version and client.ChartPathOptions.Version.
	client.Version = config.ChartVersion

	return client
}

// newRESTClientGetter creates a RESTClientGetter for Helm actions
func newRESTClientGetter(namespace string, restConfig *rest.Config) genericclioptions.RESTClientGetter {
	// Create ConfigFlags with usePersistentConfig=true to reuse discovery clients and REST mappers.
	flags := genericclioptions.NewConfigFlags(true)
	// Set the namespace on the ConfigFlags object.
	flags.Namespace = &namespace

	// WrapConfigFn is the key. It intercepts the config that ConfigFlags would have built (from kubeconfig files)
	// and allows us to substitute our own. This is the idiomatic way to inject a programmatic rest.Config
	// while still getting the benefit of the default discovery/mapper implementations.
	flags.WrapConfigFn = func(c *rest.Config) *rest.Config {
		// If the user provided a specific rest.Config, use it. This is the custom config path.
		if restConfig != nil {
			return restConfig
		}
		// Otherwise, return the config that ConfigFlags built (from default kubeconfig path)
		return c
	}
	return flags
}
