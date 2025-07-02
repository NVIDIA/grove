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

package opts

import (
	"fmt"
	"os"

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	operatorvalidation "github.com/NVIDIA/grove/operator/api/config/validation"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var configDecoder runtime.Decoder

func init() {
	configScheme := runtime.NewScheme()
	utilruntime.Must(configv1alpha1.AddToScheme(configScheme))
	configDecoder = serializer.NewCodecFactory(configScheme).UniversalDecoder()
}

// CLIOptions provides convenience abstraction to initialize and validate OperatorConfiguration from CLI flags.
type CLIOptions struct {
	configFile string
	// Config is the operator configuration initialized from the CLI flags.
	Config *configv1alpha1.OperatorConfiguration
}

// NewCLIOptions creates a new CLIOptions and adds the required CLI flags to the flag.flagSet.
func NewCLIOptions(fs *pflag.FlagSet) *CLIOptions {
	cliOpts := &CLIOptions{}
	cliOpts.addFlags(fs)
	return cliOpts
}

// Complete reads the configuration file and decodes it into an OperatorConfiguration.
func (o *CLIOptions) Complete() error {
	if len(o.configFile) == 0 {
		return fmt.Errorf("missing config file")
	}
	data, err := os.ReadFile(o.configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}
	o.Config = &configv1alpha1.OperatorConfiguration{}
	if err = runtime.DecodeInto(configDecoder, data, o.Config); err != nil {
		return fmt.Errorf("error decoding config: %w", err)
	}
	return nil
}

// Validate validates the created OperatorConfiguration.
func (o *CLIOptions) Validate() error {
	if errs := operatorvalidation.ValidateOperatorConfiguration(o.Config); errs != nil {
		return errs.ToAggregate()
	}
	return nil
}

func (o *CLIOptions) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.configFile, "config", o.configFile, "Path to configuration file.")
}
