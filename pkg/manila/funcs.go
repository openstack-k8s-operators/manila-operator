package manila

import (
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetOwningManilaName - Given a ManilaAPI, ManilaScheduler, ManilaBackup or ManilaVolume
// object, returning the parent Manila object that created it (if any)
func GetOwningManilaName(instance client.Object) string {
	for _, ownerRef := range instance.GetOwnerReferences() {
		if ownerRef.Kind == "Manila" {
			return ownerRef.Name
		}
	}
	return ""
}

// GetManilaSecurityContext - Returns the right set of SecurityContext that
// does not violate the k8s requirements
func GetManilaSecurityContext() *corev1.SecurityContext {
	falseVal := false
	trueVal := true
	runAsUser := int64(42429)
	runAsGroup := int64(42429)
	return &corev1.SecurityContext{
		RunAsUser:                &runAsUser,
		RunAsGroup:               &runAsGroup,
		RunAsNonRoot:             &trueVal,
		AllowPrivilegeEscalation: &falseVal,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// GetPodAffinity - Returns a corev1.Affinity reference for the specified component.
func GetPodAffinity(componentName string) *corev1.Affinity {
	// If possible two pods of the same component (e.g manila-share) should not
	// run on the same worker node. If this is not possible they get still
	// created on the same worker node.
	return affinity.DistributePods(
		common.ComponentSelector,
		[]string{
			componentName,
		},
		corev1.LabelHostname,
	)
}
