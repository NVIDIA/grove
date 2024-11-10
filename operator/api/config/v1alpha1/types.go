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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogFormat defines the format of the log.
type LogFormat string

const (
	// LogFormatJSON is the JSON log format.
	LogFormatJSON LogFormat = "json"
	// LogFormatText is the text log format.
	LogFormatText LogFormat = "text"
)

// LogLevel defines the log level.
type LogLevel string

const (
	// DebugLevel is the debug log level, i.e. the most verbose.
	DebugLevel LogLevel = "debug"
	// InfoLevel is the default log level.
	InfoLevel LogLevel = "info"
	// ErrorLevel is a log level where only errors are logged.
	ErrorLevel LogLevel = "error"
)

var (
	// AllLogLevels is a slice of all available log levels.
	AllLogLevels = []LogLevel{DebugLevel, InfoLevel, ErrorLevel}
	// AllLogFormats is a slice of all available log formats.
	AllLogFormats = []LogFormat{LogFormatJSON, LogFormatText}
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperatorConfiguration defines the configuration for the Grove operator.
type OperatorConfiguration struct {
	metav1.TypeMeta  `json:",inline"`
	ClientConnection ClientConnectionConfiguration `json:"runtimeClientConnection"`
	LeaderElection   LeaderElectionConfiguration
	Server           ServerConfiguration     `json:"server"`
	Debugging        *DebuggingConfiguration `json:"debugging,omitempty"`
	Controllers      ControllerConfiguration `json:"controllers"`
	LogLevel         LogLevel                `json:"logLevel"`
	LogFormat        LogFormat               `json:"logFormat"`
}

// LeaderElectionConfiguration defines the configuration for the leader election.
type LeaderElectionConfiguration struct {
	// Enabled specifies whether leader election is enabled. Set this
	// to true when running replicated instances of the operator for high availability.
	Enabled bool `json:"enabled"`
	// LeaseDuration is the duration that non-leader candidates will wait
	// after observing a leadership renewal until attempting to acquire
	// leadership of the occupied but un-renewed leader slot. This is effectively the
	// maximum duration that a leader can be stopped before it is replaced
	// by another candidate. This is only applicable if leader election is
	// enabled.
	LeaseDuration metav1.Duration `json:"leaseDuration"`
	// RenewDeadline is the interval between attempts by the acting leader to
	// renew its leadership before it stops leading. This must be less than or
	// equal to the lease duration.
	// This is only applicable if leader election is enabled.
	RenewDeadline metav1.Duration `json:"renewDeadline"`
	// RetryPeriod is the duration leader elector clients should wait
	// between attempting acquisition and renewal of leadership.
	// This is only applicable if leader election is enabled.
	RetryPeriod metav1.Duration `json:"retryPeriod"`
	// ResourceLock determines which resource lock to use for leader election.
	// This is only applicable if leader election is enabled.
	ResourceLock string `json:"resourceLock"`
	// ResourceName determines the name of the resource that leader election
	// will use for holding the leader lock.
	// This is only applicable if leader election is enabled.
	ResourceName string `json:"resourceName"`
	// ResourceNamespace determines the namespace in which the leader
	// election resource will be created.
	// This is only applicable if leader election is enabled.
	ResourceNamespace string `json:"resourceNamespace"`
}

// ClientConnectionConfiguration defines the configuration for constructing a client.
type ClientConnectionConfiguration struct {
	// QPS controls the number of queries per second allowed for this connection.
	QPS float32 `json:"qps"`
	// Burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int `json:"burst"`
	// ContentType is the content type used when sending data to the server from this client.
	ContentType string `json:"contentType"`
	// AcceptContentTypes defines the Accept header sent by clients when connecting to the server,
	// overriding the default value of 'application/json'. This field will control all connections
	// to the server used by a particular client.
	AcceptContentTypes string `json:"acceptContentTypes"`
}

// DebuggingConfiguration defines the configuration for debugging.
type DebuggingConfiguration struct {
	// EnableProfiling enables profiling via host:port/debug/pprof/ endpoints.
	EnableProfiling *bool `json:"enableProfiling,omitempty"`
}

// ServerConfiguration defines the configuration for the HTTP(S) servers.
type ServerConfiguration struct {
	// Webhooks is the configuration for the HTTP(S) webhook server.
	Webhooks WebhookServer `json:"webhooks"`
	// HealthProbes is the configuration for serving the healthz and readyz endpoints.
	HealthProbes *Server `json:"healthProbes,omitempty"`
	// Metrics is the configuration for serving the metrics endpoint.
	Metrics *Server `json:"metrics,omitempty"`
}

// WebhookServer defines the configuration for the HTTP(S) webhook server.
type WebhookServer struct {
	Server `json:",inline"`
	// ServerCertDir is the directory containing the server certificate and key.
	ServerCertDir string `json:"serverCertDir"`
}

// Server contains information for HTTP(S) server configuration.
type Server struct {
	// BindAddress is the IP address on which to listen for the specified port.
	BindAddress string `json:"bindAddress"`
	// Port is the port on which to serve requests.
	Port int `json:"port"`
}

// ControllerConfiguration defines the configuration for the controllers.
type ControllerConfiguration struct {
	// PodGangSet is the configuration for the PodGangSet controller.
	PodGangSet PodGangSetControllerConfiguration `json:"podGangSet"`
}

// PodGangSetControllerConfiguration defines the configuration for the PodGangSet controller.
type PodGangSetControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to concurrently work on events.
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}
