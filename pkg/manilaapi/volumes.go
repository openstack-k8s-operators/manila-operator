package manilaapi

import (
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
	corev1 "k8s.io/api/core/v1"
)

// GetVolumes -
func GetVolumes(parentName string, name string) []corev1.Volume {
	var config0640AccessMode int32 = 0640

	backupVolumes := []corev1.Volume{
		{
			Name: "config-data-custom",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0640AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name + "-config-data",
					},
				},
			},
		},
	}

	return append(manila.GetVolumes(parentName), backupVolumes...)
}

// GetInitVolumeMounts - ManilaAPI init task VolumeMounts
func GetInitVolumeMounts() []corev1.VolumeMount {

	customConfVolumeMount := corev1.VolumeMount{
		Name:      "config-data-custom",
		MountPath: "/var/lib/config-data/custom",
		ReadOnly:  true,
	}

	return append(manila.GetInitVolumeMounts(), customConfVolumeMount)
}

// GetVolumeMounts - ManilaAPI VolumeMounts
func GetVolumeMounts() []corev1.VolumeMount {
	return manila.GetVolumeMounts()
}
