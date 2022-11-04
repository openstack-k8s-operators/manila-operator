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

const (
	// ServiceName -
	ServiceName = "manila"
	// ServiceNameV2 -
	ServiceNameV2 = "manilav2"
	// ServiceType -
	ServiceType = "manila"
	// ServiceTypeV2 -
	ServiceTypeV2 = "sharev2"
	// DatabaseName -
	DatabaseName = "manila"
	// ServiceAccount -
	ServiceAccount = "manila-operator-manila"

	// ManilaAdminPort -
	ManilaAdminPort int32 = 8786
	// ManilaPublicPort -
	ManilaPublicPort int32 = 8786
	// ManilaInternalPort -
	ManilaInternalPort int32 = 8786

	// KollaConfigDbSync -
	KollaConfigDbSync = "/var/lib/config-data/merged/db-sync-config.json"
)
