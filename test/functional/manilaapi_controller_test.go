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
package functional

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	"k8s.io/utils/ptr"
)

var _ = Describe("ManilaAPI controller", func() {
	var memcachedSpec memcachedv1.MemcachedSpec

	BeforeEach(func() {
		memcachedSpec = memcachedv1.MemcachedSpec{
			Replicas: ptr.To(int32(3)),
		}
		apiSpec := GetDefaultManilaAPISpec()
		apiSpec["customServiceConfig"] = "foo=bar"
		DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, manilaTest.MemcachedInstance, memcachedSpec))
		DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
		DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetManilaSpec(apiSpec)))
		DeferCleanup(th.DeleteInstance, CreateManilaAPI(manilaTest.Instance, GetDefaultManilaAPISpec()))
		DeferCleanup(
			mariadb.DeleteDBService,
			mariadb.CreateDBService(
				namespace,
				GetManila(manilaTest.Instance).Spec.DatabaseInstance,
				corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 3306}},
				},
			),
		)
		mariadb.CreateMariaDBDatabase(manilaTest.ManilaDatabaseName.Namespace, manilaTest.ManilaDatabaseName.Name, mariadbv1.MariaDBDatabaseSpec{})

		dbAccount, dbSecret := mariadb.CreateMariaDBAccountAndSecret(manilaTest.ManilaDatabaseAccount, mariadbv1.MariaDBAccountSpec{})
		DeferCleanup(k8sClient.Delete, ctx, dbAccount)
		DeferCleanup(k8sClient.Delete, ctx, dbSecret)

		mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
		mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
	})

	When("ManilaAPI CR is created", func() {
		It("is not Ready", func() {
			th.ExpectCondition(
				manilaTest.Instance,
				ConditionGetterFunc(ManilaAPIConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})
		It("has empty Status fields", func() {
			instance := GetManilaAPI(manilaTest.Instance)
			Expect(instance.Status.Hash).To(BeEmpty())
			Expect(instance.Status.ReadyCount).To(Equal(int32(0)))
		})
	})

	When("an unrelated Secret is created the CR state does not change", func() {
		BeforeEach(func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "not-relevant-secret",
					Namespace: manilaTest.Instance.Namespace,
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, secret)
		})
		It("is not Ready", func() {
			th.ExpectCondition(
				manilaTest.Instance,
				ConditionGetterFunc(ManilaAPIConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})
	})
	When("the Secret is created with all the expected fields", func() {
		BeforeEach(func() {
			infra.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			infra.SimulateMemcachedReady(manilaTest.ManilaMemcached)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(namespace))
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
		})
		It("a manila-api-config-secret containing the config has been created", func() {
			Eventually(func() corev1.Secret {
				return th.GetSecret(manilaTest.ManilaAPIConfigSecret)
			}, timeout, interval).ShouldNot(BeNil())
			secretDataMap := th.GetSecret(manilaTest.ManilaAPIConfigSecret)
			Expect(secretDataMap).ShouldNot(BeNil())
			// We apply customServiceConfig to the ManilaAPI Pod
			Expect(secretDataMap.Data).Should(HaveKey("03-config.conf"))
			//Double check customServiceConfig has been applied
			configData := string(secretDataMap.Data["03-config.conf"])
			Expect(configData).Should(ContainSubstring("foo=bar"))

			Expect(secretDataMap.Data).Should(HaveKey("my.cnf"))
			configData = string(secretDataMap.Data["my.cnf"])
			Expect(configData).To(
				ContainSubstring("[client]\nssl=0"))
		})

		When("manila-api-config is ready", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateManilaAPI(manilaTest.Instance, GetDefaultManilaAPISpec()))
				th.SimulateJobSuccess(manilaTest.ManilaDBSync)
				keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
				keystone.SimulateKeystoneEndpointReady(manilaTest.Instance)
			})
			It("creates a StatefulSet for manila-api service", func() {
				th.SimulateStatefulSetReplicaReady(manilaTest.ManilaAPI)
				ss := th.GetStatefulSet(manilaTest.ManilaAPI)
				// Check the resulting deployment fields
				Expect(int(*ss.Spec.Replicas)).To(Equal(1))
				Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(6))
				Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(2))

				container := ss.Spec.Template.Spec.Containers[1]
				Expect(container.VolumeMounts).To(HaveLen(8))
				Expect(container.Image).To(Equal(manilaTest.ContainerImage))
				Expect(container.LivenessProbe.HTTPGet.Port.IntVal).To(Equal(int32(8786)))
				Expect(container.ReadinessProbe.HTTPGet.Port.IntVal).To(Equal(int32(8786)))
			})
			It("stored the input hash in the Status", func() {
				Eventually(func(g Gomega) {
					manilaAPI := GetManilaAPI(manilaTest.ManilaAPI)
					g.Expect(manilaAPI.Status.Hash).Should((HaveKeyWithValue("input", Not(BeEmpty()))))
				}, timeout, interval).Should(Succeed())
			})
			It("reports that input is ready", func() {
				th.ExpectCondition(
					manilaTest.Instance,
					ConditionGetterFunc(ManilaAPIConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionTrue,
				)
			})
		})

		When("manila-api statefulset is ready", func() {
			BeforeEach(func() {
				th.SimulateStatefulSetReplicaReady(manilaTest.ManilaAPI)
				keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
			})
			It("reports that StatefulSet Condition is ready", func() {
				th.ExpectCondition(
					manilaTest.ManilaAPI,
					ConditionGetterFunc(ManilaAPIConditionGetter),
					condition.DeploymentReadyCondition,
					corev1.ConditionTrue,
				)
				// StatefulSet is Ready, check the actual ReadyCount is > 0
				manilaAPI := GetManilaAPI(manilaTest.ManilaAPI)
				Expect(manilaAPI.Status.ReadyCount).To(BeNumerically(">", 0))
			})
			It("exposes both internal and public services", func() {
				apiInstance := th.GetService(manilaTest.ManilaServicePublic)
				Expect(apiInstance.Labels["service"]).To(Equal(manilaTest.Instance.Name))
			})
			It("creates KeystoneEndpoint", func() {
				th.ExpectCondition(
					manilaTest.ManilaAPI,
					ConditionGetterFunc(ManilaAPIConditionGetter),
					condition.KeystoneEndpointReadyCondition,
					corev1.ConditionTrue,
				)
				keystoneEndpoint := keystone.GetKeystoneEndpoint(manilaTest.ManilaKeystoneEndpoint)
				endpoints := keystoneEndpoint.Spec.Endpoints
				// 2 entries are expected: internal and public
				Expect(endpoints).To(HaveLen(2))
				Expect(endpoints).To(HaveKeyWithValue("public", "http://manila-public."+manilaTest.Instance.Namespace+".svc:8786/v2"))
				Expect(endpoints).To(HaveKeyWithValue("internal", "http://manila-internal."+manilaTest.Instance.Namespace+".svc:8786/v2"))
			})
		})
	})
})
