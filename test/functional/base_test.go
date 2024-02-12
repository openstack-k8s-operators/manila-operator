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

	. "github.com/onsi/gomega"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
)

func CreateManilaSecret(namespace string, name string) *corev1.Secret {
	return th.CreateSecret(
		types.NamespacedName{Namespace: namespace, Name: name},
		map[string][]byte{
			"ManilaPassword":         []byte(manilaTest.ManilaPassword),
			"ManilaDatabasePassword": []byte(manilaTest.ManilaPassword),
			"MetadataSecret":         []byte(manilaTest.ManilaPassword),
		},
	)
}

func CreateManilaMessageBusSecret(namespace string, name string) *corev1.Secret {
	s := th.CreateSecret(
		types.NamespacedName{Namespace: namespace, Name: name},
		map[string][]byte{
			"transport_url": []byte(fmt.Sprintf("rabbit://%s/fake", name)),
		},
	)
	logger.Info("Secret created", "name", name)
	return s
}

func CreateUnstructured(rawObj map[string]interface{}) *unstructured.Unstructured {
	logger.Info("Creating", "raw", rawObj)
	unstructuredObj := &unstructured.Unstructured{Object: rawObj}
	_, err := controllerutil.CreateOrPatch(
		ctx, k8sClient, unstructuredObj, func() error { return nil })
	Expect(err).ShouldNot(HaveOccurred())
	return unstructuredObj
}

func GetManilaEmptySpec() map[string]interface{} {
	return map[string]interface{}{
		"databaseInstance": "openstack",
		"secret":           SecretName,
	}
}

func GetDefaultManilaSpec() map[string]interface{} {
	return map[string]interface{}{
		"databaseInstance": "openstack",
		"secret":           SecretName,
		"manilaAPI":        GetDefaultManilaAPISpec(),
		"manilaScheduler":  GetDefaultManilaSchedulerSpec(),
		"manilaShare":      GetDefaultManilaShareSpec(),
	}
}

func GetManilaSpec(customSpec map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"databaseInstance": "openstack",
		"secret":           SecretName,
		"manilaAPI":        GetManilaCommonSpec(customSpec),
		"manilaScheduler":  GetManilaCommonSpec(customSpec),
		"manilaShares": map[string]interface{}{
			"share0": GetManilaCommonSpec(customSpec),
			"share1": GetManilaCommonSpec(customSpec),
		},
	}
}

func GetManilaCommonSpec(spec map[string]interface{}) map[string]interface{} {
	defaultSpec := map[string]interface{}{
		"secret":             SecretName,
		"replicas":           1,
		"containerImage":     manilaTest.ContainerImage,
		"serviceAccount":     manilaTest.ManilaSA.Name,
		"transportURLSecret": manilaTest.RabbitmqSecretName,
	}
	// append to the defaultSpec map the additional keys passed as input
	for k, v := range spec {
		defaultSpec[k] = v
	}
	return defaultSpec
}

func GetTLSManilaSpec() map[string]interface{} {
	return map[string]interface{}{
		"databaseInstance": "openstack",
		"secret":           SecretName,
		"manilaAPI":        GetTLSManilaAPISpec(),
		"manilaScheduler":  GetDefaultManilaSchedulerSpec(),
	}
}

func GetDefaultManilaAPISpec() map[string]interface{} {
	return map[string]interface{}{
		"secret":             SecretName,
		"replicas":           1,
		"containerImage":     manilaTest.ContainerImage,
		"serviceAccount":     manilaTest.ManilaSA.Name,
		"transportURLSecret": manilaTest.RabbitmqSecretName,
	}
}

func GetDefaultManilaSchedulerSpec() map[string]interface{} {
	return map[string]interface{}{
		"secret":         SecretName,
		"replicas":       1,
		"containerImage": manilaTest.ContainerImage,
		"serviceAccount": manilaTest.ManilaSA.Name,
	}
}

func GetDefaultManilaShareSpec() map[string]interface{} {
	return map[string]interface{}{
		"secret":         SecretName,
		"replicas":       1,
		"containerImage": manilaTest.ContainerImage,
		"serviceAccount": manilaTest.ManilaSA.Name,
	}
}

func GetManila(name types.NamespacedName) *manilav1.Manila {
	instance := &manilav1.Manila{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func CreateManila(name types.NamespacedName, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "manila.openstack.org/v1beta1",
		"kind":       "Manila",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return CreateUnstructured(raw)
}

func ManilaConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetManila(name)
	return instance.Status.Conditions
}

func CreateManilaAPI(name types.NamespacedName, spec map[string]interface{}) client.Object {
	// we get the parent CR and set ownership to the manilaAPI CR
	parent := GetManila(manilaTest.Instance)
	raw := map[string]interface{}{
		"apiVersion": "manila.openstack.org/v1beta1",
		"kind":       "ManilaAPI",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion":         "manila.openstack.org/v1beta1",
					"blockOwnerDeletion": true,
					"controller":         true,
					"kind":               "Manila",
					"name":               parent.GetObjectMeta().GetName(),
					"uid":                parent.GetObjectMeta().GetUID(),
				},
			},
		},
		"spec": spec,
	}

	return CreateUnstructured(raw)
}

func CreateManilaScheduler(name types.NamespacedName, spec map[string]interface{}) client.Object {
	raw := map[string]interface{}{
		"apiVersion": "manila.openstack.org/v1beta1",
		"kind":       "ManilaScheduler",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return CreateUnstructured(raw)
}

func CreateManilaShare(name types.NamespacedName, spec map[string]interface{}) client.Object {
	raw := map[string]interface{}{
		"apiVersion": "manila.openstack.org/v1beta1",
		"kind":       "ManilaShare",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return CreateUnstructured(raw)
}

func GetManilaAPI(name types.NamespacedName) *manilav1.ManilaAPI {
	instance := &manilav1.ManilaAPI{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func GetManilaScheduler(name types.NamespacedName) *manilav1.ManilaScheduler {
	instance := &manilav1.ManilaScheduler{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func GetManilaShare(name types.NamespacedName) *manilav1.ManilaShare {
	instance := &manilav1.ManilaShare{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func GetTLSManilaAPISpec() map[string]interface{} {
	spec := GetDefaultManilaAPISpec()
	maps.Copy(spec, map[string]interface{}{
		"tls": map[string]interface{}{
			"api": map[string]interface{}{
				"internal": map[string]interface{}{
					"secretName": InternalCertSecretName,
				},
				"public": map[string]interface{}{
					"secretName": PublicCertSecretName,
				},
			},
			"caBundleSecretName": CABundleSecretName,
		},
	})

	return spec
}

func ManilaAPIConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetManilaAPI(name)
	return instance.Status.Conditions
}

func ManilaSchedulerConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetManilaScheduler(name)
	return instance.Status.Conditions
}

func ManilaShareConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetManilaShare(name)
	return instance.Status.Conditions
}

func ManilaAPINotExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &manilav1.ManilaAPI{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

func ManilaAPIExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &manilav1.ManilaAPI{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeFalse())
	}, timeout, interval).Should(Succeed())
}

func ManilaSchedulerExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &manilav1.ManilaScheduler{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeFalse())
	}, timeout, interval).Should(Succeed())
}

func ManilaShareExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &manilav1.ManilaShare{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeFalse())
	}, timeout, interval).Should(Succeed())
}

func ManilaSchedulerNotExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &manilav1.ManilaScheduler{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

func ManilaShareNotExists(name types.NamespacedName) {
	Consistently(func(g Gomega) {
		instance := &manilav1.ManilaShare{}
		err := k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}
