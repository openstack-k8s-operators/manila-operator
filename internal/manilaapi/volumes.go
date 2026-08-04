package manilaapi

import (
	"github.com/openstack-k8s-operators/lib-common/modules/common/volume"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/internal/manila"
	corev1 "k8s.io/api/core/v1"
)

// configMode is the default file permission for secret-backed config volumes.
var configMode int32 = 0440

// GetVolumes -
func GetVolumes(parentName string, name string, extraVol []manilav1.ManilaExtraVolMounts) []corev1.Volume {
	apiVolumes := []corev1.Volume{
		{
			Name: "config-data-custom",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &configMode,
					SecretName:  name + "-config-data",
				},
			},
		},
		volume.WritableDirVolume(volume.RunHttpdVolumeName),
		volume.WritableDirVolume(volume.VarLogHttpdVolumeName),
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
			MountPath: "/etc/httpd/conf/httpd.conf",
			SubPath:   "httpd.conf",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/httpd/conf.d/ssl.conf",
			SubPath:   "ssl.conf",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/httpd/conf.d/10-manila_wsgi.conf",
			SubPath:   "10-manila_wsgi.conf",
			ReadOnly:  true,
		},
		volume.WritableDirVolumeMount(volume.RunHttpdVolumeName, volume.RunHttpdMountPath),
		volume.WritableDirVolumeMount(volume.VarLogHttpdVolumeName, volume.VarLogHttpdMountPath),
	}

	return append(manila.GetVolumeMounts(extraVol, manila.ManilaAPIPropagation), apiVolumeMounts...)
}

// GetLogVolumeMount - Manila API LogVolumeMount
func GetLogVolumeMount() corev1.VolumeMount {
	return volume.WritableDirVolumeMount(logVolume, "/var/log/manila")
}

// GetLogVolume - Manila API LogVolume
func GetLogVolume() corev1.Volume {
	return volume.WritableDirVolume(logVolume)
}
