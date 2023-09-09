package manilaapi

import (
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
	corev1 "k8s.io/api/core/v1"
)

// GetVolumes -
func GetVolumes(parentName string, name string, extraVol []manilav1.ManilaExtraVolMounts) []corev1.Volume {
	var config0644AccessMode int32 = 0644

	apiVolumes := []corev1.Volume{
		{
			Name: "config-data-custom",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  name + "-config-data",
				},
			},
		},
	}

	return append(manila.GetVolumes(parentName, extraVol, manila.ManilaAPIPropagation), apiVolumes...)
}

// GetVolumeMounts - ManilaAPI VolumeMounts
func GetVolumeMounts(extraVol []manilav1.ManilaExtraVolMounts) []corev1.VolumeMount {
	apiVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "config-data-custom",
			MountPath: "/etc/manila/manila.conf.d",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/var/lib/kolla/config_files/config.json",
			SubPath:   "manila-api-config.json",
			ReadOnly:  true,
		},
	}

	return append(manila.GetVolumeMounts(extraVol, manila.ManilaAPIPropagation), apiVolumeMounts...)
}
