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

package framework

import (
	"fmt"
	"strings"

	grovecrds "github.com/NVIDIA/grove/operator/api/core/v1alpha1/crds"

	schedulercrds "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1/crds"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func getOperatorCRDs() ([]*apiextensionsv1.CustomResourceDefinition, error) {
	crdContents := []string{
		grovecrds.PodGangSetCRD(),
		grovecrds.PodCliqueCRD(),
		grovecrds.PodCliqueScalingGroupCRD(),
	}

	var crds []*apiextensionsv1.CustomResourceDefinition
	for _, crdContent := range crdContents {
		crd, err := parseCRDFromYAML(crdContent)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CRD: %w", err)
		}
		crds = append(crds, crd)
	}

	return crds, nil
}

func getSchedulerCRDs() ([]*apiextensionsv1.CustomResourceDefinition, error) {
	crdContents := []string{
		schedulercrds.PodGangCRD(),
	}

	var crds []*apiextensionsv1.CustomResourceDefinition
	for _, crdContent := range crdContents {
		crd, err := parseCRDFromYAML(crdContent)
		if err != nil {
			return nil, fmt.Errorf("failed to parse scheduler CRD: %w", err)
		}
		crds = append(crds, crd)
	}

	return crds, nil
}

// parseCRDFromYAML parses a CRD from YAML content
func parseCRDFromYAML(yamlContent string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}

	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(yamlContent), 4096)
	if err := decoder.Decode(crd); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	return crd, nil
}
