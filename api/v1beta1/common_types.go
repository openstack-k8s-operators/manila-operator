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
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1 "k8s.io/api/core/v1"
)

const (
	// Container image fall-back defaults

	// ManilaAPIContainerImage is the fall-back container image for ManilaAPI
	ManilaAPIContainerImage = "quay.io/podified-antelope-centos9/openstack-manila-api:current-podified"
	// ManilaSchedulerContainerImage is the fall-back container image for ManilaScheduler
	ManilaSchedulerContainerImage = "quay.io/podified-antelope-centos9/openstack-manila-scheduler:current-podified"
	// ManilaShareContainerImage is the fall-back container image for ManilaShare
	ManilaShareContainerImage = "quay.io/podified-antelope-centos9/openstack-manila-share:current-podified"
)

// ManilaTemplate defines common input parameters used by all Manila services
type ManilaTemplate struct {

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=manila
	// ServiceUser - optional username used for this service to register in manila
	ServiceUser string `json:"serviceUser"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=manila
	// DatabaseUser - optional username used for manila DB, defaults to manila
	// TODO: -> implement needs work in mariadb-operator, right now only manila
	DatabaseUser string `json:"databaseUser,omitempty"`

	// +kubebuilder:validation:Optional
	// Secret containing OpenStack password information for ManilaDatabasePassword, AdminPassword
	Secret string `json:"secret,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={database: ManilaDatabasePassword, service: ManilaPassword}
	// PasswordSelectors - Selectors to identify the DB and ServiceUser password from the Secret
	PasswordSelectors PasswordSelector `json:"passwordSelectors,omitempty"`
}

// ManilaServiceTemplate defines the input parameters that can be defined for a given
// Manila service
type ManilaServiceTemplate struct {

	// +kubebuilder:validation:Required
	// ContainerImage - Manila API Container Image URL
	ContainerImage string `json:"containerImage"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service. Setting here overrides
	// any global NodeSelector settings within the Manila CR.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// Debug - enable debug for different deploy stages. If an init container is used, it runs and the
	// actual action pod gets started with sleep infinity
	Debug ManilaServiceDebug `json:"debug,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="# add your customization here"
	// CustomServiceConfig - customize the service config using this parameter to change service defaults,
	// or overwrite rendered information using raw OpenStack config format. The content gets added to
	// to /etc/<service>/<service>.conf.d directory as custom.conf file.
	CustomServiceConfig string `json:"customServiceConfig,omitempty"`

	// +kubebuilder:validation:Optional
	// ConfigOverwrite - interface to overwrite default config files like e.g. policy.json.
	// But can also be used to add additional files. Those get added to the service config dir in /etc/<service> .
	DefaultConfigOverwrite map[string]string `json:"defaultConfigOverwrite,omitempty"`

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
}

// PasswordSelector to identify the DB and AdminUser password from the Secret
type PasswordSelector struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="ManilaDatabasePassword"
	// Database - Selector to get the manila database user password from the Secret
	// TODO: not used, need change in mariadb-operator
	Database string `json:"database,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="ManilaPassword"
	// Service - Selector to get the manila service password from the Secret
	Service string `json:"service,omitempty"`
}

// ManilaDebug indicates whether certain stages of Manila deployment should
// pause in debug mode
type ManilaDebug struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// dbInitContainer enable debug (waits until /tmp/stop-init-container disappears)
	DBInitContainer bool `json:"dbInitContainer,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// dbSync enable debug
	DBSync bool `json:"dbSync,omitempty"`
}

// ManilaServiceDebug indicates whether certain stages of Manila service
// deployment should pause in debug mode
type ManilaServiceDebug struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// initContainer enable debug (waits until /tmp/stop-init-container disappears)
	InitContainer bool `json:"initContainer,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// service enable debug
	Service bool `json:"service,omitempty"`
}

// SetupDefaults - initializes any CRD field defaults based on environment variables (the defaulting mechanism itself is implemented via webhooks)
func SetupDefaults() {
	// Acquire environmental defaults and initialize Manila defaults with them
	manilaDefaults := ManilaDefaults{
		APIContainerImageURL:       util.GetEnvVar("RELATED_IMAGE_MANILA_API_IMAGE_URL_DEFAULT", ManilaAPIContainerImage),
		SchedulerContainerImageURL: util.GetEnvVar("RELATED_IMAGE_MANILA_SCHEDULER_IMAGE_URL_DEFAULT", ManilaSchedulerContainerImage),
		ShareContainerImageURL:     util.GetEnvVar("RELATED_IMAGE_MANILA_SHARE_IMAGE_URL_DEFAULT", ManilaShareContainerImage),
	}

	SetupManilaDefaults(manilaDefaults)
}
