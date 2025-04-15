/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package grovescheduling

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	klog "k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// GroveScheduling is a plugin that schedules pods in a group.
type GroveScheduling struct {
	logger           klog.Logger
	frameworkHandler framework.Handle
}

var _ framework.QueueSortPlugin = &GroveScheduling{}

var _ framework.PreFilterPlugin = &GroveScheduling{}

var _ framework.PostFilterPlugin = &GroveScheduling{}

var _ framework.PermitPlugin = &GroveScheduling{}

var _ framework.ReservePlugin = &GroveScheduling{}

var _ framework.EnqueueExtensions = &GroveScheduling{}

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = "GroveScheduling"
)

// New initializes and returns a new GroveScheduling plugin.
func New(ctx context.Context, _ runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	lh := klog.FromContext(ctx).WithValues("plugin", Name)
	lh.V(5).Info("creating new grovescheduling plugin")
	plugin := &GroveScheduling{
		logger:           lh,
		frameworkHandler: handle,
	}
	return plugin, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (gs *GroveScheduling) Name() string {
	return Name
}

// Less is used to sort pods in the scheduling queue.
func (gs *GroveScheduling) Less(_, _ *framework.QueuedPodInfo) bool {
	return false
}

// EventsToRegister establishes the events to be registered. TODO: @renormalize expand on this.
func (gs *GroveScheduling) EventsToRegister(_ context.Context) ([]framework.ClusterEventWithHint, error) {
	return []framework.ClusterEventWithHint{}, nil
}

// PreFilter acts before the filer. TODO: @renormalize expand on this.
func (gs *GroveScheduling) PreFilter(_ context.Context, _ *framework.CycleState, _ *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	return nil, framework.NewStatus(framework.Success, "")
}

// PostFilter is used to reject a group of pods if a pod does not pass PreFilter or Filter.
// Use `filteredNodeStatusMap` as framework.NodeToStatusMap variable name.
func (gs *GroveScheduling) PostFilter(_ context.Context, _ *framework.CycleState, _ *v1.Pod,
	_ framework.NodeToStatusMap) (*framework.PostFilterResult, *framework.Status) {
	return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable)
}

// PreFilterExtensions returns a PreFilterExtensions interface if the plugin implements one.
func (gs *GroveScheduling) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// Permit is the functions invoked by the framework at "Permit" extension point.
// Last field's name is `nodeName`.
func (gs *GroveScheduling) Permit(_ context.Context, _ *framework.CycleState, _ *v1.Pod, _ string) (*framework.Status, time.Duration) {
	return framework.NewStatus(framework.Success, ""), 0
}

// Reserve is the functions invoked by the framework at "reserve" extension point.
// Last field's name is `nodeName`.
func (gs *GroveScheduling) Reserve(_ context.Context, _ *framework.CycleState, _ *v1.Pod, _ string) *framework.Status {
	return nil
}

// Unreserve rejects all other Pods in the PodGroup when one of the pods in the group times out.
// Last field's name is `nodeName`.
func (gs *GroveScheduling) Unreserve(_ context.Context, _ *framework.CycleState, _ *v1.Pod, _ string) {
}
