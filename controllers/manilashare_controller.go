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

package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	nad "github.com/openstack-k8s-operators/lib-common/modules/common/networkattachment"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/statefulset"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	manilav1beta1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manilashare"
)

// GetClient -
func (r *ManilaShareReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *ManilaShareReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *ManilaShareReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *ManilaShareReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// ManilaShareReconciler reconciles a ManilaShare object
type ManilaShareReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
	Log     logr.Logger
}

//+kubebuilder:rbac:groups=manila.openstack.org,resources=manilashares,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=manila.openstack.org,resources=manilashares/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=manila.openstack.org,resources=manilashares/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;create;update;patch;delete;watch
//+kubebuilder:rbac:groups=security.openshift.io,namespace=openstack,resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch

// Reconcile -
func (r *ManilaShareReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	_ = log.FromContext(ctx)

	// Fetch the ManilaShare instance
	instance := &manilav1beta1.ManilaShare{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// update the Ready condition based on the sub conditions
		if instance.Status.Conditions.AllSubConditionIsTrue() {
			instance.Status.Conditions.MarkTrue(
				condition.ReadyCondition, condition.ReadyMessage)
		} else {
			// something is not ready so reset the Ready condition
			instance.Status.Conditions.MarkUnknown(
				condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage)
			// and recalculate it based on the state of the rest of the conditions
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}
		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) {
		return ctrl.Result{}, nil
	}

	//
	// initialize status
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		// initialize conditions used later as Status=Unknown
		cl := condition.CreateList(
			condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
			condition.UnknownCondition(condition.ServiceConfigReadyCondition, condition.InitReason, condition.ServiceConfigReadyInitMessage),
			condition.UnknownCondition(condition.DeploymentReadyCondition, condition.InitReason, condition.DeploymentReadyInitMessage),
			condition.UnknownCondition(condition.NetworkAttachmentsReadyCondition, condition.InitReason, condition.NetworkAttachmentsReadyInitMessage),
			condition.UnknownCondition(condition.TLSInputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
		)

		instance.Status.Conditions.Init(&cl)

		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, err
	}
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManilaShareReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Watch for changes to any CustomServiceConfigSecrets. Global secrets
	// (e.g. TransportURLSecret) are handled by the top Manila controller.
	secretFn := func(o client.Object) []reconcile.Request {
		var namespace string = o.GetNamespace()
		var secretName string = o.GetName()
		result := []reconcile.Request{}

		// get all API CRs
		shares := &manilav1beta1.ManilaShareList{}
		listOpts := []client.ListOption{
			client.InNamespace(namespace),
		}
		if err := r.Client.List(context.Background(), shares, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve API CRs %v")
			return nil
		}

		label := o.GetLabels()
		if l, ok := label[labels.GetOwnerNameLabelSelector(labels.GetGroupLabel(manila.ServiceName))]; ok {
			for _, cr := range shares.Items {
				// return reconcil event for the CR where the CM owner label AND the parentManilaName matches
				if l == manila.GetOwningManilaName(&cr) {
					// return namespace and Name of CR
					name := client.ObjectKey{
						Namespace: namespace,
						Name:      cr.Name,
					}
					r.Log.Info(fmt.Sprintf("Secret object %s and CR %s marked with label: %s", o.GetName(), cr.Name, l))
					result = append(result, reconcile.Request{NamespacedName: name})
				}
			}
		}
		for _, cr := range shares.Items {
			for _, v := range cr.Spec.CustomServiceConfigSecrets {
				if v == secretName {
					name := client.ObjectKey{
						Namespace: namespace,
						Name:      cr.Name,
					}
					r.Log.Info(fmt.Sprintf("Secret %s is used by Manila CR %s", secretName, cr.Name))
					result = append(result, reconcile.Request{NamespacedName: name})
				}
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	// index passwordSecretField
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &manilav1beta1.ManilaShare{}, passwordSecretField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*manilav1beta1.ManilaShare)
		if cr.Spec.Secret == "" {
			return nil
		}
		return []string{cr.Spec.Secret}
	}); err != nil {
		return err
	}

	// index caBundleSecretNameField
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &manilav1beta1.ManilaShare{}, caBundleSecretNameField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*manilav1beta1.ManilaShare)
		if cr.Spec.TLS.CaBundleSecretName == "" {
			return nil
		}
		return []string{cr.Spec.TLS.CaBundleSecretName}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&manilav1beta1.ManilaShare{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Secret{}).
		// watch the secrets we don't own
		Watches(&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(secretFn)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSrc),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *ManilaShareReconciler) findObjectsForSrc(src client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	l := log.FromContext(context.Background()).WithName("Controllers").WithName("ManilaShare")

	for _, field := range commonWatchFields {
		crList := &manilav1beta1.ManilaShareList{}
		listOps := &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(field, src.GetName()),
			Namespace:     src.GetNamespace(),
		}
		err := r.List(context.TODO(), crList, listOps)
		if err != nil {
			return []reconcile.Request{}
		}

		for _, item := range crList.Items {
			l.Info(fmt.Sprintf("input source %s changed, reconcile: %s - %s", src.GetName(), item.GetName(), item.GetNamespace()))

			requests = append(requests,
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      item.GetName(),
						Namespace: item.GetNamespace(),
					},
				},
			)
		}
	}

	return requests
}

