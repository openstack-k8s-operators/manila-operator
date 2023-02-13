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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/condition"
)


const (
	// DbSyncHash hash
	DbSyncHash = "dbsync"

	// DeploymentHash hash used to detect changes
	DeploymentHash = "deployment"
)

// ManilaSpec defines the desired state of Manila
type ManilaSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=manila
	// ServiceUser - optional username used for this service to register in manila
	ServiceUser string `json:"serviceUser"`

	// +kubebuilder:validation:Required
	// MariaDB instance name
	// Right now required by the maridb-operator to get the credentials from the instance to create the DB
	// Might not be required in future
	DatabaseInstance string `json:"databaseInstance,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=manila
	// DatabaseUser - optional username used for manila DB, defaults to manila
	// TODO: -> implement needs work in mariadb-operator, right now only manila
	DatabaseUser string `json:"databaseUser"`

	// +kubebuilder:validation:Required
	// Secret containing OpenStack password information for ManilaDatabasePassword, AdminPassword
	Secret string `json:"secret,omitempty"`

	// +kubebuilder:validation:Optional
	// PasswordSelectors - Selectors to identify the DB and AdminUser password and TransportURL from the Secret
	PasswordSelectors PasswordSelector `json:"passwordSelectors,omitempty"`

	// +kubebuilder:validation:Optional
	// Debug - enable debug for different deploy stages. If an init container is used, it runs and the
	// actual action pod gets started with sleep infinity
	Debug ManilaDebug `json:"debug,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// PreserveJobs - do not delete jobs after they finished e.g. to check logs
	PreserveJobs bool `json:"preserveJobs,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="# add your customization here"
	// CustomServiceConfig - customize the service config for all Manila services using this parameter to change service defaults,
	// or overwrite rendered information using raw OpenStack config format. The content gets added to
	// to /etc/<service>/<service>.conf.d directory as custom.conf file.
	CustomServiceConfig string `json:"customServiceConfig,omitempty"`

	// +kubebuilder:validation:Optional
	// ConfigOverwrite - interface to overwrite default config files like e.g. policy.json.
	// But can also be used to add additional files. Those get added to the service config dir in /etc/<service> .
	// TODO: -> implement
	DefaultConfigOverwrite map[string]string `json:"defaultConfigOverwrite,omitempty"`

	// +kubebuilder:validation:Required
	// ManilaAPI - Spec definition for the API service of this Manila deployment
	ManilaAPI ManilaAPISpec `json:"manilaAPI"`

	// +kubebuilder:validation:Required
	// ManilaScheduler - Spec definition for the Scheduler service of this Manila deployment
	ManilaScheduler ManilaSchedulerSpec `json:"manilaScheduler"`

	// +kubebuilder:validation:Optional
	// ManilaShares - Map of chosen names to spec definitions for the Share(s) service(s) of this Manila deployment
	ManilaShares map[string]ManilaShareSpec `json:"manilaShares"`
}

// ManilaStatus defines the observed state of Manila
type ManilaStatus struct {
	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// Manila Database Hostname
	DatabaseHostname string `json:"databaseHostname,omitempty"`

	// API endpoints
	APIEndpoints map[string]map[string]string `json:"apiEndpoints,omitempty"`

	// ServiceIDs
	ServiceIDs map[string]string `json:"serviceIDs,omitempty"`

	// ReadyCount of Manila API instance
	ManilaAPIReadyCount int32 `json:"manilaAPIReadyCount,omitempty"`

	// ReadyCount of Manila Scheduler instance
	ManilaSchedulerReadyCount int32 `json:"manilaSchedulerReadyCount,omitempty"`

	// ReadyCounts of Manila Share instances
	ManilaSharesReadyCounts map[string]int32 `json:"manilaSharesReadyCounts,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Manila is the Schema for the manilas API
type Manila struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManilaSpec   `json:"spec,omitempty"`
	Status ManilaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManilaList contains a list of Manila
type ManilaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Manila `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Manila{}, &ManilaList{})
}

// IsReady - returns true if service is ready to serve requests
func (instance Manila) IsReady() bool {
	ready := instance.Status.ManilaAPIReadyCount > 0 &&
		instance.Status.ManilaSchedulerReadyCount > 0

	for name := range instance.Spec.ManilaShares {
		ready = ready && instance.Status.ManilaSharesReadyCounts[name] > 0
	}

	return ready
}
