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

package opts

import (
	"flag"
	"strings"

	"github.com/NVIDIA/grove/operator/internal/version"
)

// CLIOptions defines the configuration that is passed to the init container
type CLIOptions struct {
	// podCliqueFQNs stores comma seperated parent fully qualified PodClique names.
	podCliqueFQNs string
	// podCliqueNamespace contains the namespace that the parent PodCliques are present in.
	podCliqueNamespace string
}

// RegisterFlags registers all the flags that are defined for the init container
func (c *CLIOptions) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.podCliqueFQNs, "pod-cliques", "", "comma seperated namespaced names of PodCliques that the init container should wait for to be ready")
	fs.StringVar(&c.podCliqueNamespace, "pod-clique-namespace", "default", "namespace that the PodClique are deployed in")
	version.AddFlags(fs)
}

// PodCliqueNames returns a slice of PodClique names passed as the argument
func (c *CLIOptions) PodCliqueNames() []string {
	var podCliquesNames []string
	for cliqueFQN := range strings.SplitSeq(c.podCliqueFQNs, ",") {
		trimmedCliqueFQN := strings.TrimSpace(cliqueFQN)
		if trimmedCliqueFQN != "" {
			podCliquesNames = append(podCliquesNames, trimmedCliqueFQN)
		}
	}
	return podCliquesNames
}

func (c *CLIOptions) PodCliqueNamespace() string {
	return c.podCliqueNamespace
}

func InitializeCLIOptions() (CLIOptions, error) {
	config := CLIOptions{}
	flagSet := flag.CommandLine

	config.RegisterFlags(flagSet)
	flag.Parse()

	return config, nil
}
