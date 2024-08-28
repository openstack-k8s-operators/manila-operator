/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package manilascheduler

import (
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	manila "github.com/openstack-k8s-operators/manila-operator/pkg/manila"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// ServiceCommand -
	ServiceCommand = "/usr/local/bin/kolla_start"
)

// StatefulSet func
func StatefulSet(
	instance *manilav1.ManilaScheduler,
	configHash string,
	labels map[string]string,
	annotations map[string]string,
) *appsv1.StatefulSet {
	manilaUser := manila.ManilaUserID
	manilaGroup := manila.ManilaGroupID

	livenessProbe := &corev1.Probe{
		// TODO might need tuning
		TimeoutSeconds:      20,
		PeriodSeconds:       20,
		InitialDelaySeconds: 10,
	}

	startupProbe := &corev1.Probe{
		TimeoutSeconds:      10,
		FailureThreshold:    12,
		PeriodSeconds:       10,
		InitialDelaySeconds: 10,
	}

	var probeCommand []string
	livenessProbe.HTTPGet = &corev1.HTTPGetAction{
		Port: intstr.FromInt(8080),
	}
	startupProbe.HTTPGet = livenessProbe.HTTPGet
	probeCommand = []string{
		"/usr/local/bin/container-scripts/healthcheck.py",
		"scheduler",
		"/etc/manila/manila.conf.d",
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

	volumes := GetVolumes(
		manila.GetOwningManilaName(instance),
		instance.Name,
		instance.Spec.ExtraMounts)

	volumeMounts := GetVolumeMounts(instance.Spec.ExtraMounts)

	// Add the CA bundle
	if instance.Spec.TLS.CaBundleSecretName != "" {
		volumes = append(volumes, instance.Spec.TLS.CreateVolume())
		volumeMounts = append(volumeMounts, instance.Spec.TLS.CreateVolumeMounts(nil)...)
	}
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels:      labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: instance.Spec.ServiceAccount,
					Containers: []corev1.Container{
						{
							Name: ComponentName,
							Command: []string{
								"/usr/bin/dumb-init",
							},
							Args: []string{
								"--single-child",
								"--",
								"/bin/bash",
								"-c",
								string(ServiceCommand),
							},
							Image: instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &manilaUser,
							},
							Env:           env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:  volumeMounts,
							Resources:     instance.Spec.Resources,
							LivenessProbe: livenessProbe,
							StartupProbe:  startupProbe,
						},
						{
							Name:    "probe",
							Command: probeCommand,
							Image:   instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  &manilaUser,
								RunAsGroup: &manilaGroup,
							},
							VolumeMounts: volumeMounts,
						},
					},
					Affinity:     manila.GetPodAffinity(ComponentName),
					NodeSelector: instance.Spec.NodeSelector,
					Volumes:      volumes,
				},
			},
		},
	}

	return statefulset
}
