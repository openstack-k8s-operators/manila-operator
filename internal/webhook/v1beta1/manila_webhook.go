/*
Copyright 2025.

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

// Package v1beta1 implements webhook handlers for Manila v1beta1 API resources.
package v1beta1

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	manilav1beta1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
)

var (
	// ErrInvalidObjectType is returned when an unexpected object type is provided
	ErrInvalidObjectType = errors.New("invalid object type")
)

// nolint:unused
// log is for logging in this package.
var manilalog = logf.Log.WithName("manila-resource")

// SetupManilaWebhookWithManager registers the webhook for Manila in the manager.
func SetupManilaWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&manilav1beta1.Manila{}).
		WithValidator(&ManilaCustomValidator{}).
		WithDefaulter(&ManilaCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-manila-openstack-org-v1beta1-manila,mutating=true,failurePolicy=fail,sideEffects=None,groups=manila.openstack.org,resources=manilas,verbs=create;update,versions=v1beta1,name=mmanila-v1beta1.kb.io,admissionReviewVersions=v1

// ManilaCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Manila when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type ManilaCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &ManilaCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Manila.
func (d *ManilaCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	manila, ok := obj.(*manilav1beta1.Manila)

	if !ok {
		return fmt.Errorf("expected an Manila object but got %T: %w", obj, ErrInvalidObjectType)
	}
	manilalog.Info("Defaulting for Manila", "name", manila.GetName())

	// Call the Default method on the Manila type
	manila.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-manila-openstack-org-v1beta1-manila,mutating=false,failurePolicy=fail,sideEffects=None,groups=manila.openstack.org,resources=manilas,verbs=create;update,versions=v1beta1,name=vmanila-v1beta1.kb.io,admissionReviewVersions=v1

// ManilaCustomValidator struct is responsible for validating the Manila resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ManilaCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &ManilaCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Manila.
func (v *ManilaCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	manila, ok := obj.(*manilav1beta1.Manila)
	if !ok {
		return nil, fmt.Errorf("expected a Manila object but got %T: %w", obj, ErrInvalidObjectType)
	}
	manilalog.Info("Validation for Manila upon creation", "name", manila.GetName())

	return manila.ValidateCreate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Manila.
func (v *ManilaCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	manila, ok := newObj.(*manilav1beta1.Manila)
	if !ok {
		return nil, fmt.Errorf("expected a Manila object for the newObj but got %T: %w", newObj, ErrInvalidObjectType)
	}
	manilalog.Info("Validation for Manila upon update", "name", manila.GetName())

	return manila.ValidateUpdate(oldObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Manila.
func (v *ManilaCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	manila, ok := obj.(*manilav1beta1.Manila)
	if !ok {
		return nil, fmt.Errorf("expected a Manila object but got %T: %w", obj, ErrInvalidObjectType)
	}
	manilalog.Info("Validation for Manila upon deletion", "name", manila.GetName())

	return manila.ValidateDelete()
}
