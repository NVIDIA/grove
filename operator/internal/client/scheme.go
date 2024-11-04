// /*
// Copyright 2024.
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

package client

import (
	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	podgangsetv1alpha1 "github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// Scheme is the kubernetes runtime scheme
var Scheme = runtime.NewScheme()

func init() {
	localSchemeBuilder := runtime.NewSchemeBuilder(
		configv1alpha1.AddToScheme,
		podgangsetv1alpha1.AddToScheme,
	)
	utilruntime.Must(localSchemeBuilder.AddToScheme(Scheme))
}
