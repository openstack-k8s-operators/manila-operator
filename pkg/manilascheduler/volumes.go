package manilascheduler

import (
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
	corev1 "k8s.io/api/core/v1"
)

// GetVolumes -
func GetVolumes(parentName string, name string, secretNames []string, extraVol []manilav1.ManilaExtraVolMounts) []corev1.Volume {
	var config0640AccessMode int32 = 0640

	schedulerVolumes := []corev1.Volume{
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
	schedulerVolumes = append(schedulerVolumes, secretConfig...)

	return append(manila.GetVolumes(parentName, extraVol, manila.ManilaSchedulerPropagation), schedulerVolumes...)
}

// GetInitVolumeMounts - ManilaScheduler init task VolumeMounts
func GetInitVolumeMounts(secretNames []string, extraVol []manilav1.ManilaExtraVolMounts) []corev1.VolumeMount {

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

	return append(manila.GetInitVolumeMounts(extraVol, manila.ManilaSchedulerPropagation), initVolumeMount...)
}

// GetVolumeMounts - ManilaScheduler VolumeMounts
func GetVolumeMounts(extraVol []manilav1.ManilaExtraVolMounts) []corev1.VolumeMount {
	schedulerVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "config-data",
			MountPath: "/var/lib/kolla/config_files/config.json",
			SubPath:   "manila-scheduler-config.json",
			ReadOnly:  true,
		},
	}

	return append(manila.GetVolumeMounts(extraVol, manila.ManilaSchedulerPropagation), schedulerVolumeMounts...)
}
