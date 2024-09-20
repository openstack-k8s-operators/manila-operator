package manila

import (
	"fmt"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManilaJob func
func Job(
	instance *manilav1.Manila,
	labels map[string]string,
	annotations map[string]string,
	jobName string,
	jobCommand string,
) *batchv1.Job {
	var config0644AccessMode int32 = 0644
	// Unlike the individual manila services, DbSyncJob or a Job executing a
	// manila-manage command doesn't need a secret that contains all of the
	// config snippets required by every service, The two snippet files that it
	// does need (DefaultsConfigFileName and CustomConfigFileName) can be
	// extracted from the top-level manila config-data secret.
	manilaJobVolume := []corev1.Volume{
		{
			Name: "job-config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  instance.Name + "-config-data",
					Items: []corev1.KeyToPath{
						{
							Key:  DefaultsConfigFileName,
							Path: DefaultsConfigFileName,
						},
						{
							Key:  CustomConfigFileName,
							Path: CustomConfigFileName,
						},
					},
				},
			},
		},
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &config0644AccessMode,
					SecretName:  instance.Name + "-config-data",
				},
			},
		},
	}

	manilaJobMounts := []corev1.VolumeMount{
		{
			Name:      "job-config-data",
			MountPath: "/etc/manila/manila.conf.d",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/var/lib/kolla/config_files/config.json",
			SubPath:   "db-sync-config.json",
			ReadOnly:  true,
		},
	}

	args := []string{"-c", jobCommand}

	// add CA cert if defined
	if instance.Spec.ManilaAPI.TLS.CaBundleSecretName != "" {
		manilaJobVolume = append(manilaJobVolume, instance.Spec.ManilaAPI.TLS.CreateVolume())
		manilaJobMounts = append(manilaJobMounts, instance.Spec.ManilaAPI.TLS.CreateVolumeMounts(nil)...)
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["KOLLA_BOOTSTRAP"] = env.SetValue("TRUE")

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			//Name:      instance.Name + "-db-sync",
			Name:      fmt.Sprintf("%s-%s", instance.Name, jobName),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: instance.RbacResourceName(),
					Containers: []corev1.Container{
						{
							Name: fmt.Sprintf("%s-%s", instance.Name, jobName),
							Command: []string{
								"/bin/bash",
							},
							Args:            args,
							Image:           instance.Spec.ManilaAPI.ContainerImage,
							SecurityContext: manilaDefaultSecurityContext(),
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:    manilaJobMounts,
						},
					},
					Volumes: manilaJobVolume,
				},
			},
		},
	}

	return job
}
