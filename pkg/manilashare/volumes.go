package manilashare

import (
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

// GetVolumes -
func GetVolumes(parentName string, name string, extraVol []manilav1.ManilaExtraVolMounts) []corev1.Volume {
	var config0644AccessMode int32 = 0644
	var dirOrCreate = corev1.HostPathDirectoryOrCreate

	shareVolumes := []corev1.Volume{
		{
			Name: "var-lib-manila",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/lib/manila",
					Type: &dirOrCreate,
				},
			},
		},
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

	// Set the propagation levels for ManilaShare, including the backend name
	propagation := append(manila.ManilaSharePropagation, storage.PropagationType(strings.TrimPrefix(name, "manila-share-")))
	return append(manila.GetVolumes(parentName, extraVol, propagation), shareVolumes...)
}

// GetVolumeMounts - Manila Share VolumeMounts
func GetVolumeMounts(name string, extraVol []manilav1.ManilaExtraVolMounts) []corev1.VolumeMount {
	shareVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "config-data-custom",
			MountPath: "/etc/manila/manila.conf.d",
			ReadOnly:  true,
		}, {
			Name:      "var-lib-manila",
			MountPath: "/var/lib/manila",
		},
		{
			Name:      "config-data",
			MountPath: "/var/lib/kolla/config_files/config.json",
			SubPath:   "manila-share-config.json",
			ReadOnly:  true,
		},
	}

	// Set the propagation levels for ManilaShare, including the backend name
	propagation := append(manila.ManilaSharePropagation, storage.PropagationType(strings.TrimPrefix(name, "manila-share-")))
	return append(manila.GetVolumeMounts(extraVol, propagation), shareVolumeMounts...)
}
