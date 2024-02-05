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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	util "github.com/openstack-k8s-operators/lib-common/modules/common/util"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
)

var _ = Describe("Manila controller", func() {
	var memcachedSpec memcachedv1.MemcachedSpec

	BeforeEach(func() {
		memcachedSpec = memcachedv1.MemcachedSpec{
			Replicas: ptr.To(int32(3)),
		}
	})

	When("Manila CR instance is created", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetDefaultManilaSpec()))
		})
		It("initializes the status fields", func() {
			Eventually(func(g Gomega) {
				glance := GetManila(manilaName)
				g.Expect(glance.Status.Conditions).To(HaveLen(14))

				g.Expect(glance.Status.DatabaseHostname).To(Equal(""))
			}, timeout*2, interval).Should(Succeed())
		})
		It("is not Ready", func() {
			th.ExpectCondition(
				manilaTest.Instance,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})
		It("should have the Spec fields initialized", func() {
			Manila := GetManila(manilaTest.Instance)
			Expect(Manila.Spec.DatabaseInstance).Should(Equal("openstack"))
			Expect(Manila.Spec.DatabaseUser).Should(Equal(manilaTest.ManilaDataBaseUser))
			Expect(Manila.Spec.MemcachedInstance).Should(Equal(manilaTest.MemcachedInstance))
			Expect(Manila.Spec.RabbitMqClusterName).Should(Equal(manilaTest.RabbitmqClusterName))
			Expect(Manila.Spec.ServiceUser).Should(Equal(manilaTest.ManilaServiceUser))
		})
		It("should have the Status fields initialized", func() {
			Manila := GetManila(manilaTest.Instance)
			Expect(Manila.Status.Hash).To(BeEmpty())
			Expect(Manila.Status.DatabaseHostname).To(Equal(""))
			Expect(Manila.Status.TransportURLSecret).To(Equal(""))
			Expect(Manila.Status.ManilaAPIReadyCount).To(Equal(int32(0)))
			Expect(Manila.Status.ManilaSchedulerReadyCount).To(Equal(int32(0)))
		})
		It("should have Unknown Conditions initialized", func() {
			for _, cond := range []condition.Type{
				condition.DBReadyCondition,
				condition.DBSyncReadyCondition,
				condition.InputReadyCondition,
				condition.MemcachedReadyCondition,
				manilav1.ManilaAPIReadyCondition,
				manilav1.ManilaSchedulerReadyCondition,
				manilav1.ManilaShareReadyCondition,
			} {
				th.ExpectCondition(
					manilaTest.Manila,
					ConditionGetterFunc(ManilaConditionGetter),
					cond,
					corev1.ConditionUnknown,
				)
			}
		})
		It("should have a finalizer", func() {
			// the reconciler loop adds the finalizer so we have to wait for
			// it to run
			Eventually(func() []string {
				return GetManila(manilaTest.Instance).Finalizers
			}, timeout, interval).Should(ContainElement("Manila"))
		})
		It("creates service account, role and rolebindig", func() {

			th.ExpectCondition(
				manilaName,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.ServiceAccountReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				manilaName,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.RoleReadyCondition,
				corev1.ConditionTrue,
			)
			role := th.GetRole(manilaTest.ManilaRole)
			Expect(role.Rules).To(HaveLen(2))
			Expect(role.Rules[0].Resources).To(Equal([]string{"securitycontextconstraints"}))
			Expect(role.Rules[1].Resources).To(Equal([]string{"pods"}))

			th.ExpectCondition(
				manilaName,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.RoleBindingReadyCondition,
				corev1.ConditionTrue,
			)

			sa := th.GetServiceAccount(manilaTest.ManilaSA)

			binding := th.GetRoleBinding(manilaTest.ManilaRoleBinding)
			Expect(binding.Subjects).To(HaveLen(1))
			Expect(binding.RoleRef.Name).To(Equal(role.Name))
			Expect(binding.Subjects[0].Name).To(Equal(sa.Name))
		})
	})
	When("Manila DB is created", func() {
		BeforeEach(func() {
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetDefaultManilaSpec()))
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
			infra.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, manilaTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(manilaTest.ManilaMemcached)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(namespace))
		})
		It("Should set DBReady Condition and set DatabaseHostname Status when DB is Created", func() {
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.Instance)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			Manila := GetManila(manilaTest.Instance)
			Expect(Manila.Status.DatabaseHostname).To(
				Equal(fmt.Sprintf("hostname-for-openstack.%s.svc", manilaTest.Instance.Namespace)))
			th.ExpectCondition(
				manilaName,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.DBReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				manilaName,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.DBSyncReadyCondition,
				corev1.ConditionFalse,
			)
		})
		It("Should fail if db-sync job fails when DB is Created", func() {
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.Instance)
			th.SimulateJobFailure(manilaTest.ManilaDBSync)
			th.ExpectCondition(
				manilaTest.Instance,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.DBReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				manilaTest.Instance,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.DBSyncReadyCondition,
				corev1.ConditionFalse,
			)
		})
		It("Does not create ManilaAPI", func() {
			ManilaAPINotExists(manilaTest.Instance)
		})
		It("Does not create ManilaScheduler", func() {
			ManilaSchedulerNotExists(manilaTest.Instance)
		})
		It("Does not create ManilaShare", func() {
			ManilaShareNotExists(manilaTest.Instance)
		})
	})
	When("Both TransportURL secret and osp-secret are available", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetDefaultManilaSpec()))
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
			DeferCleanup(mariadb.DeleteDBService, mariadb.CreateDBService(
				manilaTest.Instance.Namespace,
				GetManila(manilaName).Spec.DatabaseInstance,
				corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 3306}},
				},
			),
			)
			infra.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, "memcached", memcachedSpec))
			infra.SimulateMemcachedReady(manilaTest.ManilaMemcached)
		})
		It("should create config-data and scripts ConfigMaps", func() {
			keystoneAPI := keystone.CreateKeystoneAPI(manilaTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
			Eventually(func() corev1.Secret {
				return th.GetSecret(manilaTest.ManilaConfigSecret)
			}, timeout, interval).ShouldNot(BeNil())
			Eventually(func() corev1.Secret {
				return th.GetSecret(manilaTest.ManilaConfigScripts)
			}, timeout, interval).ShouldNot(BeNil())
		})
	})
	When("Manila CR is created without container images defined", func() {
		BeforeEach(func() {
			// ManilaEmptySpec is used to provide a standard Manila CR where no
			// field is customized
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetManilaEmptySpec()))
		})
		It("has the expected container image defaults", func() {
			manilaDefault := GetManila(manilaTest.Instance)
			Expect(manilaDefault.Spec.ManilaAPI.ManilaServiceTemplate.ContainerImage).To(Equal(util.GetEnvVar("RELATED_IMAGE_MANILA_API_IMAGE_URL_DEFAULT", manilav1.ManilaAPIContainerImage)))
			Expect(manilaDefault.Spec.ManilaScheduler.ManilaServiceTemplate.ContainerImage).To(Equal(util.GetEnvVar("RELATED_IMAGE_MANILA_SCHEDULER_IMAGE_URL_DEFAULT", manilav1.ManilaSchedulerContainerImage)))
			for _, share := range manilaDefault.Spec.ManilaShares {
				Expect(share.ContainerImage).To(Equal(util.GetEnvVar("RELATED_IMAGE_MANILA_SHARE_IMAGE_URL_DEFAULT", manilav1.ManilaShareContainerImage)))
			}
		})
	})
	When("All the Resources are ready", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetDefaultManilaSpec()))
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
			DeferCleanup(th.DeleteInstance, CreateManilaAPI(manilaTest.Instance, GetDefaultManilaAPISpec()))
			DeferCleanup(th.DeleteInstance, CreateManilaScheduler(manilaTest.Instance, GetDefaultManilaSchedulerSpec()))
			DeferCleanup(th.DeleteInstance, CreateManilaShare(manilaTest.Instance, GetDefaultManilaShareSpec()))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					manilaTest.Instance.Namespace,
					GetManila(manilaName).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			infra.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, manilaTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(manilaTest.ManilaMemcached)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(manilaTest.Instance.Namespace))
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.Instance)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			keystone.SimulateKeystoneServiceReady(manilaTest.Instance)
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
		})
		It("Creates ManilaAPI", func() {
			ManilaAPIExists(manilaTest.Instance)
		})
		It("Creates ManilaScheduler", func() {
			ManilaSchedulerExists(manilaTest.Instance)
		})
		It("Creates ManilaShare", func() {
			ManilaShareExists(manilaTest.Instance)
		})
		It("Assert Services are created", func() {
			th.AssertServiceExists(manilaTest.ManilaServicePublic)
			th.AssertServiceExists(manilaTest.ManilaServiceInternal)
		})
	})
	When("Manila CR instance is deleted", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetDefaultManilaSpec()))
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					manilaTest.Instance.Namespace,
					GetManila(manilaTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			infra.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, manilaTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(manilaTest.ManilaMemcached)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystone.CreateKeystoneAPI(manilaTest.Instance.Namespace))
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.Instance)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
		})
		It("removes the finalizers from the Manila DB", func() {
			keystone.SimulateKeystoneServiceReady(manilaTest.Instance)

			mDB := mariadb.GetMariaDBDatabase(manilaTest.Instance)
			Expect(mDB.Finalizers).To(ContainElement("Manila"))

			th.DeleteInstance(GetManila(manilaTest.Instance))

			mDB = mariadb.GetMariaDBDatabase(manilaTest.Instance)
			Expect(mDB.Finalizers).NotTo(ContainElement("Manila"))
		})
	})
	When("Manila CR instance is built with NAD", func() {
		BeforeEach(func() {
			nad := th.CreateNetworkAttachmentDefinition(manilaTest.InternalAPINAD)
			DeferCleanup(th.DeleteInstance, nad)
			serviceOverride := map[string]interface{}{}
			serviceOverride["internal"] = map[string]interface{}{
				"metadata": map[string]map[string]string{
					"annotations": {
						"metallb.universe.tf/address-pool":    "osp-internalapi",
						"metallb.universe.tf/allow-shared-ip": "osp-internalapi",
						"metallb.universe.tf/loadBalancerIPs": "internal-lb-ip-1,internal-lb-ip-2",
					},
					"labels": {
						"internal": "true",
						"service":  "nova",
					},
				},
				"spec": map[string]interface{}{
					"type": "LoadBalancer",
				},
			}

			rawSpec := map[string]interface{}{
				"secret":              SecretName,
				"databaseInstance":    "openstack",
				"rabbitMqClusterName": "rabbitmq",
				"manilaAPI": map[string]interface{}{
					"containerImage":     manilav1.ManilaAPIContainerImage,
					"networkAttachments": []string{"internalapi"},
					"override": map[string]interface{}{
						"service": serviceOverride,
					},
				},
				"manilaScheduler": map[string]interface{}{
					"containerImage":     manilav1.ManilaSchedulerContainerImage,
					"networkAttachments": []string{"internalapi"},
				},
				"manilaShares": map[string]interface{}{
					"share0": map[string]interface{}{
						"containerImage":     manilav1.ManilaShareContainerImage,
						"networkAttachments": []string{"internalapi"},
					},
				},
			}
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, rawSpec))
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
			DeferCleanup(
				mariadb.DeleteDBService,
				mariadb.CreateDBService(
					manilaTest.Instance.Namespace,
					GetManila(manilaTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			infra.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, manilaTest.MemcachedInstance, memcachedSpec))
			infra.SimulateMemcachedReady(manilaTest.ManilaMemcached)
			keystoneAPIName := keystone.CreateKeystoneAPI(manilaTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPIName)
			keystoneAPI := keystone.GetKeystoneAPI(keystoneAPIName)
			keystoneAPI.Status.APIEndpoints["internal"] = "http://keystone-internal-openstack.testing"
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Status().Update(ctx, keystoneAPI.DeepCopy())).Should(Succeed())
			}, timeout, interval).Should(Succeed())
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.Instance)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			keystone.SimulateKeystoneServiceReady(manilaTest.Instance)
		})
		It("Check the resulting endpoints of the generated sub-CRs", func() {
			th.SimulateStatefulSetReplicaReadyWithPods(
				manilaTest.ManilaAPI,
				map[string][]string{manilaName.Namespace + "/internalapi": {"10.0.0.1"}},
			)
			th.SimulateStatefulSetReplicaReadyWithPods(
				manilaTest.ManilaScheduler,
				map[string][]string{manilaName.Namespace + "/internalapi": {"10.0.0.1"}},
			)
			th.SimulateStatefulSetReplicaReadyWithPods(
				manilaTest.ManilaShares[0],
				map[string][]string{manilaName.Namespace + "/internalapi": {"10.0.0.1"}},
			)
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
			// Retrieve the generated resources
			manila := GetManila(manilaTest.Instance)
			api := GetManilaAPI(manilaTest.ManilaAPI)
			sched := GetManilaScheduler(manilaTest.ManilaScheduler)
			share := GetManilaShare(manilaTest.ManilaShares[0])
			// Check ManilaAPI NADs
			Expect(api.Spec.NetworkAttachments).To(Equal(manila.Spec.ManilaAPI.ManilaServiceTemplate.NetworkAttachments))
			// Check ManilaScheduler NADs
			Expect(sched.Spec.NetworkAttachments).To(Equal(manila.Spec.ManilaScheduler.ManilaServiceTemplate.NetworkAttachments))
			// Check ManilaShare exists
			ManilaShareExists(manilaTest.ManilaShares[0])
			// Check ManilaShare NADs
			Expect(share.Spec.NetworkAttachments).To(Equal(share.Spec.ManilaServiceTemplate.NetworkAttachments))

			// As the internal endpoint has service override configured it
			// gets a LoadBalancer Service with MetalLB annotations
			service := th.GetService(types.NamespacedName{
				Namespace: manila.Namespace,
				Name:      manila.Name + "-internal",
			})
			Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
			Expect(service.Annotations).To(
				HaveKeyWithValue("dnsmasq.network.openstack.org/hostname", "manila-internal."+manila.Namespace+".svc"))
			Expect(service.Annotations).To(
				HaveKeyWithValue("metallb.universe.tf/address-pool", "osp-internalapi"))
			Expect(service.Annotations).To(
				HaveKeyWithValue("metallb.universe.tf/allow-shared-ip", "osp-internalapi"))
			Expect(service.Annotations).To(
				HaveKeyWithValue("metallb.universe.tf/loadBalancerIPs", "internal-lb-ip-1,internal-lb-ip-2"))

			// check keystone endpoints for v1 and v2
			keystoneEndpoint := keystone.GetKeystoneEndpoint(types.NamespacedName{Namespace: manila.Namespace, Name: "manila"})
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("public", "http://manila-public."+manila.Namespace+".svc:8786/v1/%(project_id)s"))
			Expect(endpoints).To(HaveKeyWithValue("internal", "http://manila-internal."+manila.Namespace+".svc:8786/v1/%(project_id)s"))

			keystoneEndpoint = keystone.GetKeystoneEndpoint(types.NamespacedName{Namespace: manila.Namespace, Name: "manilav2"})
			endpoints = keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("public", "http://manila-public."+manila.Namespace+".svc:8786/v2"))
			Expect(endpoints).To(HaveKeyWithValue("internal", "http://manila-internal."+manila.Namespace+".svc:8786/v2"))
		})
	})
})
