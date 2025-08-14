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

package common

const (
	// PodGangNameFileName is the name of the file that contains the PodGang name in which the pod is running.
	PodGangNameFileName = "podgangname"
	// PodNamespaceFileName is the name of the file that contains the namespace in which the pod is running.
	PodNamespaceFileName = "namespace"
	// VolumeMountPathPodInfo contains the file path at which the downward API volume is mounted.
	VolumeMountPathPodInfo = "/var/grove/pod-info"
)
