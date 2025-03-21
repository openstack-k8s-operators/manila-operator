/*
Copyright 2020 Red Hat

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// Container image fall-back defaults

	// ManilaAPIContainerImage is the fall-back container image for ManilaAPI
	ManilaAPIContainerImage = "quay.io/podified-antelope-centos9/openstack-manila-api:current-podified"
	// ManilaSchedulerContainerImage is the fall-back container image for ManilaScheduler
	ManilaSchedulerContainerImage = "quay.io/podified-antelope-centos9/openstack-manila-scheduler:current-podified"
	// ManilaShareContainerImage is the fall-back container image for ManilaShare
	ManilaShareContainerImage = "quay.io/podified-antelope-centos9/openstack-manila-share:current-podified"
	//DBPurgeDefaultAge indicates the number of days of purging DB records
	DBPurgeDefaultAge = 30
	//DBPurgeDefaultSchedule is in crontab format, and the default runs the job once every day
	DBPurgeDefaultSchedule = "1 0 * * *"
	// APIDefaultTimeout indicates the default APITimeout for HAProxy and Apache, defaults to 60 seconds
	APIDefaultTimeout = 60
)

// ManilaTemplate defines common input parameters used by all Manila services
type ManilaTemplate struct {

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=manila
	// ServiceUser - optional username used for this service to register in manila
	ServiceUser string `json:"serviceUser"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=manila
	// DatabaseAccount - optional MariaDBAccount CR name used for manila DB, defaults to manila
	DatabaseAccount string `json:"databaseAccount"`

	// +kubebuilder:validation:Optional
	// Secret containing OpenStack password information for AdminPassword
	Secret string `json:"secret,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={service: ManilaPassword}
	// PasswordSelectors - Selectors to identify the ServiceUser password from the Secret
	PasswordSelectors PasswordSelector `json:"passwordSelectors,omitempty"`
}

// ManilaServiceTemplate defines the input parameters that can be defined for a given
// Manila service
type ManilaServiceTemplate struct {

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service. Setting here overrides
	// any global NodeSelector settings within the Manila CR.
	NodeSelector *map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="# add your customization here"
	// CustomServiceConfig - customize the service config using this parameter to change service defaults,
	// or overwrite rendered information using raw OpenStack config format. The content gets added to
	// to /etc/<service>/<service>.conf.d directory a custom config file.
	CustomServiceConfig string `json:"customServiceConfig,omitempty"`

	// +kubebuilder:validation:Optional
	// CustomServiceConfigSecrets - customize the service config using this parameter to specify Secrets
	// that contain sensitive service config data. The content of each Secret gets added to the
	// /etc/<service>/<service>.conf.d directory as a custom config file.
	CustomServiceConfigSecrets []string `json:"customServiceConfigSecrets,omitempty"`
	// +kubebuilder:validation:Optional
	// Resources - Compute Resources required by this service (Limits/Requests).
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// NetworkAttachments is a list of NetworkAttachment resource names to expose the services to the given network
	NetworkAttachments []string `json:"networkAttachments,omitempty"`

	// +kubebuilder:validation:Optional
	// TopologyRef to apply the Topology defined by the associated CR referenced
	// by name
	TopologyRef *topologyv1.TopoRef `json:"topologyRef,omitempty"`
}

// PasswordSelector to identify the DB and AdminUser password from the Secret
type PasswordSelector struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="ManilaPassword"
	// Service - Selector to get the manila service password from the Secret
	Service string `json:"service,omitempty"`
}

// ValidateTopology -
func (instance *ManilaServiceTemplate) ValidateTopology(
	basePath *field.Path,
	namespace string,
) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = append(allErrs, topologyv1.ValidateTopologyRef(
		instance.TopologyRef,
		*basePath.Child("topologyRef"), namespace)...)
	return allErrs
}
