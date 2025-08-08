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

package kubernetes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPodActive(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "active running pod",
			args: args{pod: createTestPod("running-pod")},
			want: true,
		},
		{
			name: "terminating pod",
			args: args{pod: func() *corev1.Pod {
				pod := createTestPod("terminating-pod")
				pod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				return pod
			}(),
			},
			want: false,
		},
		{
			name: "failed pod with no restart",
			args: args{pod: func() *corev1.Pod {
				pod := createTestPod("failed-pod")
				pod.Status.Phase = corev1.PodFailed
				pod.Spec.RestartPolicy = corev1.RestartPolicyNever
				return pod
			}(),
			},
			want: false,
		},
		{
			name: "failed pod that will restart",
			args: args{pod: func() *corev1.Pod {
				pod := createTestPod("failed-restart-pod")
				pod.Status.Phase = corev1.PodFailed
				pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
				return pod
			}(),
			},
			want: true,
		},
		{
			name: "succeeded pod",
			args: args{pod: func() *corev1.Pod {
				pod := createTestPod("succeeded-pod")
				pod.Status.Phase = corev1.PodSucceeded
				return pod
			}(),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsPodActive(tt.args.pod))
		})
	}
}

func createTestPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
