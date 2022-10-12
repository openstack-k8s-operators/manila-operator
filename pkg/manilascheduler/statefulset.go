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
	common "github.com/***REMOVED***-k8s-operators/lib-common/modules/common"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/affinity"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/env"
	manilav1 "github.com/***REMOVED***-k8s-operators/manila-operator/api/v1beta1"
	manila "github.com/***REMOVED***-k8s-operators/manila-operator/pkg/manila"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// ServiceCommand -
	ServiceCommand = "/usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start"
)

// StatefulSet func
func StatefulSet(
	instance *manilav1.ManilaScheduler,
	configHash string,
	labels map[string]string,
) *appsv1.StatefulSet {
	rootUser := int64(0)
	// manila's uid and gid magic numbers come from the 'manila-user' in
	// https://github.com/***REMOVED***/kolla/blob/master/kolla/common/users.py
	manilaUser := int64(42429)
	manilaGroup := int64(42429)

	// TODO until we determine how to properly query for these
	livenessProbe := &corev1.Probe{
		// TODO might need tuning
		TimeoutSeconds:      5,
		PeriodSeconds:       3,
		InitialDelaySeconds: 3,
	}

	startupProbe := &corev1.Probe{
		TimeoutSeconds:      5,
		FailureThreshold:    12,
		PeriodSeconds:       5,
		InitialDelaySeconds: 5,
	}

	args := []string{"-c"}
	var probeCommand []string
	// When debugging the service container will run kolla_set_configs and
	// sleep forever and the probe container will just sleep forever.
	if instance.Spec.Debug.Service {
		args = append(args, common.DebugCommand)
		livenessProbe.Exec = &corev1.ExecAction{
			Command: []string{
				"/bin/true",
			},
		}
		startupProbe.Exec = livenessProbe.Exec
		probeCommand = []string{
			"/bin/sleep", "infinity",
		}
	} else {
		args = append(args, ServiceCommand)
		// TODO: Config livenessProbe
		/*
			livenessProbe.HTTPGet = &corev1.HTTPGetAction{
				Port: intstr.FromInt(8080),
			}
			startupProbe.HTTPGet = livenessProbe.HTTPGet
			// Probe doesn't run kolla_set_configs because it uses the 'manila' uid
			// and gid and doesn't have permissions to make files be owned by root,
			// so manila.conf is in its original location
			probeCommand = []string{
				"/usr/local/bin/container-scripts/healthcheck.py",
				"scheduler",
				"/var/lib/config-data/merged/manila.conf",
			}
		*/
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_FILE"] = env.SetValue(KollaConfig)
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

	volumeMounts := GetVolumeMounts()

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: &instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: manila.ServiceAccount,
					Containers: []corev1.Container{
						{
							Name: manila.ServiceName + "-scheduler",
							Command: []string{
								"/bin/bash",
							},
							Args:  args,
							Image: instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &rootUser,
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
					NodeSelector: instance.Spec.NodeSelector,
				},
			},
		},
	}
	statefulset.Spec.Template.Spec.Volumes = GetVolumes(manila.GetOwningManilaName(instance), instance.Name)
	// If possible two pods of the same service should not
	// run on the same worker node. If this is not possible
	// the get still created on the same worker node.
	statefulset.Spec.Template.Spec.Affinity = affinity.DistributePods(
		common.AppSelector,
		[]string{
			manila.ServiceName,
		},
		corev1.LabelHostname,
	)
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		statefulset.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}

	initContainerDetails := manila.APIDetails{
		ContainerImage:       instance.Spec.ContainerImage,
		DatabaseHost:         instance.Spec.DatabaseHostname,
		DatabaseUser:         instance.Spec.DatabaseUser,
		DatabaseName:         manila.DatabaseName,
		OSPSecret:            instance.Spec.Secret,
		DBPasswordSelector:   instance.Spec.PasswordSelectors.Database,
		UserPasswordSelector: instance.Spec.PasswordSelectors.Service,
		VolumeMounts:         GetInitVolumeMounts(),
		Debug:                instance.Spec.Debug.InitContainer,
	}

	statefulset.Spec.Template.Spec.InitContainers = manila.InitContainer(initContainerDetails)

	// TODO: Clean up this hack
	// Add custom config for the Scheduler Service
	envVars = map[string]env.Setter{}
	envVars["CustomConf"] = env.SetValue(common.CustomServiceConfigFileName)
	statefulset.Spec.Template.Spec.InitContainers[0].Env = env.MergeEnvs(statefulset.Spec.Template.Spec.InitContainers[0].Env, envVars)

	return statefulset
}
