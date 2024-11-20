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

package manilashare

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
	instance *manilav1.ManilaShare,
	configHash string,
	labels map[string]string,
	annotations map[string]string,
) *appsv1.StatefulSet {
	trueVar := true

	manilaUser := manila.ManilaUserID
	manilaGroup := manila.ManilaGroupID

	// TODO until we determine how to properly query for these
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
		"share",
		"/etc/manila/manila.conf.d",
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

	// Tune glibc for reduced memory usage and fragmentation using single malloc arena for all
	// threads and disabling dynamic thresholds to reduce memory usage when using native threads
	// directly or via eventlet.tpool
	// https://www.gnu.org/software/libc/manual/html_node/Memory-Allocation-Tunables.html
	envVars["MALLOC_ARENA_MAX"] = env.SetValue("1")
	envVars["MALLOC_MMAP_THRESHOLD_"] = env.SetValue("131072")
	envVars["MALLOC_TRIM_THRESHOLD_"] = env.SetValue("262144")

	volumes := GetVolumes(
		manila.GetOwningManilaName(instance),
		instance.Name,
		instance.Spec.ExtraMounts,
		instance.ShareName(),
	)

	volumeMounts := GetVolumeMounts(
		instance.Spec.ExtraMounts,
		instance.ShareName(),
	)

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
								RunAsUser:  &manilaUser,
								Privileged: &trueVar,
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
					Affinity: manila.GetPodAffinity(ComponentName),
					Volumes:  volumes,
				},
			},
		},
	}

	if instance.Spec.NodeSelector != nil {
		statefulset.Spec.Template.Spec.NodeSelector = *instance.Spec.NodeSelector
	}

	return statefulset
}
