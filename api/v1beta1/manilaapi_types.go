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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManilaAPITemplate defines the input parameter for the ManilaAPI service
type ManilaAPITemplate struct {

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
}

// APIOverrideSpec to override the generated manifest of several child resources.
type APIOverrideSpec struct {
	// Override configuration for the Service created to serve traffic to the cluster.
	// The key must be the endpoint type (public, internal)
	Service map[string]service.OverrideSpec `json:"service,omitempty"`
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

	// +kubebuilder:validation:Optional
	// ExtraMounts containing conf files and credentials
	ExtraMounts []ManilaExtraVolMounts `json:"extraMounts,omitempty"`

	// +kubebuilder:validation:Required
	// ServiceAccount - service account name used internally to provide the default SA name
	ServiceAccount string `json:"serviceAccount"`
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

// IsReady - returns true if ManilaAPI is reconciled successfully
func (instance ManilaAPI) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}
