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

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	defaultLeaderElectionResourceLock = "leases"
	defaultLeaderElectionResourceName = "grove-operator-leader-election"
)

// SetDefaults_ClientConnectionConfiguration sets defaults for the k8s client connection.
func SetDefaults_ClientConnectionConfiguration(obj *ClientConnectionConfiguration) {
	if obj.QPS == 0.0 {
		obj.QPS = 100.0
	}
	if obj.Burst == 0 {
		obj.Burst = 120
	}
}

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the Grove operator.
func SetDefaults_LeaderElectionConfiguration(obj *LeaderElectionConfiguration) {
	zero := metav1.Duration{}
	if obj.LeaseDuration == zero {
		obj.LeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	}
	if obj.RenewDeadline == zero {
		obj.RenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	}
	if obj.RetryPeriod == zero {
		obj.RetryPeriod = metav1.Duration{Duration: 2 * time.Second}
	}
	if obj.ResourceLock == "" {
		obj.ResourceLock = defaultLeaderElectionResourceLock
	}
	if obj.ResourceName == "" {
		obj.ResourceName = defaultLeaderElectionResourceName
	}
}

// SetDefaults_OperatorConfiguration sets defaults for the configuration of the Grove operator.
func SetDefaults_OperatorConfiguration(obj *OperatorConfiguration) {
	if obj.LogLevel == "" {
		obj.LogLevel = "info"
	}
	if obj.LogFormat == "" {
		obj.LogFormat = "json"
	}
}

// SetDefaults_ServerConfiguration sets defaults for the server configuration.
func SetDefaults_ServerConfiguration(obj *ServerConfiguration) {
	if obj.Webhooks.Port == 0 {
		obj.Webhooks.Port = 2750
	}

	if obj.HealthProbes == nil {
		obj.HealthProbes = &Server{}
	}
	if obj.HealthProbes.Port == 0 {
		obj.HealthProbes.Port = 2751
	}

	if obj.Metrics == nil {
		obj.Metrics = &Server{}
	}
	if obj.Metrics.Port == 0 {
		obj.Metrics.Port = 2752
	}
}

// SetDefaults_PodGangSetControllerConfiguration sets defaults for the PodGangSetControllerConfiguration.
func SetDefaults_PodGangSetControllerConfiguration(obj *PodGangSetControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(1)
	}
}
