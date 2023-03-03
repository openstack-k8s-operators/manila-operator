/*
Copyright 2022.

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
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManilaAPISpec defines the desired state of ManilaAPI
type ManilaAPISpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=manila
	// ServiceUser - optional username used for this service to register in manila
	ServiceUser string `json:"serviceUser"`

	// +kubebuilder:validation:Optional
	// ContainerImage - Manila API Container Image URL
	ContainerImage string `json:"containerImage,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// Replicas - Manila API Replicas
	Replicas int32 `json:"replicas"`

	// +kubebuilder:validation:Optional
	// DatabaseHostname - Manila Database Hostname
	DatabaseHostname string `json:"databaseHostname,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=manila
	// DatabaseUser - optional username used for manila DB, defaults to manila
	// TODO: -> implement needs work in mariadb-operator, right now only manila
	DatabaseUser string `json:"databaseUser,omitempty"`

	// +kubebuilder:validation:Optional
	// Secret containing OpenStack password information for ManilaDatabasePassword, AdminPassword
	Secret string `json:"secret,omitempty"`

	// Secret containing RabbitMq transport URL
	TransportURLSecret string `json:"transportURLSecret,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={database: ManilaDatabasePassword, service: ManilaPassword}
	// PasswordSelectors - Selectors to identify the DB and ServiceUser password from the Secret
	PasswordSelectors PasswordSelector `json:"passwordSelectors,omitempty"`

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
	// Resources - Compute Resources required by this service (Limits/Requests).
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// ExtraMounts containing conf files and credentials
	ExtraMounts []ManilaExtraVolMounts `json:"extraMounts"`
}

// ManilaAPIStatus defines the observed state of ManilaAPI
type ManilaAPIStatus struct {
	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// API endpoints
	APIEndpoints map[string]map[string]string `json:"apiEndpoints,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// ReadyCount of Manila API instances
	ReadyCount int32 `json:"readyCount,omitempty"`

	// ServiceIDs
	ServiceIDs map[string]string `json:"serviceIDs,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ManilaAPI is the Schema for the manilaapis API
type ManilaAPI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManilaAPISpec   `json:"spec,omitempty"`
	Status ManilaAPIStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManilaAPIList contains a list of ManilaAPI
type ManilaAPIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManilaAPI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManilaAPI{}, &ManilaAPIList{})
}

// IsReady - returns true if service is ready to serve requests
func (instance ManilaAPI) IsReady() bool {
	return instance.Status.ReadyCount >= 1
}
