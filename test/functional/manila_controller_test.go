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
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	mariadb_test "github.com/openstack-k8s-operators/mariadb-operator/api/test/helpers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	util "github.com/openstack-k8s-operators/lib-common/modules/common/util"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
)

var _ = Describe("Manila controller", func() {
	var memcachedSpec memcachedv1.MemcachedSpec

	BeforeEach(func() {
		memcachedSpec = memcachedv1.MemcachedSpec{
			MemcachedSpecCore: memcachedv1.MemcachedSpecCore{
				Replicas: ptr.To[int32](3),
			},
		}
	})

	When("Manila CR instance is created", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetDefaultManilaSpec()))
		})
		It("initializes the status fields", func() {
			Eventually(func(g Gomega) {
				manila := GetManila(manilaName)
				g.Expect(manila.Status.Conditions).To(HaveLen(14))

				g.Expect(manila.Status.DatabaseHostname).To(Equal(""))
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
			Expect(Manila.Spec.DatabaseAccount).Should(Equal(manilaTest.ManilaDatabaseAccount.Name))
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
			}, timeout, interval).Should(ContainElement("openstack.org/manila"))
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
		})
		It("should create config-data and scripts ConfigMaps", func() {
			keystoneAPI := keystone.CreateKeystoneAPI(manilaTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)

			secretDataMap := th.GetSecret(manilaTest.ManilaConfigSecret)
			Expect(secretDataMap).ShouldNot(BeNil())
			myCnf := string(secretDataMap.Data["my.cnf"])
			Expect(myCnf).To(
				ContainSubstring("[client]\nssl=0"))
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
			Expect(manilaDefault.Spec.ManilaAPI.ContainerImage).To(Equal(util.GetEnvVar("RELATED_IMAGE_MANILA_API_IMAGE_URL_DEFAULT", manilav1.ManilaAPIContainerImage)))
			Expect(manilaDefault.Spec.ManilaScheduler.ContainerImage).To(Equal(util.GetEnvVar("RELATED_IMAGE_MANILA_SCHEDULER_IMAGE_URL_DEFAULT", manilav1.ManilaSchedulerContainerImage)))
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
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
		It("configures DB Purge job", func() {
			Eventually(func(g Gomega) {
				manila := GetManila(manilaTest.Instance)
				cron := GetCronJob(manilaTest.DBPurgeCronJob)
				g.Expect(cron.Spec.Schedule).To(Equal(manila.Spec.DBPurge.Schedule))
			}, timeout, interval).Should(Succeed())
		})
		It("update DB Purge job", func() {
			Eventually(func(g Gomega) {
				manila := GetManila(manilaTest.Instance)
				manila.Spec.DBPurge.Schedule = "*/30 * * * *"
				g.Expect(k8sClient.Update(ctx, manila)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				manila := GetManila(manilaTest.Instance)
				cron := GetCronJob(manilaTest.DBPurgeCronJob)
				g.Expect(cron.Spec.Schedule).To(Equal(manila.Spec.DBPurge.Schedule))
			}, timeout, interval).Should(Succeed())
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
		})
		It("removes the finalizers from the Manila DB", func() {
			keystone.SimulateKeystoneServiceReady(manilaTest.Instance)

			mDB := mariadb.GetMariaDBDatabase(manilaTest.ManilaDatabaseName)
			Expect(mDB.Finalizers).To(ContainElement("openstack.org/manila"))

			th.DeleteInstance(GetManila(manilaTest.Instance))

			mDB = mariadb.GetMariaDBDatabase(manilaTest.ManilaDatabaseName)
			Expect(mDB.Finalizers).NotTo(ContainElement("openstack.org/manila"))
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			th.SimulateLoadBalancerServiceIP(types.NamespacedName{
				Namespace: namespace,
				Name:      manilaTest.Instance.Name + "-internal",
			})
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
	When("A Manila with TLS is created", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetTLSManilaSpec()))
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
			mariadb.SimulateMariaDBTLSDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
		})

		It("reports that the CA secret is missing", func() {
			th.ExpectCondition(
				manilaTest.ManilaAPI,
				ConditionGetterFunc(ManilaAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionFalse,
			)

			th.ExpectCondition(
				manilaTest.ManilaScheduler,
				ConditionGetterFunc(ManilaSchedulerConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("reports that the internal cert secret is missing", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(manilaTest.CABundleSecret))
			th.ExpectCondition(
				manilaTest.ManilaAPI,
				ConditionGetterFunc(ManilaAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("reports that the public cert secret is missing", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(manilaTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(manilaTest.InternalCertSecret))
			th.ExpectCondition(
				manilaTest.ManilaAPI,
				ConditionGetterFunc(ManilaAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("should create config-data and scripts ConfigMaps", func() {
			keystoneAPI := keystone.CreateKeystoneAPI(manilaTest.Instance.Namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)

			secretDataMap := th.GetSecret(manilaTest.ManilaConfigSecret)
			Expect(secretDataMap).ShouldNot(BeNil())
			myCnf := string(secretDataMap.Data["my.cnf"])
			Expect(myCnf).To(
				ContainSubstring("[client]\nssl-ca=/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem\nssl=1"))
			Eventually(func() corev1.Secret {
				return th.GetSecret(manilaTest.ManilaConfigScripts)
			}, timeout, interval).ShouldNot(BeNil())
		})

		It("Creates ManilaAPI", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(manilaTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(manilaTest.InternalCertSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(manilaTest.PublicCertSecret))
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)

			ManilaAPIExists(manilaTest.ManilaAPI)

			th.ExpectCondition(
				manilaTest.ManilaAPI,
				ConditionGetterFunc(ManilaAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionTrue,
			)

			d := th.GetStatefulSet(manilaTest.ManilaAPI)
			// Check the resulting deployment fieldsq
			Expect(int(*d.Spec.Replicas)).To(Equal(1))

			Expect(d.Spec.Template.Spec.Volumes).To(HaveLen(9))
			Expect(d.Spec.Template.Spec.Containers).To(HaveLen(2))

			// cert deployment volumes
			th.AssertVolumeExists(manilaTest.CABundleSecret.Name, d.Spec.Template.Spec.Volumes)
			th.AssertVolumeExists(manilaTest.InternalCertSecret.Name, d.Spec.Template.Spec.Volumes)
			th.AssertVolumeExists(manilaTest.PublicCertSecret.Name, d.Spec.Template.Spec.Volumes)

			// cert volumeMounts
			container := d.Spec.Template.Spec.Containers[1]
			th.AssertVolumeMountExists(manilaTest.InternalCertSecret.Name, "tls.key", container.VolumeMounts)
			th.AssertVolumeMountExists(manilaTest.InternalCertSecret.Name, "tls.crt", container.VolumeMounts)
			th.AssertVolumeMountExists(manilaTest.PublicCertSecret.Name, "tls.key", container.VolumeMounts)
			th.AssertVolumeMountExists(manilaTest.PublicCertSecret.Name, "tls.crt", container.VolumeMounts)
			th.AssertVolumeMountExists(manilaTest.CABundleSecret.Name, "tls-ca-bundle.pem", container.VolumeMounts)

			Expect(container.ReadinessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
			Expect(container.LivenessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
		})

		It("reconfigures the manila pods when CA changes", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(manilaTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(manilaTest.InternalCertSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(manilaTest.PublicCertSecret))
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)

			ManilaAPIExists(manilaTest.Instance)
			ManilaSchedulerExists(manilaTest.Instance)

			th.ExpectCondition(
				manilaTest.ManilaAPI,
				ConditionGetterFunc(ManilaAPIConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionTrue,
			)

			th.ExpectCondition(
				manilaTest.ManilaScheduler,
				ConditionGetterFunc(ManilaSchedulerConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionTrue,
			)

			// Grab the current config hash
			apiOriginalHash := GetEnvVarValue(
				th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.Containers[0].Env, "CONFIG_HASH", "")
			Expect(apiOriginalHash).NotTo(BeEmpty())
			schedulerOriginalHash := GetEnvVarValue(
				th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.Containers[0].Env, "CONFIG_HASH", "")
			Expect(schedulerOriginalHash).NotTo(BeEmpty())

			// Change the content of the CA secret
			th.UpdateSecret(manilaTest.CABundleSecret, "tls-ca-bundle.pem", []byte("DifferentCAData"))

			// Assert that the deployment is updated
			Eventually(func(g Gomega) {
				newHash := GetEnvVarValue(
					th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.Containers[0].Env, "CONFIG_HASH", "")
				g.Expect(newHash).NotTo(BeEmpty())
				g.Expect(newHash).NotTo(Equal(apiOriginalHash))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				newHash := GetEnvVarValue(
					th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.Containers[0].Env, "CONFIG_HASH", "")
				g.Expect(newHash).NotTo(BeEmpty())
				g.Expect(newHash).NotTo(Equal(schedulerOriginalHash))
			}, timeout, interval).Should(Succeed())
		})

		It("Assert Services are created", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(manilaTest.CABundleSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(manilaTest.InternalCertSecret))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(manilaTest.PublicCertSecret))
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)

			th.AssertServiceExists(manilaTest.ManilaServicePublic)
			th.AssertServiceExists(manilaTest.ManilaServiceInternal)

			// check keystone endpoints
			keystoneEndpoint := keystone.GetKeystoneEndpoint(manilaTest.ManilaKeystoneEndpoint)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("public", "https://manila-public."+namespace+".svc:8786/v2"))
			Expect(endpoints).To(HaveKeyWithValue("internal", "https://manila-internal."+namespace+".svc:8786/v2"))
		})
	})

	When("Manila is created with topologyRef", func() {
		var topologyRef, topologyRefAlt *topologyv1.TopoRef
		BeforeEach(func() {
			// Create Test Topologies
			for _, t := range manilaTest.ManilaTopologies {
				// Build the topology Spec
				topologySpec, _ := GetSampleTopologySpec(t.Name)
				infra.CreateTopology(t, topologySpec)
			}
			spec := GetDefaultManilaSpec()

			topologyRef = &topologyv1.TopoRef{
				Name:      manilaTest.ManilaTopologies[0].Name,
				Namespace: manilaTest.ManilaTopologies[0].Namespace,
			}
			topologyRefAlt = &topologyv1.TopoRef{
				Name:      manilaTest.ManilaTopologies[1].Name,
				Namespace: manilaTest.ManilaTopologies[1].Namespace,
			}
			spec["topologyRef"] = map[string]interface{}{
				"name": topologyRef.Name,
			}
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, spec))
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			keystone.SimulateKeystoneServiceReady(manilaTest.Instance)
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
			th.SimulateStatefulSetReplicaReady(manilaTest.ManilaAPI)
			th.SimulateStatefulSetReplicaReady(manilaTest.ManilaScheduler)
			th.SimulateStatefulSetReplicaReady(manilaTest.ManilaShares[0])
		})

		It("sets topology in CR status", func() {
			expectedTopology := &topologyv1.TopoRef{
				Name:      topologyRef.Name,
				Namespace: topologyRef.Namespace,
			}
			var finalizers []string
			Eventually(func(g Gomega) {
				tp := infra.GetTopology(types.NamespacedName{
					Name:      expectedTopology.Name,
					Namespace: expectedTopology.Namespace,
				})
				g.Expect(tp.GetFinalizers()).To(HaveLen(3))
				finalizers = tp.GetFinalizers()

				manilaAPI := GetManilaAPI(manilaTest.ManilaAPI)
				g.Expect(manilaAPI.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(manilaAPI.Status.LastAppliedTopology).To(Equal(expectedTopology))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/manilaapi-%s", manilaAPI.Name)))

				manilaScheduler := GetManilaScheduler(manilaTest.ManilaScheduler)
				g.Expect(manilaScheduler.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(manilaScheduler.Status.LastAppliedTopology).To(Equal(expectedTopology))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/manilascheduler-%s", manilaScheduler.Name)))

				manilaShare := GetManilaShare(manilaTest.ManilaShares[0])
				g.Expect(manilaShare.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(manilaShare.Status.LastAppliedTopology).To(Equal(expectedTopology))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/manilashare-%s", manilaShare.Name)))
			}, timeout, interval).Should(Succeed())
		})
		It("sets Topology in resource specs", func() {
			Eventually(func(g Gomega) {
				_, expectedTopologySpecObj := GetSampleTopologySpec(topologyRef.Name)
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.Affinity).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.TopologySpreadConstraints).ToNot(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.TopologySpreadConstraints).To(Equal(expectedTopologySpecObj))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.Affinity).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.TopologySpreadConstraints).ToNot(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.TopologySpreadConstraints).To(Equal(expectedTopologySpecObj))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.Affinity).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.TopologySpreadConstraints).ToNot(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.TopologySpreadConstraints).To(Equal(expectedTopologySpecObj))
			}, timeout, interval).Should(Succeed())
		})
		It("updates topology when the reference changes", func() {
			expectedTopology := &topologyv1.TopoRef{
				Name:      topologyRefAlt.Name,
				Namespace: topologyRefAlt.Namespace,
			}
			var finalizers []string
			Eventually(func(g Gomega) {
				manila := GetManila(manilaTest.Instance)
				manila.Spec.TopologyRef.Name = expectedTopology.Name
				g.Expect(k8sClient.Update(ctx, manila)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				tp := infra.GetTopology(types.NamespacedName{
					Name:      expectedTopology.Name,
					Namespace: expectedTopology.Namespace,
				})
				finalizers = tp.GetFinalizers()
				g.Expect(finalizers).To(HaveLen(3))

				manilaAPI := GetManilaAPI(manilaTest.ManilaAPI)
				g.Expect(manilaAPI.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(manilaAPI.Status.LastAppliedTopology).To(Equal(expectedTopology))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/manilaapi-%s", manilaAPI.Name)))

				manilaScheduler := GetManilaScheduler(manilaTest.ManilaScheduler)
				g.Expect(manilaScheduler.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(manilaScheduler.Status.LastAppliedTopology).To(Equal(expectedTopology))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/manilascheduler-%s", manilaScheduler.Name)))

				manilaShare := GetManilaShare(manilaTest.ManilaShares[0])
				g.Expect(manilaShare.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(manilaShare.Status.LastAppliedTopology).To(Equal(expectedTopology))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/manilashare-%s", manilaShare.Name)))

				// Get the previous topology and verify there are no finalizers
				// anymore
				tp = infra.GetTopology(types.NamespacedName{
					Name:      topologyRef.Name,
					Namespace: topologyRef.Namespace,
				})
				g.Expect(tp.GetFinalizers()).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
		It("overrides topology when the reference changes", func() {
			Eventually(func(g Gomega) {
				manila := GetManila(manilaTest.Instance)
				//Patch ManilaAPI Spec
				newAPI := GetManilaAPISpec(manilaTest.ManilaAPI)
				newAPI.TopologyRef.Name = manilaTest.ManilaTopologies[1].Name
				manila.Spec.ManilaAPI = newAPI
				//Patch ManilaScheduler Spec
				newSch := GetManilaSchedulerSpec(manilaTest.ManilaScheduler)
				newSch.TopologyRef.Name = manilaTest.ManilaTopologies[2].Name
				manila.Spec.ManilaScheduler = newSch
				//Patch ManilaShare (share0) Spec
				newSh := GetManilaShareSpec(manilaTest.ManilaShares[0])
				newSh.TopologyRef.Name = manilaTest.ManilaTopologies[3].Name
				manila.Spec.ManilaShares["share0"] = newSh
				g.Expect(k8sClient.Update(ctx, manila)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				expectedTopology := &topologyv1.TopoRef{
					Name:      manilaTest.ManilaTopologies[1].Name,
					Namespace: manilaTest.ManilaTopologies[1].Namespace,
				}
				tp := infra.GetTopology(types.NamespacedName{
					Name:      expectedTopology.Name,
					Namespace: expectedTopology.Namespace,
				})
				finalizers := tp.GetFinalizers()
				g.Expect(finalizers).To(HaveLen(1))

				manilaAPI := GetManilaAPI(manilaTest.ManilaAPI)
				g.Expect(manilaAPI.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(manilaAPI.Status.LastAppliedTopology).To(Equal(expectedTopology))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/manilaapi-%s", manilaAPI.Name)))

			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				expectedTopology := &topologyv1.TopoRef{
					Name:      manilaTest.ManilaTopologies[2].Name,
					Namespace: manilaTest.ManilaTopologies[2].Namespace,
				}
				tp := infra.GetTopology(types.NamespacedName{
					Name:      expectedTopology.Name,
					Namespace: expectedTopology.Namespace,
				})
				finalizers := tp.GetFinalizers()
				g.Expect(finalizers).To(HaveLen(1))
				manilaScheduler := GetManilaScheduler(manilaTest.ManilaScheduler)
				g.Expect(manilaScheduler.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(manilaScheduler.Status.LastAppliedTopology).To(Equal(expectedTopology))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/manilascheduler-%s", manilaScheduler.Name)))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				expectedTopology := &topologyv1.TopoRef{
					Name:      manilaTest.ManilaTopologies[3].Name,
					Namespace: manilaTest.ManilaTopologies[3].Namespace,
				}
				tp := infra.GetTopology(types.NamespacedName{
					Name:      expectedTopology.Name,
					Namespace: expectedTopology.Namespace,
				})
				g.Expect(tp.GetFinalizers()).To(HaveLen(1))
				finalizers := tp.GetFinalizers()
				manilaShare := GetManilaShare(manilaTest.ManilaShares[0])
				g.Expect(manilaShare.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(manilaShare.Status.LastAppliedTopology).To(Equal(expectedTopology))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/manilashare-%s", manilaShare.Name)))
			}, timeout, interval).Should(Succeed())
		})
		It("removes topology from the spec", func() {
			Eventually(func(g Gomega) {
				manila := GetManila(manilaTest.Instance)
				// Remove the TopologyRef from the existing Manila .Spec
				manila.Spec.TopologyRef = nil
				g.Expect(k8sClient.Update(ctx, manila)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				manilaAPI := GetManilaAPI(manilaTest.ManilaAPI)
				g.Expect(manilaAPI.Status.LastAppliedTopology).Should(BeNil())
				manilaScheduler := GetManilaScheduler(manilaTest.ManilaScheduler)
				g.Expect(manilaScheduler.Status.LastAppliedTopology).Should(BeNil())
				manilaShare := GetManilaShare(manilaTest.ManilaShares[0])
				g.Expect(manilaShare.Status.LastAppliedTopology).Should(BeNil())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.TopologySpreadConstraints).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.Affinity).ToNot(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.TopologySpreadConstraints).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.Affinity).ToNot(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.TopologySpreadConstraints).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.Affinity).ToNot(BeNil())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				for _, topology := range manilaTest.ManilaTopologies {
					// Get the current topology and verify there are no finalizers
					tp := infra.GetTopology(types.NamespacedName{
						Name:      topology.Name,
						Namespace: topology.Namespace,
					})
					g.Expect(tp.GetFinalizers()).To(BeEmpty())
				}
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A Manila is created with nodeSelector", func() {
		BeforeEach(func() {
			spec := GetDefaultManilaSpec()
			spec["nodeSelector"] = map[string]interface{}{
				"foo": "bar",
			}
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, spec))
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			keystone.SimulateKeystoneServiceReady(manilaTest.Instance)
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
			th.SimulateStatefulSetReplicaReady(manilaTest.ManilaAPI)
			th.SimulateStatefulSetReplicaReady(manilaTest.ManilaScheduler)
			th.SimulateStatefulSetReplicaReady(manilaTest.ManilaShares[0])
		})

		It("sets nodeSelector in resource specs", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
		})

		It("updates nodeSelector in resource specs when changed", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				manila := GetManila(manilaName)
				newNodeSelector := map[string]string{
					"foo2": "bar2",
				}
				manila.Spec.NodeSelector = &newNodeSelector
				g.Expect(k8sClient.Update(ctx, manila)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateJobSuccess(manilaTest.ManilaDBSync)
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
			}, timeout, interval).Should(Succeed())
		})

		It("removes nodeSelector from resource specs when cleared", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				manila := GetManila(manilaName)
				emptyNodeSelector := map[string]string{}
				manila.Spec.NodeSelector = &emptyNodeSelector
				g.Expect(k8sClient.Update(ctx, manila)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateJobSuccess(manilaTest.ManilaDBSync)
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(BeNil())
			}, timeout, interval).Should(Succeed())
		})

		It("removes nodeSelector from resource specs when nilled", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				manila := GetManila(manilaName)
				manila.Spec.NodeSelector = nil
				g.Expect(k8sClient.Update(ctx, manila)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateJobSuccess(manilaTest.ManilaDBSync)
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(BeNil())
			}, timeout, interval).Should(Succeed())
		})

		It("allows nodeSelector service override", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				manila := GetManila(manilaName)
				apiNodeSelector := map[string]string{
					"foo": "api",
				}
				manila.Spec.ManilaAPI.NodeSelector = &apiNodeSelector
				schedulerNodeSelector := map[string]string{
					"foo": "scheduler",
				}
				manila.Spec.ManilaScheduler.NodeSelector = &schedulerNodeSelector
				shareNodeSelector := map[string]string{
					"foo": "share",
				}
				share := manila.Spec.ManilaShares["share0"]
				share.NodeSelector = &shareNodeSelector
				manila.Spec.ManilaShares["share0"] = share
				g.Expect(k8sClient.Update(ctx, manila)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateJobSuccess(manilaTest.ManilaDBSync)
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "api"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "scheduler"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "share"}))
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
		})

		It("allows nodeSelector service override to empty", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				manila := GetManila(manilaName)
				apiNodeSelector := map[string]string{}
				manila.Spec.ManilaAPI.NodeSelector = &apiNodeSelector
				schedulerNodeSelector := map[string]string{}
				manila.Spec.ManilaScheduler.NodeSelector = &schedulerNodeSelector
				shareNodeSelector := map[string]string{}
				share := manila.Spec.ManilaShares["share0"]
				share.NodeSelector = &shareNodeSelector
				manila.Spec.ManilaShares["share0"] = share
				g.Expect(k8sClient.Update(ctx, manila)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateJobSuccess(manilaTest.ManilaDBSync)
				g.Expect(th.GetStatefulSet(manilaTest.ManilaAPI).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaScheduler).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetStatefulSet(manilaTest.ManilaShares[0]).Spec.Template.Spec.NodeSelector).To(BeNil())
				g.Expect(th.GetJob(manilaTest.ManilaDBSync).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
				g.Expect(GetCronJob(manilaTest.DBPurgeCronJob).Spec.JobTemplate.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("Manila CR instance is built with ExtraMounts", func() {
		BeforeEach(func() {
			rawSpec := map[string]interface{}{
				"secret":              SecretName,
				"databaseInstance":    "openstack",
				"rabbitMqClusterName": "rabbitmq",
				"extraMounts":         GetExtraMounts(),
				"manilaAPI": map[string]interface{}{
					"containerImage": manilav1.ManilaAPIContainerImage,
				},
				"manilaScheduler": map[string]interface{}{
					"containerImage": manilav1.ManilaSchedulerContainerImage,
				},
				"manilaShares": map[string]interface{}{
					"share0": map[string]interface{}{
						"containerImage": manilav1.ManilaShareContainerImage,
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(manilaTest.ManilaDatabaseAccount)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			keystone.SimulateKeystoneServiceReady(manilaTest.Instance)
		})
		It("Check the extraMounts of the resulting StatefulSets", func() {
			th.SimulateStatefulSetReplicaReady(manilaTest.ManilaAPI)
			th.SimulateStatefulSetReplicaReady(manilaTest.ManilaScheduler)
			th.SimulateStatefulSetReplicaReady(manilaTest.ManilaShares[0])
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
			// Retrieve the generated resources
			share := manilaTest.ManilaShares[0]
			th.SimulateStatefulSetReplicaReady(share)
			ss := th.GetStatefulSet(share)
			// Check the resulting deployment fields
			Expect(int(*ss.Spec.Replicas)).To(Equal(1))
			Expect(ss.Spec.Template.Spec.Volumes).To(HaveLen(7))
			Expect(ss.Spec.Template.Spec.Containers).To(HaveLen(2))
			// Get the manila-share container
			container := ss.Spec.Template.Spec.Containers[1]
			// Fail if manila-share doesn't have the right number of
			// VolumeMounts entries
			Expect(container.VolumeMounts).To(HaveLen(9))
			// Inspect VolumeMounts and make sure we have the Ceph MountPath
			// provided through extraMounts
			for _, vm := range container.VolumeMounts {
				if vm.Name == "ceph" {
					Expect(vm.MountPath).To(
						ContainSubstring(ManilaCephExtraMountsPath))
				}
			}
		})
	})

	// Run MariaDBAccount suite tests.  these are pre-packaged ginkgo tests
	// that exercise standard account create / update patterns that should be
	// common to all controllers that ensure MariaDBAccount CRs.
	mariadbSuite := &mariadb_test.MariaDBTestHarness{
		PopulateHarness: func(harness *mariadb_test.MariaDBTestHarness) {
			harness.Setup(
				"Manila",
				manilaTest.Instance.Namespace,
				manilaTest.Instance.Name,
				"openstack.org/manila",
				mariadb, timeout, interval,
			)
		},

		// Generate a fully running service given an accountName
		// needs to make it all the way to the end where the mariadb finalizers
		// are removed from unused accounts since that's part of what we are testing
		SetupCR: func(accountName types.NamespacedName) {
			memcachedSpec = memcachedv1.MemcachedSpec{
				MemcachedSpecCore: memcachedv1.MemcachedSpecCore{
					Replicas: ptr.To[int32](3),
				},
			}

			spec := GetDefaultManilaSpec()
			spec["databaseAccount"] = accountName.Name
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, spec))
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
			mariadb.SimulateMariaDBDatabaseCompleted(manilaTest.ManilaDatabaseName)
			mariadb.SimulateMariaDBAccountCompleted(accountName)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			keystone.SimulateKeystoneServiceReady(manilaTest.Instance)
			keystone.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
		},
		// Change the account name in the service to a new name
		UpdateAccount: func(newAccountName types.NamespacedName) {

			Eventually(func(g Gomega) {
				manila := GetManila(manilaName)
				manila.Spec.DatabaseAccount = newAccountName.Name
				g.Expect(th.K8sClient.Update(ctx, manila)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

		},
		// delete the CR instance to exercise finalizer removal
		DeleteCR: func() {
			th.DeleteInstance(GetManila(manilaName))
		},
	}

	mariadbSuite.RunBasicSuite()

	mariadbSuite.RunURLAssertSuite(func(_ types.NamespacedName, username string, password string) {
		Eventually(func(g Gomega) {
			secretDataMap := th.GetSecret(manilaTest.ManilaConfigSecret)

			conf := secretDataMap.Data["00-config.conf"]

			g.Expect(string(conf)).Should(
				ContainSubstring(fmt.Sprintf("connection = mysql+pymysql://%s:%s@hostname-for-openstack.%s.svc/%s?read_default_file=/etc/my.cnf",
					username, password, namespace, manila.DatabaseName)))

		}).Should(Succeed())

	})

})

var _ = Describe("Manila Webhook", func() {

	BeforeEach(func() {
		err := os.Setenv("OPERATOR_TEMPLATES", "../../templates")
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects with wrong ManilaAPI service override endpoint type", func() {
		spec := GetDefaultManilaSpec()
		apiSpec := GetDefaultManilaAPISpec()
		apiSpec["override"] = map[string]interface{}{
			"service": map[string]interface{}{
				"internal": map[string]interface{}{},
				"wrooong":  map[string]interface{}{},
			},
		}
		spec["manilaAPI"] = apiSpec

		raw := map[string]interface{}{
			"apiVersion": "manila.openstack.org/v1beta1",
			"kind":       "Manila",
			"metadata": map[string]interface{}{
				"name":      manilaTest.Instance.Name,
				"namespace": manilaTest.Instance.Namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(
			ContainSubstring(
				"invalid: spec.manilaAPI.override.service[wrooong]: " +
					"Invalid value: \"wrooong\": invalid endpoint type: wrooong"),
		)
	})

	DescribeTable("rejects wrong topology for",
		func(serviceNameFunc func() (string, string)) {

			component, errorPath := serviceNameFunc()
			expectedErrorMessage := fmt.Sprintf("spec.%s.namespace: Invalid value: \"namespace\": Customizing namespace field is not supported", errorPath)

			spec := GetDefaultManilaSpec()
			// API and Scheduler
			if component != "top-level" && component != "share0" {
				spec[component] = map[string]interface{}{
					"topologyRef": map[string]interface{}{
						"name":      "bar",
						"namespace": "foo",
					},
				}
			}
			// manilaShares share0
			if component == "share0" {
				shareList := map[string]interface{}{
					"share0": map[string]interface{}{
						"topologyRef": map[string]interface{}{
							"name":      "foo",
							"namespace": "bar",
						},
					},
				}
				spec["manilaShares"] = shareList
				// top-level topologyRef
			} else {
				spec["topologyRef"] = map[string]interface{}{
					"name":      "bar",
					"namespace": "foo",
				}
			}
			// Build the manila CR
			raw := map[string]interface{}{
				"apiVersion": "manila.openstack.org/v1beta1",
				"kind":       "Manila",
				"metadata": map[string]interface{}{
					"name":      manilaTest.Instance.Name,
					"namespace": manilaTest.Instance.Namespace,
				},
				"spec": spec,
			}
			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(
				ContainSubstring(expectedErrorMessage))
		},
		Entry("top-level topologyRef", func() (string, string) {
			return "top-level", "topologyRef"
		}),
		Entry("manilaAPI topologyRef", func() (string, string) {
			component := "manilaAPI"
			return component, fmt.Sprintf("%s.topologyRef", component)
		}),
		Entry("manilaScheduler topologyRef", func() (string, string) {
			component := "manilaScheduler"
			return component, fmt.Sprintf("%s.topologyRef", component)
		}),
		Entry("manilaShare share0 topologyRef", func() (string, string) {
			instance := "share0"
			return instance, fmt.Sprintf("manilaShares[%s].topologyRef", instance)
		}),
	)
})
