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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ManilaSchedulerTemplate defines the input parameter for the ManilaScheduler service
type ManilaSchedulerTemplate struct {

	// Common input parameters collected in the ManilaServiceTemplate
	ManilaServiceTemplate `json:",inline"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=0
	// Replicas - Manila API Replicas
	Replicas *int32 `json:"replicas"`
}

// ManilaSchedulerSpec defines the desired state of ManilaScheduler
type ManilaSchedulerSpec struct {

	// Common input parameters for all Manila services
	ManilaTemplate `json:",inline"`

	// Input parameters for the ManilaScheduler service
	ManilaSchedulerTemplate `json:",inline"`

	// +kubebuilder:validation:Optional
	// DatabaseHostname - manila Database Hostname
	DatabaseHostname string `json:"databaseHostname,omitempty"`

	// Secret containing RabbitMq transport URL
	TransportURLSecret string `json:"transportURLSecret,omitempty"`

	// +kubebuilder:validation:Optional
	// ExtraMounts containing conf files and credentials
	ExtraMounts []ManilaExtraVolMounts `json:"extraMounts,omitempty"`

	// +kubebuilder:validation:Required
	// ServiceAccount - service account name used internally to provide the default SA name
	ServiceAccount string `json:"serviceAccount"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// TLS - Parameters related to the TLS
	TLS tls.Ca `json:"tls,omitempty"`
}

// ManilaSchedulerStatus defines the observed state of ManilaScheduler
type ManilaSchedulerStatus struct {
	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// ReadyCount of Manila Scheduler instances
	ReadyCount int32 `json:"readyCount,omitempty"`

	// NetworkAttachments status of the deployment pods
	NetworkAttachments map[string][]string `json:"networkAttachments,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="NetworkAttachments",type="string",JSONPath=".status.networkAttachments",description="NetworkAttachments"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// ManilaScheduler is the Schema for the manilaschedulers API
type ManilaScheduler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManilaSchedulerSpec   `json:"spec,omitempty"`
	Status ManilaSchedulerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManilaSchedulerList contains a list of ManilaScheduler
type ManilaSchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManilaScheduler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManilaScheduler{}, &ManilaSchedulerList{})
}

// IsReady - returns true if ManilaScheduler is reconciled successfully
func (instance ManilaScheduler) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}
