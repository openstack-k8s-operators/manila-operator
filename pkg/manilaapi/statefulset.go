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

package manilaapi

import (
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	manila "github.com/openstack-k8s-operators/manila-operator/pkg/manila"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// ServiceCommand -
	ServiceCommand = "/usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start"
)

// StatefulSet func
func StatefulSet(
	instance *manilav1.ManilaAPI,
	configHash string,
	labels map[string]string,
	annotations map[string]string,
) (*appsv1.StatefulSet, error) {
	runAsUser := int64(0)

	livenessProbe := &corev1.Probe{
		TimeoutSeconds:      10,
		PeriodSeconds:       10,
		InitialDelaySeconds: 10,
	}
	readinessProbe := &corev1.Probe{
		TimeoutSeconds:      10,
		PeriodSeconds:       10,
		InitialDelaySeconds: 10,
	}

	//
	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
	//

	livenessProbe.HTTPGet = &corev1.HTTPGetAction{
		Path: "/healthcheck",
		Port: intstr.IntOrString{Type: intstr.Int, IntVal: int32(manila.ManilaPublicPort)},
	}
	readinessProbe.HTTPGet = livenessProbe.HTTPGet

	if instance.Spec.TLS.API.Enabled(service.EndpointPublic) {
		livenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
		readinessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	}

	// create Volume and VolumeMounts
	volumes := append(GetVolumes(manila.GetOwningManilaName(instance), instance.Name, instance.Spec.ExtraMounts), GetLogVolume())
	volumeMounts := append(GetVolumeMounts(instance.Spec.ExtraMounts), GetLogVolumeMount())

	// add CA cert if defined
	if instance.Spec.TLS.CaBundleSecretName != "" {
		volumes = append(volumes, instance.Spec.TLS.CreateVolume())
		volumeMounts = append(volumeMounts, instance.Spec.TLS.CreateVolumeMounts(nil)...)
	}

	for _, endpt := range []service.Endpoint{service.EndpointInternal, service.EndpointPublic} {
		if instance.Spec.TLS.API.Enabled(endpt) {
			var tlsEndptCfg tls.GenericService
			switch endpt {
			case service.EndpointPublic:
				tlsEndptCfg = instance.Spec.TLS.API.Public
			case service.EndpointInternal:
				tlsEndptCfg = instance.Spec.TLS.API.Internal
			}

			svc, err := tlsEndptCfg.ToService()
			if err != nil {
				return nil, err
			}
			volumes = append(volumes, svc.CreateVolume(endpt.String()))
			volumeMounts = append(volumeMounts, svc.CreateVolumeMounts(endpt.String())...)
		}
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

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
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Replicas:            instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels:      labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: instance.Spec.ServiceAccount,
					Containers: []corev1.Container{
						// the first container in a pod is the default selected
						// by oc log so define the log stream container first.
						{
							Name: instance.Name + "-log",
							Command: []string{
								"/usr/bin/dumb-init",
							},
							Args: []string{
								"--single-child",
								"--",
								"/bin/sh",
								"-c",
								"/usr/bin/tail -n+1 -F " + LogFile + " 2>/dev/null",
							},
							Image: instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							Env:          env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts: []corev1.VolumeMount{GetLogVolumeMount()},
							Resources:    instance.Spec.Resources,
						},
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
								RunAsUser: &runAsUser,
							},
							Env:            env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:   volumeMounts,
							Resources:      instance.Spec.Resources,
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
						},
					},
					Affinity:     manila.GetPodAffinity(ComponentName),
					NodeSelector: instance.Spec.NodeSelector,
					Volumes:      volumes,
				},
			},
		},
	}

	return statefulset, nil
}
