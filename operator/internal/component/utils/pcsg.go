package utils

import (
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/samber/lo"
	"strings"
)

// FindMatchingPCSGConfig finds matching PodCliqueScalingGroupConfig defined in PodGangSet which matches PodCliqueScalingGroup fully qualified name.
func FindMatchingPCSGConfig(pgs *grovecorev1alpha1.PodGangSet, pcsgFQN string) grovecorev1alpha1.PodCliqueScalingGroupConfig {
	matchingPCSG, _ := lo.Find(pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs, func(pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig) bool {
		return strings.HasSuffix(pcsgFQN, pcsgConfig.Name)
	})
	return matchingPCSG
}
