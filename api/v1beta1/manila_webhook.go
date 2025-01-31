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

//
// Generated by:
//
// operator-sdk create webhook --group manila --version v1beta1 --kind Manila --programmatic-validation --defaulting
//

package v1beta1

import (
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ManilaDefaults -
type ManilaDefaults struct {
	APIContainerImageURL       string
	SchedulerContainerImageURL string
	ShareContainerImageURL     string
	DBPurgeAge                 int
	DBPurgeSchedule            string
	APITimeout                 int
}

var manilaDefaults ManilaDefaults

// log is for logging in this package.
var manilalog = logf.Log.WithName("manila-resource")

// SetupDefaults - initializes any CRD field defaults based on environment variables
// (the defaulting mechanism itself is implemented via webhooks)
func SetupDefaults() {
	// Acquire environmental defaults and initialize Manila defaults with them
	manilaDefaults = ManilaDefaults{
		APIContainerImageURL:       util.GetEnvVar("RELATED_IMAGE_MANILA_API_IMAGE_URL_DEFAULT", ManilaAPIContainerImage),
		SchedulerContainerImageURL: util.GetEnvVar("RELATED_IMAGE_MANILA_SCHEDULER_IMAGE_URL_DEFAULT", ManilaSchedulerContainerImage),
		ShareContainerImageURL:     util.GetEnvVar("RELATED_IMAGE_MANILA_SHARE_IMAGE_URL_DEFAULT", ManilaShareContainerImage),
		DBPurgeAge:                 DBPurgeDefaultAge,
		DBPurgeSchedule:            DBPurgeDefaultSchedule,
		APITimeout:                 APIDefaultTimeout,
	}

	manilalog.Info("Manila defaults initialized", "defaults", manilaDefaults)
}

// SetupWebhookWithManager sets up the webhook with the Manager
func (r *Manila) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-manila-openstack-org-v1beta1-manila,mutating=true,failurePolicy=fail,sideEffects=None,groups=manila.openstack.org,resources=manilas,verbs=create;update,versions=v1beta1,name=mmanila.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Manila{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Manila) Default() {
	manilalog.Info("default", "name", r.Name)

	r.Spec.Default()
}

// Default - set defaults for this Manila spec
func (spec *ManilaSpec) Default() {
	// only put container image validations here
	if spec.ManilaAPI.ContainerImage == "" {
		spec.ManilaAPI.ContainerImage = manilaDefaults.APIContainerImageURL
	}

	if spec.ManilaScheduler.ContainerImage == "" {
		spec.ManilaScheduler.ContainerImage = manilaDefaults.SchedulerContainerImageURL
	}

	for key, manilaShare := range spec.ManilaShares {
		if manilaShare.ContainerImage == "" {
			manilaShare.ContainerImage = manilaDefaults.ShareContainerImageURL
			// This is required, as the loop variable is a by-value copy
			spec.ManilaShares[key] = manilaShare
		}
	}
	spec.ManilaSpecBase.Default()

}

// Default - set defaults for this Manila spec base
func (spec *ManilaSpecBase) Default() {

	if spec.APITimeout == 0 {
		spec.APITimeout = manilaDefaults.APITimeout
	}

	if spec.DBPurge.Age == 0 {
		spec.DBPurge.Age = manilaDefaults.DBPurgeAge
	}

	if spec.DBPurge.Schedule == "" {
		spec.DBPurge.Schedule = manilaDefaults.DBPurgeSchedule
	}
}

// Default - set defaults for this Manila spec core. This version is used by OpenStackControlplane
func (spec *ManilaSpecCore) Default() {

	spec.ManilaSpecBase.Default()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-manila-openstack-org-v1beta1-manila,mutating=false,failurePolicy=fail,sideEffects=None,groups=manila.openstack.org,resources=manilas,verbs=create;update,versions=v1beta1,name=vmanila.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Manila{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Manila) ValidateCreate() (admission.Warnings, error) {
	manilalog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList
	basePath := field.NewPath("spec")

	allErrs = r.Spec.ValidateManilaTopology(basePath, r.Namespace)

	if err := r.Spec.ValidateCreate(basePath); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "manila.openstack.org", Kind: "Manila"},
			r.Name, allErrs)
	}

	return nil, nil
}