func (r *ManilaShareReconciler) reconcileDelete(ctx context.Context, instance *manilav1beta1.ManilaShare, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}

func (r *ManilaShareReconciler) reconcileInit(
	ctx context.Context,
	instance *manilav1beta1.ManilaShare,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' init", instance.Name))

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' init successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaShareReconciler) reconcileNormal(ctx context.Context, instance *manilav1beta1.ManilaShare, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))

	// configVars
	configVars := make(map[string]env.Setter)

	//
	// check for required OpenStack secret holding passwords for service/admin user and add hash to the vars map
	//
	ctrlResult, err := r.getSecret(ctx, helper, instance, instance.Spec.Secret, &configVars)
	if err != nil {
		return ctrlResult, err
	}

	//
	// check for required TransportURL secret holding transport URL string
	//
	ctrlResult, err = r.getSecret(ctx, helper, instance, instance.Spec.TransportURLSecret, &configVars)
	if err != nil {
		return ctrlResult, err
	}

	//
	// check for required service secrets
	//
	for _, secretName := range instance.Spec.CustomServiceConfigSecrets {
		ctrlResult, err = r.getSecret(ctx, helper, instance, secretName, &configVars)
		if err != nil {
			return ctrlResult, err
		}
	}

	parentManilaName := manila.GetOwningManilaName(instance)
	parentSecrets := []string{
		fmt.Sprintf("%s-scripts", parentManilaName),     // ScriptsSecret
		fmt.Sprintf("%s-config-data", parentManilaName), // Secret used for ServiceConfig
	}

	for _, parentSecret := range parentSecrets {
		ctrlResult, err = r.getSecret(ctx, helper, instance, parentSecret, &configVars)
		if err != nil {
			return ctrlResult, err
		}
	}
	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)

	//
	// TLS input validation
	//
	// Validate the CA cert secret if provided
	if instance.Spec.TLS.CaBundleSecretName != "" {
		hash, ctrlResult, err := tls.ValidateCACertSecret(
			ctx,
			helper.GetClient(),
			types.NamespacedName{
				Name:      instance.Spec.TLS.CaBundleSecretName,
				Namespace: instance.Namespace,
			},
		)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.TLSInputReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.TLSInputErrorMessage,
				err.Error()))
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		if hash != "" {
			configVars[tls.CABundleKey] = env.SetValue(hash)
		}
	}
	// all cert input checks out so report InputReady
	instance.Status.Conditions.MarkTrue(condition.TLSInputReadyCondition, condition.InputReadyMessage)

	serviceLabels := map[string]string{
		common.AppSelector:       manila.ServiceName,
		common.ComponentSelector: manilashare.Component,
	}
	//
	// create service Secrets for manila-share service
	//
	err = r.generateServiceConfig(ctx, helper, instance, &configVars, serviceLabels)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	//
	// create hash over all the different input resources to identify if any those changed
	// and a restart/recreate is required.
	//
	inputHash, hashChanged, err := r.createHashOfInputHashes(ctx, instance, configVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	} else if hashChanged {
		// Hash changed and instance status should be updated (which will be done by main defer func),
		// so we need to return and reconcile again
		return ctrl.Result{}, nil
	}
	instance.Status.Conditions.MarkTrue(condition.ServiceConfigReadyCondition, condition.ServiceConfigReadyMessage)

	//
	// TODO check when/if Init, Update, or Upgrade should/could be skipped
	//

	// networks to attach to
	for _, netAtt := range instance.Spec.NetworkAttachments {
		_, err := nad.GetNADWithName(ctx, helper, netAtt, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				instance.Status.Conditions.Set(condition.FalseCondition(
					condition.NetworkAttachmentsReadyCondition,
					condition.RequestedReason,
					condition.SeverityInfo,
					condition.NetworkAttachmentsReadyWaitingMessage,
					netAtt))
				return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("network-attachment-definition %s not found", netAtt)
			}
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.NetworkAttachmentsReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.NetworkAttachmentsReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
	}

	serviceAnnotations, err := nad.CreateNetworksAnnotation(instance.Namespace, instance.Spec.NetworkAttachments)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed create network annotation from %s: %w",
			instance.Spec.NetworkAttachments, err)
	}

	// Handle service init
	ctrlResult, err = r.reconcileInit(ctx, instance, helper, serviceLabels)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service update
	ctrlResult, err = r.reconcileUpdate(ctx, instance, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service upgrade
	ctrlResult, err = r.reconcileUpgrade(ctx, instance, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	//
	// normal reconcile tasks
	//

	// Deploy a statefulset
	ss := statefulset.NewStatefulSet(
		manilashare.StatefulSet(instance, inputHash, serviceLabels, serviceAnnotations),
		time.Duration(5)*time.Second,
	)

	ctrlResult, err = ss.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DeploymentReadyRunningMessage))
		return ctrlResult, nil
	}
	instance.Status.ReadyCount = ss.GetStatefulSet().Status.ReadyReplicas

	// verify if network attachment matches expectations
	networkReady := false
	networkAttachmentStatus := map[string][]string{}
	if *(instance.Spec.Replicas) > 0 {
		// verify if network attachment matches expectations
		networkReady, networkAttachmentStatus, err = nad.VerifyNetworkStatusFromAnnotation(
			ctx,
			helper,
			instance.Spec.NetworkAttachments,
			serviceLabels,
			instance.Status.ReadyCount,
		)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		networkReady = true
	}

	instance.Status.NetworkAttachments = networkAttachmentStatus
	if networkReady {
		instance.Status.Conditions.MarkTrue(condition.NetworkAttachmentsReadyCondition, condition.NetworkAttachmentsReadyMessage)
	} else {
		err := fmt.Errorf("not all pods have interfaces with ips as configured in NetworkAttachments: %s", instance.Spec.NetworkAttachments)
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.NetworkAttachmentsReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.NetworkAttachmentsReadyErrorMessage,
			err.Error()))

		return ctrl.Result{}, err
	}

	if instance.Status.ReadyCount > 0 || *instance.Spec.Replicas == 0 {
		instance.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition, condition.DeploymentReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DeploymentReadyRunningMessage))
	}
	// create StatefulSet - end

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaShareReconciler) reconcileUpdate(ctx context.Context, instance *manilav1beta1.ManilaShare, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' update", instance.Name))

	// TODO: should have minor update tasks if required
	// - delete dbsync hash from status to rerun it?

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' update successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaShareReconciler) reconcileUpgrade(ctx context.Context, instance *manilav1beta1.ManilaShare, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' upgrade", instance.Name))

	// TODO: should have major version upgrade tasks
	// -delete dbsync hash from status to rerun it?

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' upgrade successfully", instance.Name))
	return ctrl.Result{}, nil
}

