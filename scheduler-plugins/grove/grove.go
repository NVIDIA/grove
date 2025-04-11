/*
Copyright 2020 The Kubernetes Authors.

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

package grove

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Grove is a plugin that schedules pods in a group.
type Grove struct {
	logger           klog.Logger
	frameworkHandler framework.Handle
}

var _ framework.QueueSortPlugin = &Grove{}
var _ framework.PreFilterPlugin = &Grove{}
var _ framework.PostFilterPlugin = &Grove{}
var _ framework.PermitPlugin = &Grove{}
var _ framework.ReservePlugin = &Grove{}

var _ framework.EnqueueExtensions = &Grove{}

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = "Grove"
)

// New initializes and returns a new Grove plugin.
func New(ctx context.Context, obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	lh := klog.FromContext(ctx).WithValues("plugin", Name)
	lh.V(5).Info("creating new grove plugin")
	plugin := &Grove{
		logger:           lh,
		frameworkHandler: handle,
	}
	return plugin, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (gv *Grove) Name() string {
	return Name
}

// Less is used to sort pods in the scheduling queue.
func (gv *Grove) Less(podInfo1, podInfo2 *framework.QueuedPodInfo) bool {
	return false
}

func (gv *Grove) EventsToRegister(_ context.Context) ([]framework.ClusterEventWithHint, error) {
	return []framework.ClusterEventWithHint{}, nil
}

// PreFilter
func (gv *Grove) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	return nil, framework.NewStatus(framework.Success, "")
}

// PostFilter is used to reject a group of pods if a pod does not pass PreFilter or Filter.
func (gv *Grove) PostFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	filteredNodeStatusMap framework.NodeToStatusMap) (*framework.PostFilterResult, *framework.Status) {
	return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable)
}

// PreFilterExtensions returns a PreFilterExtensions interface if the plugin implements one.
func (gv *Grove) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// Permit is the functions invoked by the framework at "Permit" extension point.
func (gv *Grove) Permit(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	return framework.NewStatus(framework.Success, ""), 0
}

// Reserve is the functions invoked by the framework at "reserve" extension point.
func (gv *Grove) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	return nil
}

// Unreserve rejects all other Pods in the PodGroup when one of the pods in the group times out.
func (gv *Grove) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	return
}
