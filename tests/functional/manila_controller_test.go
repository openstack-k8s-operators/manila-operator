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

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	util "github.com/openstack-k8s-operators/lib-common/modules/common/util"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
)

var _ = Describe("Manila controller", func() {
	When("Manila CR instance is created", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetDefaultManilaSpec()))
		})

		It("initializes the status fields", func() {
			Eventually(func(g Gomega) {
				glance := GetManila(manilaName)
				g.Expect(glance.Status.Conditions).To(HaveLen(12))

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
		It("should have a finalizer", func() {
			// the reconciler loop adds the finalizer so we have to wait for
			// it to run
			Eventually(func() []string {
				return GetManila(manilaTest.Instance).Finalizers
			}, timeout, interval).Should(ContainElement("Manila"))
		})
		It("should not create a config map", func() {
			Eventually(func() []corev1.ConfigMap {
				return th.ListConfigMaps(manilaTest.ManilaConfigMapData.Name).Items
			}, timeout, interval).Should(BeEmpty())
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
		It("should have Unknown Conditions initialized as transporturl not created", func() {
			th.ExpectCondition(
				manilaName,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionUnknown,
			)
		})
		// should create 01-deployment.conf secret
	})
	When("Manila DB is created", func() {
		BeforeEach(func() {
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, GetDefaultManilaSpec()))
			DeferCleanup(
				th.DeleteDBService,
				th.CreateDBService(
					namespace,
					GetManila(manilaTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			th.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			DeferCleanup(th.DeleteKeystoneAPI, th.CreateKeystoneAPI(namespace))
		})
		It("Should set DBReady Condition and set DatabaseHostname Status when DB is Created", func() {
			th.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			Manila := GetManila(manilaTest.Instance)
			Expect(Manila.Status.DatabaseHostname).To(Equal("hostname-for-openstack"))
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
			th.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
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
			DeferCleanup(th.DeleteDBService, th.CreateDBService(
				manilaTest.Instance.Namespace,
				GetManila(manilaName).Spec.DatabaseInstance,
				corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 3306}},
				},
			),
			)
			th.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
		})
		It("should create config-data and scripts ConfigMaps", func() {
			keystoneAPI := th.CreateKeystoneAPI(manilaTest.Instance.Namespace)
			DeferCleanup(th.DeleteKeystoneAPI, keystoneAPI)
			Eventually(func() corev1.ConfigMap {
				return *th.GetConfigMap(manilaTest.ManilaConfigMapData)
			}, timeout, interval).ShouldNot(BeNil())
			Eventually(func() corev1.ConfigMap {
				return *th.GetConfigMap(manilaTest.ManilaConfigMapScripts)
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
				th.DeleteDBService,
				th.CreateDBService(
					manilaTest.Instance.Namespace,
					GetManila(manilaName).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			th.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			DeferCleanup(th.DeleteKeystoneAPI, th.CreateKeystoneAPI(manilaTest.Instance.Namespace))
			th.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			th.SimulateKeystoneServiceReady(manilaTest.Instance)
			th.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
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
				th.DeleteDBService,
				th.CreateDBService(
					manilaTest.Instance.Namespace,
					GetManila(manilaTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			th.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			DeferCleanup(th.DeleteKeystoneAPI, th.CreateKeystoneAPI(manilaTest.Instance.Namespace))
			th.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
		})
		It("removes the finalizers from the Manila DB", func() {
			th.SimulateKeystoneServiceReady(manilaTest.Instance)

			mDB := th.GetMariaDBDatabase(manilaTest.Instance)
			Expect(mDB.Finalizers).To(ContainElement("Manila"))

			th.DeleteInstance(GetManila(manilaTest.Instance))

			mDB = th.GetMariaDBDatabase(manilaTest.Instance)
			Expect(mDB.Finalizers).NotTo(ContainElement("Manila"))
		})
		It("removes the ConfigMaps", func() {
			keystoneAPI := th.CreateKeystoneAPI(manilaTest.Instance.Namespace)
			DeferCleanup(th.DeleteKeystoneAPI, keystoneAPI)

			Eventually(func() corev1.ConfigMap {
				return *th.GetConfigMap(manilaTest.ManilaConfigMapData)
			}, timeout, interval).ShouldNot(BeNil())
			Eventually(func() corev1.ConfigMap {
				return *th.GetConfigMap(manilaTest.ManilaConfigMapScripts)
			}, timeout, interval).ShouldNot(BeNil())
			Eventually(func() []corev1.ConfigMap {
				return th.ListConfigMaps(manilaTest.ManilaConfigMapData.Name).Items
			}, timeout, interval).Should(BeEmpty())
			Eventually(func() []corev1.ConfigMap {
				return th.ListConfigMaps(manilaTest.ManilaConfigMapScripts.Name).Items
			}, timeout, interval).Should(BeEmpty())
		})
	})
	When("Manila CR instance is built w/ NAD", func() {
		BeforeEach(func() {
			nad := th.CreateNetworkAttachmentDefinition(manilaTest.InternalAPINAD)
			DeferCleanup(th.DeleteInstance, nad)
			rawSpec := map[string]interface{}{
				"secret":              SecretName,
				"databaseInstance":    "openstack",
				"rabbitMqClusterName": "rabbitmq",
				"manilaAPI": map[string]interface{}{
					"containerImage":     manilav1.ManilaAPIContainerImage,
					"networkAttachments": []string{"internalapi"},
				},
				"manilaScheduler": map[string]interface{}{
					"containerImage":     manilav1.ManilaSchedulerContainerImage,
					"networkAttachments": []string{"internalapi"},
				},
				"manilaShares": map[string]interface{}{
					"share1": map[string]interface{}{
						"containerImage":     manilav1.ManilaShareContainerImage,
						"networkAttachments": []string{"internalapi"},
					},
				},
			}
			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, rawSpec))
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
			DeferCleanup(
				th.DeleteDBService,
				th.CreateDBService(
					manilaTest.Instance.Namespace,
					GetManila(manilaTest.Instance).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			th.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			keystoneAPIName := th.CreateKeystoneAPI(manilaTest.Instance.Namespace)
			DeferCleanup(th.DeleteKeystoneAPI, keystoneAPIName)
			keystoneAPI := th.GetKeystoneAPI(keystoneAPIName)
			keystoneAPI.Status.APIEndpoints["internal"] = "http://keystone-internal-openstack.testing"
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Status().Update(ctx, keystoneAPI.DeepCopy())).Should(Succeed())
			}, timeout, interval).Should(Succeed())
			th.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			th.SimulateKeystoneServiceReady(manilaTest.Instance)
		})
		It("Check the resulting endpoints of the generated sub-CRs", func() {
			th.SimulateDeploymentReadyWithPods(
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
			th.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
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
		})
	})

	When("A Manila is created with service override", func() {

		BeforeEach(func() {
			spec := GetDefaultManilaSpec()
			var serviceOverride []interface{}
			serviceOverride = append(
				serviceOverride, map[string]interface{}{
					"endpoint": "internal",
					"metadata": map[string]map[string]string{
						"annotations": {
							"dnsmasq.network.openstack.org/hostname": "manila-internal.openstack.svc",
							"metallb.universe.tf/address-pool":       "osp-internalapi",
							"metallb.universe.tf/allow-shared-ip":    "osp-internalapi",
							"metallb.universe.tf/loadBalancerIPs":    "internal-lb-ip-1,internal-lb-ip-2",
						},
						"labels": {
							"internal": "true",
							"service":  "manila",
						},
					},
					"spec": map[string]interface{}{
						"type": "LoadBalancer",
					},
				},
			)

			spec["override"] = map[string]interface{}{
				"manilaAPI": map[string]interface{}{
					"override": map[string]interface{}{
						"service": serviceOverride,
					},
				},
			}

			DeferCleanup(th.DeleteInstance, CreateManila(manilaTest.Instance, spec))
			DeferCleanup(k8sClient.Delete, ctx, CreateManilaMessageBusSecret(manilaTest.Instance.Namespace, manilaTest.RabbitmqSecretName))
			DeferCleanup(th.DeleteInstance, CreateManilaAPI(manilaTest.Instance, GetDefaultManilaAPISpec()))
			DeferCleanup(th.DeleteInstance, CreateManilaScheduler(manilaTest.Instance, GetDefaultManilaSchedulerSpec()))
			DeferCleanup(th.DeleteInstance, CreateManilaShare(manilaTest.Instance, GetDefaultManilaShareSpec()))
			DeferCleanup(
				th.DeleteDBService,
				th.CreateDBService(
					manilaTest.Instance.Namespace,
					GetManila(manilaName).Spec.DatabaseInstance,
					corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
				),
			)
			th.SimulateTransportURLReady(manilaTest.ManilaTransportURL)
			DeferCleanup(th.DeleteKeystoneAPI, th.CreateKeystoneAPI(manilaTest.Instance.Namespace))
			th.SimulateMariaDBDatabaseCompleted(manilaTest.Instance)
			th.SimulateJobSuccess(manilaTest.ManilaDBSync)
			th.SimulateKeystoneServiceReady(manilaTest.Instance)
			th.SimulateKeystoneEndpointReady(manilaTest.ManilaKeystoneEndpoint)
			th.SimulateDeploymentReadyWithPods(
				manilaTest.ManilaAPI,
				map[string][]string{},
			)
			th.SimulateStatefulSetReplicaReadyWithPods(
				manilaTest.ManilaScheduler,
				map[string][]string{},
			)
			th.SimulateStatefulSetReplicaReadyWithPods(
				manilaTest.ManilaShares[0],
				map[string][]string{},
			)
		})

		It("creates KeystoneEndpoint", func() {
			keystoneEndpoint := th.GetKeystoneEndpoint(manilaTest.ManilaKeystoneEndpoint)
			endpoints := keystoneEndpoint.Spec.Endpoints
			Expect(endpoints).To(HaveKeyWithValue("public", "http://manila-public."+manilaTest.Instance.Namespace+".svc:8786/v2"))
			Expect(endpoints).To(HaveKeyWithValue("internal", "http://manila-internal."+manilaTest.Instance.Namespace+".svc:8786/v2"))

			th.ExpectCondition(
				manilaTest.ManilaAPI,
				ConditionGetterFunc(ManilaAPIConditionGetter),
				condition.KeystoneEndpointReadyCondition,
				corev1.ConditionTrue,
			)
		})

		It("creates LoadBalancer service", func() {
			// As the internal endpoint is configured via overrides it
			// gets a LoadBalancer Service with MetalLB annotations
			service := th.GetService(types.NamespacedName{Namespace: namespace, Name: "manila-internal"})
			th.Logger.Info(fmt.Sprintf("BOO %+v", service))
			Expect(service.Annotations).To(
				HaveKeyWithValue("dnsmasq.network.openstack.org/hostname", "manila-internal.openstack.svc"))
			Expect(service.Annotations).To(
				HaveKeyWithValue("metallb.universe.tf/address-pool", "osp-internalapi"))
			Expect(service.Annotations).To(
				HaveKeyWithValue("metallb.universe.tf/allow-shared-ip", "osp-internalapi"))
			Expect(service.Annotations).To(
				HaveKeyWithValue("metallb.universe.tf/loadBalancerIPs", "internal-lb-ip-1,internal-lb-ip-2"))

			th.ExpectCondition(
				manilaTest.Instance,
				ConditionGetterFunc(ManilaConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})
})
