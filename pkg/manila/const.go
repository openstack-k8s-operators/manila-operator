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

package manila

import (
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
)

const (
	// ServiceName - API V1 service name, deprecated
	ServiceName = "manila"
	// ServiceType - API V1 service type, deprecated
	ServiceType = "share"
	// ServiceNameV2 - API V2 service name, supported
	ServiceNameV2 = "manilav2"
	// ServiceTypeV2 - API V2 service type, supported
	ServiceTypeV2 = "sharev2"
	// DatabaseName -
	DatabaseName = "manila"

	// ManilaAdminPort -
	ManilaAdminPort int32 = 8786
	// ManilaPublicPort -
	ManilaPublicPort int32 = 8786
	// ManilaInternalPort -
	ManilaInternalPort int32 = 8786

	// KollaConfigDbSync -
	KollaConfigDbSync = "/var/lib/config-data/merged/db-sync-config.json"

	// ManilaExtraVolTypeUndefined can be used to label an extraMount which
	// is not associated with a specific backend
	ManilaExtraVolTypeUndefined storage.ExtraVolType = "Undefined"
	// ManilaExtraVolTypeCeph can be used to label an extraMount which
	// is associated to a Ceph backend
	ManilaExtraVolTypeCeph storage.ExtraVolType = "Ceph"
	// ManilaShare is the definition of the manila-share group
	ManilaShare storage.PropagationType = "ManilaShare"
	// ManilaScheduler is the definition of the manila-scheduler group
	ManilaScheduler storage.PropagationType = "ManilaScheduler"
	// ManilaAPI is the definition of the manila-api group
	ManilaAPI storage.PropagationType = "ManilaAPI"
	// Manila is the global ServiceType that refers to all the components deployed
	// by the manila operator
	Manila storage.PropagationType = "Manila"
)

// DbsyncPropagation keeps track of the DBSync Service Propagation Type
var DbsyncPropagation = []storage.PropagationType{storage.DBSync}

// ManilaSchedulerPropagation is the  definition of the ManilaScheduler propagation group
// It allows the ManilaScheduler pod to mount volumes destined to Manila and ManilaScheduler
// ServiceTypes
var ManilaSchedulerPropagation = []storage.PropagationType{Manila, ManilaScheduler}

// ManilaAPIPropagation is the  definition of the ManilaAPI propagation group
// It allows the ManilaAPI pod to mount volumes destined to Manila and ManilaAPI
// ServiceTypes
var ManilaAPIPropagation = []storage.PropagationType{Manila, ManilaAPI}

// ManilaSharePropagation is the  definition of the ManilaShare propagation group
// It allows the ManilaShare pods to mount volumes destined to Manila and ManilaShare
// ServiceTypes
var ManilaSharePropagation = []storage.PropagationType{Manila, ManilaShare}