// ValidateCreate - Exported function wrapping non-exported validate functions,
// this function can be called externally to validate an manila spec.
func (spec *ManilaSpec) ValidateCreate(basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(
		basePath.Child("manilaAPI").Child("override").Child("service"),
		spec.ManilaAPI.Override.Service)...)

	return allErrs
}

// ValidateCreate -
func (spec *ManilaSpecCore) ValidateCreate(basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(
		basePath.Child("manilaAPI").Child("override").Child("service"),
		spec.ManilaAPI.Override.Service)...)

	return allErrs
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Manila) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	manilalog.Info("validate update", "name", r.Name)

	oldManila, ok := old.(*Manila)
	if !ok || oldManila == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("unable to convert existing object"))
	}

	var allErrs field.ErrorList
	basePath := field.NewPath("spec")

	allErrs = r.Spec.ValidateManilaTopology(basePath, r.Namespace)

	if err := r.Spec.ValidateUpdate(oldManila.Spec, basePath); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "manila.openstack.org", Kind: "Manila"},
			r.Name, allErrs)
	}

	return nil, nil
}

// ValidateUpdate - Exported function wrapping non-exported validate functions,
// this function can be called externally to validate an manila spec.
func (spec *ManilaSpec) ValidateUpdate(old ManilaSpec, basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(
		basePath.Child("manilaAPI").Child("override").Child("service"),
		spec.ManilaAPI.Override.Service)...)

	return allErrs
}

// ValidateUpdate -
func (spec *ManilaSpecCore) ValidateUpdate(old ManilaSpecCore, basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(
		basePath.Child("manilaAPI").Child("override").Child("service"),
		spec.ManilaAPI.Override.Service)...)

	return allErrs
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Manila) ValidateDelete() (admission.Warnings, error) {
	manilalog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

// SetDefaultRouteAnnotations sets HAProxy timeout values of the route
func (spec *ManilaSpecCore) SetDefaultRouteAnnotations(annotations map[string]string) {
	const haProxyAnno = "haproxy.router.openshift.io/timeout"
	// Use a custom annotation to flag when the operator has set the default HAProxy timeout
	// With the annotation func determines when to overwrite existing HAProxy timeout with the APITimeout
	const manilaAnno = "api.manila.openstack.org/timeout"

	valManila, okManila := annotations[manilaAnno]
	valHAProxy, okHAProxy := annotations[haProxyAnno]

	// Human operator set the HAProxy timeout manually
	if !okManila && okHAProxy {
		return
	}

	// Human operator modified the HAProxy timeout manually without removing the Manila flag
	if okManila && okHAProxy && valManila != valHAProxy {
		delete(annotations, manilaAnno)
		return
	}

	timeout := fmt.Sprintf("%ds", spec.APITimeout)
	annotations[manilaAnno] = timeout
	annotations[haProxyAnno] = timeout
}

// ValidateManilaTopology - Returns an ErrorList if the Topology is referenced
// on a different namespace
func (spec *ManilaSpec) ValidateManilaTopology(basePath *field.Path, namespace string) field.ErrorList {
	var allErrs field.ErrorList

	// When a TopologyRef CR is referenced, fail if a different Namespace is
	// referenced because is not supported
	if spec.TopologyRef != nil {
		if err := topologyv1.ValidateTopologyNamespace(spec.TopologyRef.Namespace, *basePath, namespace); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	// When a TopologyRef CR is referenced with an override to ManilaAPI, fail
	// if a different Namespace is referenced because not supported
	if spec.ManilaAPI.TopologyRef != nil {
		if err := topologyv1.ValidateTopologyNamespace(spec.ManilaAPI.TopologyRef.Namespace, *basePath, namespace); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	// When a TopologyRef CR is referenced with an override to ManilaScheduler,
	// fail if a different Namespace is referenced because not supported
	if spec.ManilaScheduler.TopologyRef != nil {
		if err := topologyv1.ValidateTopologyNamespace(spec.ManilaScheduler.TopologyRef.Namespace, *basePath, namespace); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	// When a TopologyRef CR is referenced with an override to an instance of
	// ManilaShares, fail if a different Namespace is referenced because not
	// supported
	for _, ms := range spec.ManilaShares {
		if ms.TopologyRef != nil {
			if err := topologyv1.ValidateTopologyNamespace(ms.TopologyRef.Namespace, *basePath, namespace); err != nil {
				allErrs = append(allErrs, err)
			}
		}
	}
	return allErrs
}
