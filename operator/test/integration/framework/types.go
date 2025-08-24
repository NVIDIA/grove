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

// ControllerType represents the type of controllers to be registered
type ControllerType string

const (
	// ControllerPodGangSet represents the PodGangSet controllers
	ControllerPodGangSet ControllerType = "podgangset"
	// ControllerPodClique represents the PodClique controllers
	ControllerPodClique ControllerType = "podclique"
	// ControllerScalingGroup represents the PodCliqueScalingGroup controllers
	ControllerScalingGroup ControllerType = "scalinggroup"
)

// String returns the string representation of the ControllerType
func (c ControllerType) String() string {
	return string(c)
}

// WebhookType represents the type of webhook to be registered
type WebhookType string

const (
	// WebhookValidation represents validation webhooks
	WebhookValidation WebhookType = "validation"
	// WebhookMutation represents mutation webhooks
	WebhookMutation WebhookType = "mutation"
)

// String returns the string representation of the WebhookType
func (w WebhookType) String() string {
	return string(w)
}
