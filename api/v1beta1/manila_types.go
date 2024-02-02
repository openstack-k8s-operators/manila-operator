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
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DbSyncHash hash
	DbSyncHash = "dbsync"

	// DeploymentHash hash used to detect changes
	DeploymentHash = "deployment"
)

// ManilaSpec defines the desired state of Manila
type ManilaSpec struct {
	ManilaTemplate `json:",inline"`

	// +kubebuilder:validation:Required
	// MariaDB instance name
	// Right now required by the maridb-operator to get the credentials from the instance to create the DB
	// Might not be required in future
	DatabaseInstance string `json:"databaseInstance,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=rabbitmq
	// RabbitMQ instance name
	// Needed to request a transportURL that is created and used in Manila
	RabbitMqClusterName string `json:"rabbitMqClusterName"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=memcached
	// Memcached instance name.
	MemcachedInstance string `json:"memcachedInstance"`

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
	// to /etc/<service>/<service>.conf.d directory a custom config file.
	CustomServiceConfig string `json:"customServiceConfig,omitempty"`

	// +kubebuilder:validation:Required
	// ManilaAPI - Spec definition for the API service of this Manila deployment
	ManilaAPI ManilaAPITemplate `json:"manilaAPI"`

	// +kubebuilder:validation:Required
	// ManilaScheduler - Spec definition for the Scheduler service of this Manila deployment
	ManilaScheduler ManilaSchedulerTemplate `json:"manilaScheduler"`

	// +kubebuilder:validation:Optional
	// ManilaShares - Map of chosen names to spec definitions for the Share(s) service(s) of this Manila deployment
	ManilaShares map[string]ManilaShareTemplate `json:"manilaShares,omitempty"`

	// +kubebuilder:validation:Optional
	// ExtraMounts containing conf files and credentials
	ExtraMounts []ManilaExtraVolMounts `json:"extraMounts,omitempty"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service. Setting
	// NodeSelector here acts as a default value and can be overridden by service
	// specific NodeSelector Settings.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// DBPurge parameters -
	DBPurge DBPurge `json:"dbPurge,omitempty"`
}

// ManilaStatus defines the observed state of Manila
type ManilaStatus struct {
	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// Manila Database Hostname
	DatabaseHostname string `json:"databaseHostname,omitempty"`

	// TransportURLSecret - Secret containing RabbitMQ transportURL
	TransportURLSecret string `json:"transportURLSecret,omitempty"`

	// ReadyCount of Manila API instance
	ManilaAPIReadyCount int32 `json:"manilaAPIReadyCount,omitempty"`

	// ReadyCount of Manila Scheduler instance
	ManilaSchedulerReadyCount int32 `json:"manilaSchedulerReadyCount,omitempty"`

	// ReadyCounts of Manila Share instances
	ManilaSharesReadyCounts map[string]int32 `json:"manilaSharesReadyCounts,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

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

// DBPurge struct is used to model the parameters exposed to the Manila API CronJob
type DBPurge struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=30
	// Age is the DBPurgeAge parameter and indicates the number of days of purging DB records
	Age int `json:"age"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="1 0 * * *"
	//Schedule defines the crontab format string to schedule the DBPurge cronJob
	Schedule string `json:"schedule"`
}

// ManilaDebug contains flags related to multiple debug activities. See the
// individual comments for what this means for each flag.
type ManilaDebug struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// DBPurge increases log verbosity by executing the db_purge command with "--debug".
	DBPurge bool `json:"dbPurge,omitempty"`
}

// IsReady - returns true if Manila is reconciled successfully
func (instance Manila) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}

// ManilaExtraVolMounts exposes additional parameters processed by the manila-operator
// and defines the common VolMounts structure provided by the main storage module
type ManilaExtraVolMounts struct {
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:Required
	VolMounts []storage.VolMounts `json:"extraVol"`
}

// Propagate is a function used to filter VolMounts according to the specified
// PropagationType array
func (c *ManilaExtraVolMounts) Propagate(svc []storage.PropagationType) []storage.VolMounts {

	var vl []storage.VolMounts

	for _, gv := range c.VolMounts {
		vl = append(vl, gv.Propagate(svc)...)
	}

	return vl
}

// RbacConditionsSet - set the conditions for the rbac object
func (instance Manila) RbacConditionsSet(c *condition.Condition) {
	instance.Status.Conditions.Set(c)
}

// RbacNamespace - return the namespace
func (instance Manila) RbacNamespace() string {
	return instance.Namespace
}

// RbacResourceName - return the name to be used for rbac objects (serviceaccount, role, rolebinding)
func (instance Manila) RbacResourceName() string {
	return "manila-" + instance.Name
}
