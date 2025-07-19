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
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// ShareNameLabel - Label used to identify the manila-share name / backend
	ShareNameLabel = "share-name"
)

// ManilaShareTemplate defines the input parameter for the ManilaShare service
type ManilaShareTemplate struct {

	// Common input parameters collected in the ManilaServiceTemplate
	ManilaShareTemplateCore `json:",inline"`

	// +kubebuilder:validation:Required
	// ContainerImage - Manila API Container Image URL
	ContainerImage string `json:"containerImage"`
}

// ManilaShareTemplateCore -
type ManilaShareTemplateCore struct {

	// Common input parameters collected in the ManilaServiceTemplate
	ManilaServiceTemplate `json:",inline"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=0
	// Replicas - Manila API Replicas
	Replicas *int32 `json:"replicas"`
}

// ManilaShareSpec defines the desired state of ManilaShare
type ManilaShareSpec struct {

	// Common input parameters for all Manila services
	ManilaTemplate `json:",inline"`

	// Input parameters for the ManilaScheduler service
	ManilaShareTemplate `json:",inline"`

	// +kubebuilder:validation:Optional
	// DatabaseHostname - manila Database Hostname
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

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// TLS - Parameters related to the TLS
	TLS tls.Ca `json:"tls,omitempty"`
}

// ManilaShareStatus defines the observed state of ManilaShare
type ManilaShareStatus struct {
	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// ReadyCount of ManilaShare instances
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

// ManilaShare is the Schema for the manilashares API
type ManilaShare struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManilaShareSpec   `json:"spec,omitempty"`
	Status ManilaShareStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManilaShareList contains a list of ManilaShare
type ManilaShareList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManilaShare `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManilaShare{}, &ManilaShareList{})
}

// IsReady - returns true if service is ready to serve requests
func (instance ManilaShare) IsReady() bool {
	return instance.Status.ReadyCount == *instance.Spec.Replicas
}

// ShareName - returns the name used to identify the ManilaShare
func (instance ManilaShare) ShareName() string {
	// The information is stored as a label
	return instance.Labels[ShareNameLabel]
}

// GetSpecTopologyRef - Returns the LastAppliedTopology Set in the Status
func (instance *ManilaShare) GetSpecTopologyRef() *topologyv1.TopoRef {
	return instance.Spec.TopologyRef
}

// GetLastAppliedTopology - Returns the LastAppliedTopology Set in the Status
func (instance *ManilaShare) GetLastAppliedTopology() *topologyv1.TopoRef {
	return instance.Status.LastAppliedTopology
}

// SetLastAppliedTopology - Sets the LastAppliedTopology value in the Status
func (instance *ManilaShare) SetLastAppliedTopology(topologyRef *topologyv1.TopoRef) {
	instance.Status.LastAppliedTopology = topologyRef
}
