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
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"

	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManilaAPITemplate defines the input parameter for the ManilaAPI service
type ManilaAPITemplate struct {
	ManilaAPITemplateCore `json:",inline"`

	// +kubebuilder:validation:Required
	// ContainerImage - Manila API Container Image URL
	ContainerImage string `json:"containerImage"`
}

// ManilaAPITemplateCore -
type ManilaAPITemplateCore struct {

	// Common input parameters collected in the ManilaServiceTemplate
	ManilaServiceTemplate `json:",inline"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=0
	// Replicas - Manila API Replicas
	Replicas *int32 `json:"replicas"`

	// +kubebuilder:validation:Optional
	// Override, provides the ability to override the generated manifest of several child resources.
	Override APIOverrideSpec `json:"override,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// TLS - Parameters related to the TLS
	TLS tls.API `json:"tls,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Auth - Parameters related to authentication
	Auth AuthSpec `json:"auth,omitempty"`
}

// APIOverrideSpec to override the generated manifest of several child resources.
type APIOverrideSpec struct {
	// Override configuration for the Service created to serve traffic to the cluster.
	// The key must be the endpoint type (public, internal)
	Service map[service.Endpoint]service.RoutedOverrideSpec `json:"service,omitempty"`
}

// AuthSpec defines authentication parameters
type AuthSpec struct {
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// ApplicationCredentialSecret - Secret containing Application Credential ID and Secret
	ApplicationCredentialSecret string `json:"applicationCredentialSecret,omitempty"`
}

// ManilaAPISpec defines the desired state of ManilaAPI
type ManilaAPISpec struct {

	// Common input parameters for all Manila services
	ManilaTemplate `json:",inline"`

	// Input parameters for the ManilaAPI service
	ManilaAPITemplate `json:",inline"`

	// +kubebuilder:validation:Optional
	// DatabaseHostname - Manila Database Hostname
	DatabaseHostname string `json:"databaseHostname,omitempty"`

	// Secret containing RabbitMq transport URL
	TransportURLSecret string `json:"transportURLSecret,omitempty"`

	// Secret containing Notification transport URL
	NotificationsURLSecret string `json:"notificationsURLSecret,omitempty"`

	// +kubebuilder:validation:Optional
	// ExtraMounts containing conf files and credentials
	ExtraMounts []ManilaExtraVolMounts `json:"extraMounts,omitempty"`

	// +kubebuilder:validation:Required
	// ServiceAccount - service account name used internally to provide the default SA name
	ServiceAccount string `json:"serviceAccount"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=memcached
	// Memcached instance name.
	MemcachedInstance *string `json:"memcachedInstance"`
}

// ManilaAPIStatus defines the observed state of ManilaAPI
type ManilaAPIStatus struct {
	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// ReadyCount of Manila API instances
	ReadyCount int32 `json:"readyCount,omitempty"`

	// NetworkAttachments status of the deployment pods
	NetworkAttachments map[string][]string `json:"networkAttachments,omitempty"`

	// ObservedGeneration - the most recent generation observed for this
	// service. If the observed generation is less than the spec generation,
	// then the controller has not processed the latest changes injected by
	// the opentack-operator in the top-level CR (e.g. the ContainerImage)
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastAppliedTopology - the last applied Topology
	LastAppliedTopology *topologyv1.TopoRef `json:"lastAppliedTopology,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="NetworkAttachments",type="string",JSONPath=".status.networkAttachments",description="NetworkAttachments"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

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
	return instance.Status.ReadyCount == *instance.Spec.Replicas
}

// GetSpecTopologyRef - Returns the LastAppliedTopology Set in the Status
func (instance *ManilaAPI) GetSpecTopologyRef() *topologyv1.TopoRef {
	return instance.Spec.TopologyRef
}

// GetLastAppliedTopology - Returns the LastAppliedTopology Set in the Status
func (instance *ManilaAPI) GetLastAppliedTopology() *topologyv1.TopoRef {
	return instance.Status.LastAppliedTopology
}

// SetLastAppliedTopology - Sets the LastAppliedTopology value in the Status
func (instance *ManilaAPI) SetLastAppliedTopology(topologyRef *topologyv1.TopoRef) {
	instance.Status.LastAppliedTopology = topologyRef
}
