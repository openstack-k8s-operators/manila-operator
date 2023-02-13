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

package v1beta1

import (
	condition "github.com/***REMOVED***-k8s-operators/lib-common/modules/common/condition"
)

//
// Manila Condition Types used by API objects.
//
const (
	// ManilaAPIReadyCondition Status=True condition which indicates if the ManilaAPI is configured and operational
	ManilaAPIReadyCondition condition.Type = "ManilaAPIReady"

	// ManilaSchedulerReadyCondition Status=True condition which indicates if the ManilaScheduler is configured and operational
	ManilaSchedulerReadyCondition condition.Type = "ManilaSchedulerReady"

	// ManilaShareReadyCondition Status=True condition which indicates if the ManilaShare is configured and operational
	ManilaShareReadyCondition condition.Type = "ManilaShareReady"
)

//
// Manila Reasons used by API objects.
//
const ()

//
// Common Messages used by API objects.
//
const (
	//
	// ManilaAPIReady condition messages
	//
	// ManilaAPIReadyInitMessage
	ManilaAPIReadyInitMessage = "ManilaAPI not started"

	// ManilaAPIReadyErrorMessage
	ManilaAPIReadyErrorMessage = "ManilaAPI error occured %s"

	//
	// ManilaSchedulerReady condition messages
	//
	// ManilaSchedulerReadyInitMessage
	ManilaSchedulerReadyInitMessage = "ManilaScheduler not started"

	// ManilaSchedulerReadyErrorMessage
	ManilaSchedulerReadyErrorMessage = "ManilaScheduler error occured %s"

	//
	// ManilaShareReady condition messages
	//
	// ManilaShareReadyInitMessage
	ManilaShareReadyInitMessage = "ManilaShare not started"

	// ManilaShareReadyErrorMessage
	ManilaShareReadyErrorMessage = "ManilaShare error occured %s"

	// ManilaShareReadyRunningMessage
	ManilaShareReadyRunningMessage = "ManilaShare deployments in progress"
)