// getSecret - get the specified secret, and add its hash to envVars
func (r *ManilaShareReconciler) getSecret(
	ctx context.Context,
	h *helper.Helper,
	instance *manilav1beta1.ManilaShare,
	secretName string,
	envVars *map[string]env.Setter,
) (ctrl.Result, error) {
	secret, hash, err := secret.GetSecret(ctx, h, secretName, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.InputReadyWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, fmt.Errorf("Secret %s not found", secretName)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	// Add a prefix to the var name to avoid accidental collision with other non-secret
	// vars. The secret names themselves will be unique.
	(*envVars)["secret-"+secret.Name] = env.SetValue(hash)

	return ctrl.Result{}, nil
}

// generateServiceConfig - create custom Secret to hold service-specific config
func (r *ManilaShareReconciler) generateServiceConfig(
	ctx context.Context,
	h *helper.Helper,
	instance *manilav1beta1.ManilaShare,
	envVars *map[string]env.Setter,
	serviceLabels map[string]string,
) error {
	//
	// create custom Secret for manila-share-specific config input
	// - %-config-data Secret holding custom config for the service's manila.conf
	//

	labels := labels.GetLabels(instance, labels.GetGroupLabel(manila.ServiceName), serviceLabels)

	customData := map[string]string{manila.CustomServiceConfigFileName: instance.Spec.CustomServiceConfig}

	customData[manila.CustomServiceConfigFileName] = instance.Spec.CustomServiceConfig

	// Fetch the two service config snippets (DefaultsConfigFileName and
	// CustomConfigFileName) from the Secret generated by the top level
	// manila controller, and add them to this service specific Secret.
	manilaSecretName := manila.GetOwningManilaName(instance) + "-config-data"
	manilaSecret, _, err := secret.GetSecret(ctx, h, manilaSecretName, instance.Namespace)
	if err != nil {
		return err
	}
	customData[manila.DefaultsConfigFileName] = string(manilaSecret.Data[manila.DefaultsConfigFileName])
	customData[manila.CustomConfigFileName] = string(manilaSecret.Data[manila.CustomConfigFileName])

	customSecrets := ""
	for _, secretName := range instance.Spec.CustomServiceConfigSecrets {
		secret, _, err := secret.GetSecret(ctx, h, secretName, instance.Namespace)
		if err != nil {
			return err
		}
		for _, data := range secret.Data {
			customSecrets += string(data) + "\n"
		}
	}

	customData[manila.CustomServiceConfigSecretsFileName] = customSecrets

	configTemplates := []util.Template{
		// Custom ConfigMap
		{
			Name:         fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:    instance.Namespace,
			Type:         util.TemplateTypeConfig,
			InstanceType: instance.Kind,
			CustomData:   customData,
			Labels:       labels,
		},
	}

	return secret.EnsureSecrets(ctx, h, instance, configTemplates, envVars)
}

// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
//
// returns the hash, whether the hash changed (as a bool) and any error
func (r *ManilaShareReconciler) createHashOfInputHashes(
	ctx context.Context,
	instance *manilav1beta1.ManilaShare,
	envVars map[string]env.Setter,
) (string, bool, error) {
	var hashMap map[string]string
	changed := false
	mergedMapVars := env.MergeEnvs([]corev1.EnvVar{}, envVars)
	hash, err := util.ObjectHash(mergedMapVars)
	if err != nil {
		return hash, changed, err
	}
	if hashMap, changed = util.SetHash(instance.Status.Hash, common.InputHashName, hash); changed {
		instance.Status.Hash = hashMap
		r.Log.Info(fmt.Sprintf("Input maps hash %s - %s", common.InputHashName, hash))
	}
	return hash, changed, nil
}
