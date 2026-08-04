package manilashare

import (
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/internal/manila"
	corev1 "k8s.io/api/core/v1"
)

// configMode is the default file permission for secret-backed config volumes.
var configMode int32 = 0440

// GetVolumes -
func GetVolumes(
	parentName string,
	name string,
	extraVol []manilav1.ManilaExtraVolMounts,
	propagationInstanceName string,
) []corev1.Volume {
	shareVolumes := []corev1.Volume{
		{
			Name: "config-data-custom",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &configMode,
					SecretName:  name + "-config-data",
				},
			},
		},
	}

	// Set the propagation levels for ManilaShare, including the backend name
	propagation := append(manila.ManilaSharePropagation, storage.PropagationType(propagationInstanceName))
	return append(manila.GetVolumes(parentName, extraVol, propagation), shareVolumes...)
}

// GetVolumeMounts - Manila Share VolumeMounts
func GetVolumeMounts(
	extraVol []manilav1.ManilaExtraVolMounts,
	propagationInstanceName string,
) []corev1.VolumeMount {
	shareVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "config-data-custom",
			MountPath: "/etc/manila/manila.conf.d",
			ReadOnly:  true,
		},
	}

	// Set the propagation levels for ManilaShare, including the backend name
	propagation := append(manila.ManilaSharePropagation, storage.PropagationType(propagationInstanceName))
	return append(manila.GetVolumeMounts(extraVol, propagation), shareVolumeMounts...)
}
