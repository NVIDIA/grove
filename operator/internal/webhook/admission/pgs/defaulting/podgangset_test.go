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

package defaulting

import (
	"testing"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestDefaultPodGangSet(t *testing.T) {
	want := v1alpha1.PodGangSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "PGS1",
			Namespace: "default",
		},
		Spec: v1alpha1.PodGangSetSpec{
			Template: v1alpha1.PodGangTemplateSpec{
				Cliques: []*v1alpha1.PodCliqueTemplateSpec{{
					Name: "test",
					Spec: v1alpha1.PodCliqueSpec{
						Replicas: 1,
						PodSpec: corev1.PodSpec{
							RestartPolicy:                 corev1.RestartPolicyAlways,
							TerminationGracePeriodSeconds: ptr.To[int64](30),
						},
						ScaleConfig: &v1alpha1.AutoScalingConfig{
							MinReplicas: ptr.To[int32](1),
							MaxReplicas: 3,
						},
					},
				}},
				StartupType:         ptr.To(v1alpha1.CliqueStartupTypeInOrder),
				NetworkPackStrategy: ptr.To(v1alpha1.BestEffort),
				ServiceSpec: &v1alpha1.ServiceSpec{
					PublishNotReadyAddresses: true,
				},
			},
			UpdateStrategy: &v1alpha1.GangUpdateStrategy{
				RollingUpdateConfig: &v1alpha1.RollingUpdateConfiguration{
					MaxSurge:       ptr.To(intstr.FromInt32(1)),
					MaxUnavailable: ptr.To(intstr.FromInt32(1)),
				},
			},
		},
	}
	input := v1alpha1.PodGangSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "PGS1",
		},
		Spec: v1alpha1.PodGangSetSpec{
			Template: v1alpha1.PodGangTemplateSpec{
				Cliques: []*v1alpha1.PodCliqueTemplateSpec{{
					Name: "test",
					Spec: v1alpha1.PodCliqueSpec{
						ScaleConfig: &v1alpha1.AutoScalingConfig{
							MaxReplicas: 3,
						},
					},
				}},
				ServiceSpec: &v1alpha1.ServiceSpec{
					PublishNotReadyAddresses: ptr.To(true),
				},
			},
		},
	}
	defaultPodGangSet(&input)
	assert.Equal(t, want, input)
}
