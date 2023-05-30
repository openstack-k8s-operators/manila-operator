/*
Copyright 2023.

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

// Package functional implements the envTest coverage for manila-operator
package functional

import (
	"fmt"
	"k8s.io/apimachinery/pkg/types"
)

// ManilaTestData is the data structure used to provide input data to envTest
type ManilaTestData struct {
	RabbitmqClusterName    string
	RabbitmqSecretName     string
	ManilaDataBaseUser     string
	ManilaPassword         string
	ManilaServiceUser      string
	Instance               types.NamespacedName
	ManilaRole             types.NamespacedName
	ManilaRoleBinding      types.NamespacedName
	ManilaTransportURL     types.NamespacedName
	ManilaSA               types.NamespacedName
	ManilaDBSync           types.NamespacedName
	ManilaKeystoneEndpoint types.NamespacedName
	ManilaServicePublic    types.NamespacedName
	ManilaServiceInternal  types.NamespacedName
	ManilaConfigMapData    types.NamespacedName
	ManilaConfigMapScripts types.NamespacedName
	ManilaAPI              types.NamespacedName
	ManilaScheduler        types.NamespacedName
	ManilaShares           []types.NamespacedName
	InternalAPINAD         types.NamespacedName
}

// GetManilaTestData is a function that initialize the ManilaTestData
// used in the test
func GetManilaTestData(manilaName types.NamespacedName) ManilaTestData {

	m := manilaName
	return ManilaTestData{
		Instance: m,

		ManilaDBSync: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("%s-db-sync", manilaName.Name),
		},
		ManilaAPI: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("%s-api", manilaName.Name),
		},
		ManilaScheduler: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("%s-scheduler", manilaName.Name),
		},
		ManilaShares: []types.NamespacedName{
			{
				Namespace: manilaName.Namespace,
				Name:      fmt.Sprintf("%s-share-share1", manilaName.Name),
			},
			{
				Namespace: manilaName.Namespace,
				Name:      fmt.Sprintf("%s-share-share2", manilaName.Name),
			},
		},
		ManilaRole: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("manila-%s-role", manilaName.Name),
		},
		ManilaRoleBinding: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("manila-%s-rolebinding", manilaName.Name),
		},
		ManilaSA: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("manila-%s", manilaName.Name),
		},
		ManilaTransportURL: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("manila-%s-transport", manilaName.Name),
		},
		ManilaConfigMapData: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("%s-%s", manilaName.Name, "config-data"),
		},
		ManilaConfigMapScripts: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("%s-%s", manilaName.Name, "scripts"),
		},
		// Also used to identify ManilaRoutePublic
		ManilaServicePublic: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("%s-public", manilaName.Name),
		},
		// Also used to identify ManilaKeystoneService
		ManilaServiceInternal: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("%s-internal", manilaName.Name),
		},
		ManilaKeystoneEndpoint: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      fmt.Sprintf("%sv2", manilaName.Name),
		},
		InternalAPINAD: types.NamespacedName{
			Namespace: manilaName.Namespace,
			Name:      "internalapi",
		},
		RabbitmqClusterName: "rabbitmq",
		RabbitmqSecretName:  "rabbitmq-secret",
		ManilaDataBaseUser:  "manila",
		// Password used for both db and service
		ManilaPassword:    "12345678",
		ManilaServiceUser: "manila",
	}
}
