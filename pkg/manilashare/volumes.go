package manilashare

import (
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

// GetVolumes -
func GetVolumes(parentName string, name string, secretNames []string, extraVol []manilav1.ManilaExtraVolMounts) []corev1.Volume {
	var config0640AccessMode int32 = 0640
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
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0640AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name + "-config-data",
					},
				},
			},
		},
	}

	// Mount secrets passed using the `customConfigServiceSecret` parameter
	// and they will be rendered as part of the service config
	secretConfig, _ := manila.GetConfigSecretVolumes(secretNames)
	shareVolumes = append(shareVolumes, secretConfig...)

	// Set the propagation levels for ManilaShare, including the backend name
	propagation := append(manila.ManilaSharePropagation, storage.PropagationType(strings.TrimPrefix(name, "manila-share-")))
	return append(manila.GetVolumes(parentName, extraVol, propagation), shareVolumes...)
}

// GetInitVolumeMounts - Manila Share init task
func GetInitVolumeMounts(name string, secretNames []string, extraVol []manilav1.ManilaExtraVolMounts) []corev1.VolumeMount {

	initVolumeMount := []corev1.VolumeMount{
		{
			Name:      "config-data-custom",
			MountPath: "/var/lib/config-data/custom",
			ReadOnly:  true,
		},
	}

	// Mount secrets passed using the `customConfigServiceSecret` parameter
	// and they will be rendered as part of the service config
	_, secretConfig := manila.GetConfigSecretVolumes(secretNames)
	initVolumeMount = append(initVolumeMount, secretConfig...)

	// Set the propagation levels for ManilaShare, including the backend name
	propagation := append(manila.ManilaSharePropagation, storage.PropagationType(strings.TrimPrefix(name, "manila-share-")))
	return append(manila.GetInitVolumeMounts(extraVol, propagation), initVolumeMount...)
}

// GetVolumeMounts - Manila Share VolumeMounts
func GetVolumeMounts(name string, secretNames []string, extraVol []manilav1.ManilaExtraVolMounts) []corev1.VolumeMount {
	shareVolumeMounts := []corev1.VolumeMount{
		{
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
