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

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/go-logr/logr"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	cronjob "github.com/openstack-k8s-operators/lib-common/modules/common/cronjob"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/job"
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	common_rbac "github.com/openstack-k8s-operators/lib-common/modules/common/rbac"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	manilav1beta1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// GetClient -
func (r *ManilaReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *ManilaReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *ManilaReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *ManilaReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// ManilaReconciler reconciles a Manila object
type ManilaReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilas/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaapis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaapis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaapis/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaschedulers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaschedulers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaschedulers/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilashares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilashares/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilashares/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbdatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbdatabases/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbaccounts/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds,verbs=get;list;watch;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.openstack.org,resources=transporturls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete;

// service account, role, rolebinding
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch
// service account permissions that are needed to grant permission to the above
// +kubebuilder:rbac:groups="security.openshift.io",resourceNames=anyuid;privileged,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups="",resources=pods,verbs=create;delete;get;list;patch;update;watch

// Reconcile -
func (r *ManilaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	_ = log.FromContext(ctx)

	instance := &manilav1beta1.Manila{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, fmt.Sprintf("could not fetch Manila instance %s", instance.Name))
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
		r.Log.Error(err, fmt.Sprintf("could not instantiate helper for instance %s", instance.Name))
		return ctrl.Result{}, err
	}

	// initialize status
	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	// Save a copy of the condtions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		condition.RestoreLastTransitionTimes(
			&instance.Status.Conditions, savedConditions)
		if instance.Status.Conditions.IsUnknown(condition.ReadyCondition) {
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}
		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	// Always initialize conditions used later as Status=Unknown
	cl := condition.CreateList(
		condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
		condition.UnknownCondition(condition.DBReadyCondition, condition.InitReason, condition.DBReadyInitMessage),
		condition.UnknownCondition(condition.DBSyncReadyCondition, condition.InitReason, condition.DBSyncReadyInitMessage),
		condition.UnknownCondition(condition.RabbitMqTransportURLReadyCondition, condition.InitReason, condition.RabbitMqTransportURLReadyInitMessage),
		condition.UnknownCondition(condition.MemcachedReadyCondition, condition.InitReason, condition.MemcachedReadyInitMessage),
		condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
		condition.UnknownCondition(manilav1beta1.ManilaAPIReadyCondition, condition.InitReason, manilav1beta1.ManilaAPIReadyInitMessage),
		condition.UnknownCondition(manilav1beta1.ManilaSchedulerReadyCondition, condition.InitReason, manilav1beta1.ManilaSchedulerReadyInitMessage),
		condition.UnknownCondition(manilav1beta1.ManilaShareReadyCondition, condition.InitReason, manilav1beta1.ManilaShareReadyInitMessage),
		condition.UnknownCondition(condition.NetworkAttachmentsReadyCondition, condition.InitReason, condition.NetworkAttachmentsReadyInitMessage),
		// service account, role, rolebinding conditions
		condition.UnknownCondition(condition.ServiceAccountReadyCondition, condition.InitReason, condition.ServiceAccountReadyInitMessage),
		condition.UnknownCondition(condition.RoleReadyCondition, condition.InitReason, condition.RoleReadyInitMessage),
		condition.UnknownCondition(condition.RoleBindingReadyCondition, condition.InitReason, condition.RoleBindingReadyInitMessage),
		condition.UnknownCondition(condition.CronJobReadyCondition, condition.InitReason, condition.CronJobReadyInitMessage),
	)
	instance.Status.Conditions.Init(&cl)
	instance.Status.ObservedGeneration = instance.Generation

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if (instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer())) || isNewInstance {
		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, nil
	}

	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}
	if instance.Status.ManilaSharesReadyCounts == nil {
		instance.Status.ManilaSharesReadyCounts = map[string]int32{}
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

// fields to index to reconcile when change
const (
	passwordSecretField     = ".spec.secret"
	caBundleSecretNameField = ".spec.tls.caBundleSecretName"
	tlsAPIInternalField     = ".spec.tls.api.internal.secretName"
	tlsAPIPublicField       = ".spec.tls.api.public.secretName"
)

var (
	commonWatchFields = []string{
		passwordSecretField,
		caBundleSecretNameField,
	}
	manilaAPIWatchFields = []string{
		passwordSecretField,
		caBundleSecretNameField,
		tlsAPIInternalField,
		tlsAPIPublicField,
	}
)

// SetupWithManager sets up the controller with the Manager.
func (r *ManilaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// transportURLSecretFn - Watch for changes made to the secret associated with the RabbitMQ
	// TransportURL created and used by Manila CRs. Watch functions return a list of namespace-scoped
	// CRs that then get fed to the reconciler. Hence, in this case, we need to know the name of the
	// Manila CR associated with the secret we are examining in the function. We could parse the name
	// out of the "%s-manila-transport" secret label, which would be faster than getting the list of
	// the Manila CRs and trying to match on each one. The downside there, however, is that technically
	// someone could randomly label a secret "something-manila-transport" where "something" actually
	// matches the name of an existing Manila CR. In that case changes to that secret would trigger
	// reconciliation for a Manila CR that does not need it.
	//
	// TODO: We also need a watch func to monitor for changes to the secret referenced by Manila.Spec.Secret
	transportURLSecretFn := func(_ context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all Manila CRs
		manilas := &manilav1beta1.ManilaList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), manilas, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve Manila CRs %v")
			return nil
		}

		for _, ownerRef := range o.GetOwnerReferences() {
			if ownerRef.Kind == "TransportURL" {
				for _, cr := range manilas.Items {
					if ownerRef.Name == fmt.Sprintf("%s-manila-transport", cr.Name) {
						// return namespace and Name of CR
						name := client.ObjectKey{
							Namespace: o.GetNamespace(),
							Name:      cr.Name,
						}
						r.Log.Info(fmt.Sprintf("TransportURL Secret %s belongs to TransportURL belonging to Manila CR %s", o.GetName(), cr.Name))
						result = append(result, reconcile.Request{NamespacedName: name})
					}
				}
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	memcachedFn := func(_ context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all Manila CRs
		manilas := &manilav1beta1.ManilaList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), manilas, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve Manila CRs %w")
			return nil
		}

		for _, cr := range manilas.Items {
			if o.GetName() == cr.Spec.MemcachedInstance {
				name := client.ObjectKey{
					Namespace: o.GetNamespace(),
					Name:      cr.Name,
				}
				r.Log.Info(fmt.Sprintf("Memcached %s is used by Manila CR %s", o.GetName(), cr.Name))
				result = append(result, reconcile.Request{NamespacedName: name})
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&manilav1beta1.Manila{}).
		Owns(&mariadbv1.MariaDBDatabase{}).
		Owns(&mariadbv1.MariaDBAccount{}).
		Owns(&manilav1beta1.ManilaAPI{}).
		Owns(&manilav1beta1.ManilaScheduler{}).
		Owns(&manilav1beta1.ManilaShare{}).
		Owns(&rabbitmqv1.TransportURL{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		// Watch for TransportURL Secrets which belong to any TransportURLs created by Manila CRs
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(transportURLSecretFn)).
		Watches(&memcachedv1.Memcached{},
			handler.EnqueueRequestsFromMapFunc(memcachedFn)).
		Complete(r)
}

func (r *ManilaReconciler) reconcileDelete(ctx context.Context, instance *manilav1beta1.Manila, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// remove db finalizer first
	db, err := mariadbv1.GetDatabaseByNameAndAccount(ctx, helper, manila.DatabaseCRName, instance.Spec.DatabaseAccount, instance.Namespace)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if !k8s_errors.IsNotFound(err) {
		if err := db.DeleteFinalizer(ctx, helper); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}

func (r *ManilaReconciler) reconcileInit(
	ctx context.Context,
	instance *manilav1beta1.Manila,
	helper *helper.Helper,
	serviceLabels map[string]string,
	serviceAnnotations map[string]string,
) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' init", instance.Name))

	//
	// run manila db sync
	//
	dbSyncHash := instance.Status.Hash[manilav1beta1.DbSyncHash]
	jobDef := manila.Job(
		instance,
		serviceLabels,
		serviceAnnotations,
		nil,
		manila.DBSyncJobName,
		manila.DBSyncCommand,
		0, // no need to delay dbSync
	)
	dbSyncjob := job.NewJob(
		jobDef,
		manilav1beta1.DbSyncHash,
		instance.Spec.PreserveJobs,
		manila.ShortDuration,
		dbSyncHash,
	)
	ctrlResult, err := dbSyncjob.DoJob(
		ctx,
		helper,
	)
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBSyncReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBSyncReadyRunningMessage))
		return ctrlResult, nil
	}
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBSyncReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBSyncReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if dbSyncjob.HasChanged() {
		instance.Status.Hash[manilav1beta1.DbSyncHash] = dbSyncjob.GetHash()
		r.Log.Info(fmt.Sprintf("Service '%s' - Job %s hash added - %s", instance.Name, jobDef.Name, instance.Status.Hash[manilav1beta1.DbSyncHash]))
	}
	instance.Status.Conditions.MarkTrue(condition.DBSyncReadyCondition, condition.DBSyncReadyMessage)

	// run Manila db sync - end

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' init successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaReconciler) reconcileNormal(ctx context.Context, instance *manilav1beta1.Manila, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))

	// Service account, role, binding
	rbacRules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			ResourceNames: []string{"anyuid", "privileged"},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
		},
	}
	rbacResult, err := common_rbac.ReconcileRbac(ctx, helper, instance, rbacRules)
	if err != nil {
		return rbacResult, err
	} else if (rbacResult != ctrl.Result{}) {
		return rbacResult, nil
	}

	// ConfigMap
	configVars := make(map[string]env.Setter)

	//
	// create RabbitMQ transportURL CR and get the actual URL from the associated secret that is created
	//

	transportURL, op, err := r.transportURLCreateOrUpdate(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.RabbitMqTransportURLReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.RabbitMqTransportURLReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("TransportURL %s successfully reconciled - operation: %s", transportURL.Name, string(op)))
	}

	instance.Status.TransportURLSecret = transportURL.Status.SecretName

	if instance.Status.TransportURLSecret == "" {
		r.Log.Info(fmt.Sprintf("Waiting for TransportURL %s secret to be created", transportURL.Name))
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.RabbitMqTransportURLReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.RabbitMqTransportURLReadyRunningMessage))
		return manila.ResultRequeue, nil
	}

	instance.Status.Conditions.MarkTrue(condition.RabbitMqTransportURLReadyCondition, condition.RabbitMqTransportURLReadyMessage)

	// end transportURL

	//
	// Check for required memcached used for caching
	//
	memcached, err := memcachedv1.GetMemcachedByName(ctx, helper, instance.Spec.MemcachedInstance, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			r.Log.Info(fmt.Sprintf("memcached %s not found", instance.Spec.MemcachedInstance))
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.MemcachedReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.MemcachedReadyWaitingMessage))
			return manila.ResultRequeue, nil
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.MemcachedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.MemcachedReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	if !memcached.IsReady() {
		r.Log.Info(fmt.Sprintf("memcached %s is not ready", memcached.Name))
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.MemcachedReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.MemcachedReadyWaitingMessage))
		return manila.ResultRequeue, nil
	}
	// Mark the Memcached Service as Ready if we get to this point with no errors
	instance.Status.Conditions.MarkTrue(
		condition.MemcachedReadyCondition, condition.MemcachedReadyMessage)
	// run check memcached - end

	//
	// check for required OpenStack secret holding passwords for service/admin user and add hash to the vars map
	//

	ctrlResult, err := verifyServiceSecret(
		ctx,
		types.NamespacedName{Namespace: instance.Namespace, Name: instance.Spec.Secret},
		[]string{
			instance.Spec.PasswordSelectors.Service,
		},
		helper.GetClient(),
		&instance.Status.Conditions,
		manila.NormalDuration,
		&configVars,
	)
	if (err != nil || ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}
	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)
	// run check OpenStack secret - end

	//
	// create service DB instance
	//
	db, result, err := r.ensureDB(ctx, helper, instance)
	if err != nil {
		return ctrl.Result{}, err
	} else if (result != ctrl.Result{}) {
		return result, nil
	}
	// create service DB - end

	//
	// Create ConfigMaps and Secrets required as input for the Service and calculate an overall hash of hashes
	//
	serviceLabels := map[string]string{
		common.AppSelector: manila.ServiceName,
	}
	//
	// create Config required for Manila input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal manila config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the OpenStack secret via the init container
	//
	err = r.generateServiceConfig(ctx, helper, instance, &configVars, serviceLabels, memcached, db)
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
	_, hashChanged, err := r.createHashOfInputHashes(instance, configVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	} else if hashChanged {
		r.Log.Info(fmt.Sprintf("%s... requeueing", condition.ServiceConfigReadyInitMessage))
		instance.Status.Conditions.MarkFalse(
			condition.ServiceConfigReadyCondition,
			condition.InitReason,
			condition.SeverityInfo,
			condition.ServiceConfigReadyInitMessage)
		// Hash changed and instance status should be updated (which will be done by main defer func),
		// so we need to return and reconcile again
		return ctrl.Result{}, nil
	}
	// Create Service Config and Secrets - end

	instance.Status.Conditions.MarkTrue(condition.ServiceConfigReadyCondition, condition.ServiceConfigReadyMessage)

	var serviceAnnotations map[string]string
	// Ensure NAD annotations are generated
	serviceAnnotations, ctrlResult, err = ensureNAD(ctx, &instance.Status.Conditions, instance.Spec.ManilaAPI.NetworkAttachments, helper)
	if err != nil {
		return ctrlResult, err
	}
	instance.Status.Conditions.MarkTrue(condition.NetworkAttachmentsReadyCondition, condition.NetworkAttachmentsReadyMessage)

	// Handle service init
	ctrlResult, err = r.reconcileInit(ctx, instance, helper, serviceLabels, serviceAnnotations)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	//
	// normal reconcile tasks
	//

	// deploy manila-api
	manilaAPI, op, err := r.apiDeploymentCreateOrUpdate(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			manilav1beta1.ManilaAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			manilav1beta1.ManilaAPIReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	apiObsGen, err := r.checkManilaAPIGeneration(instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			manilav1beta1.ManilaAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			manilav1beta1.ManilaAPIReadyErrorMessage,
			err.Error()))
		return ctrlResult, nil
	}
	if !apiObsGen {
		instance.Status.Conditions.Set(condition.UnknownCondition(
			manilav1beta1.ManilaAPIReadyCondition,
			condition.InitReason,
			manilav1beta1.ManilaAPIReadyInitMessage,
		))
	} else {
		// Mirror ManilaAPI status' ReadyCount to this parent CR
		instance.Status.ManilaAPIReadyCount = manilaAPI.Status.ReadyCount
		// Mirror ManilaAPI's condition status
		c := manilaAPI.Status.Conditions.Mirror(manilav1beta1.ManilaAPIReadyCondition)
		if c != nil {
			instance.Status.Conditions.Set(c)
		}
	}
	if op != controllerutil.OperationResultNone && apiObsGen {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// remove finalizers from unused MariaDBAccount records
	err = mariadbv1.DeleteUnusedMariaDBAccountFinalizers(
		ctx, helper, manila.DatabaseCRName,
		instance.Spec.DatabaseAccount, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Deploy ManilaScheduler
	manilaScheduler, op, err := r.schedulerDeploymentCreateOrUpdate(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			manilav1beta1.ManilaSchedulerReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			manilav1beta1.ManilaSchedulerReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	schedObsGen, err := r.checkManilaSchedulerGeneration(instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			manilav1beta1.ManilaSchedulerReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			manilav1beta1.ManilaSchedulerReadyErrorMessage,
			err.Error()))
		return ctrlResult, nil
	}
	if !schedObsGen {
		instance.Status.Conditions.Set(condition.UnknownCondition(
			manilav1beta1.ManilaSchedulerReadyCondition,
			condition.InitReason,
			manilav1beta1.ManilaSchedulerReadyInitMessage,
		))
	} else {
		// Mirror ManilaScheduler status' ReadyCount to this parent CR
		instance.Status.ManilaSchedulerReadyCount = manilaScheduler.Status.ReadyCount

		// Mirror ManilaScheduler's condition status
		c := manilaScheduler.Status.Conditions.Mirror(manilav1beta1.ManilaSchedulerReadyCondition)
		if c != nil {
			instance.Status.Conditions.Set(c)
		}
	}
	if op != controllerutil.OperationResultNone && schedObsGen {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Deploy ManilaShare
	var shareCondition *condition.Condition
	for name, share := range instance.Spec.ManilaShares {
		manilaShare, op, err := r.shareDeploymentCreateOrUpdate(ctx, instance, name, share, serviceLabels)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				manilav1beta1.ManilaShareReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				manilav1beta1.ManilaShareReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
		shareObsGen, err := r.checkManilaShareGeneration(instance)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				manilav1beta1.ManilaShareReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				manilav1beta1.ManilaShareReadyErrorMessage,
				err.Error()))
			return ctrlResult, nil
		}
		if !shareObsGen {
			instance.Status.Conditions.Set(condition.UnknownCondition(
				manilav1beta1.ManilaShareReadyCondition,
				condition.InitReason,
				manilav1beta1.ManilaShareReadyInitMessage,
			))
		} else {
			// Mirror ManilaShare status' ReadyCount to this parent CR
			if instance.Status.ManilaSharesReadyCounts == nil {
				instance.Status.ManilaSharesReadyCounts = map[string]int32{}
			}
			instance.Status.ManilaSharesReadyCounts[name] = manilaShare.Status.ReadyCount
		}
		if op != controllerutil.OperationResultNone && shareObsGen {
			r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
		}

		// If this manilaShare is not IsReady, mirror the condition to get the latest step it is in.
		// Could also check the overall ReadyCondition of the manilaShare.
		if !manilaShare.IsReady() {
			c := manilaShare.Status.Conditions.Mirror(manilav1beta1.ManilaShareReadyCondition)
			// Get the condition with higher priority for shareCondition.
			shareCondition = condition.GetHigherPrioCondition(c, shareCondition).DeepCopy()
		}
	}

	if shareCondition != nil {
		// If there was a Status=False condition, set that as the ManilaShareReadyCondition
		instance.Status.Conditions.Set(shareCondition)
	} else {
		// The ManilaShares are ready.
		// Using "condition.DeploymentReadyMessage" here because that is what gets mirrored
		// as the message for the other Manila children when they are successfully-deployed
		instance.Status.Conditions.MarkTrue(manilav1beta1.ManilaShareReadyCondition, condition.DeploymentReadyMessage)
	}

	// create CronJob
	cronjobDef := manila.CronJob(instance, serviceLabels, serviceAnnotations)
	cronjob := cronjob.NewCronJob(
		cronjobDef,
		manila.ShortDuration,
	)

	ctrlResult, err = cronjob.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.CronJobReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.CronJobReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	}

	instance.Status.Conditions.MarkTrue(condition.CronJobReadyCondition, condition.CronJobReadyMessage)
	// create CronJob - end

	cleanJob, hash, err := r.shareCleanup(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	if cleanJob {
		jobDef := manila.Job(
			instance,
			serviceLabels,
			serviceAnnotations,
			ptr.To(manila.TTL),
			fmt.Sprintf("%s-%s", manila.SvcCleanupJobName, hash[:manila.TruncateHash]),
			manila.SvcCleanupCommand,
			manila.ManilaServiceCleanupDelay,
		)
		shareCleanupJob := job.NewJob(
			jobDef,
			manilav1beta1.SvcCleanupHash,
			false,
			manila.ShortDuration,
			"",
		)
		ctrlResult, err := shareCleanupJob.DoJob(
			ctx,
			helper,
		)
		if err != nil {
			return ctrlResult, err
		}
	}
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' successfully", instance.Name))
	// update the overall status condition if service is ready
	if instance.IsReady() {
		instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
	}
	return ctrl.Result{}, nil
}

// generateServiceConfigMaps - create create configmaps which hold scripts and service configuration
func (r *ManilaReconciler) generateServiceConfig(
	ctx context.Context,
	h *helper.Helper,
	instance *manilav1beta1.Manila,
	envVars *map[string]env.Setter,
	serviceLabels map[string]string,
	memcached *memcachedv1.Memcached,
	db *mariadbv1.Database,
) error {
	//
	// create Secret required for manila input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal manila config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the ospSecret via the init container
	//

	labels := labels.GetLabels(instance, labels.GetGroupLabel(manila.ServiceName), serviceLabels)

	var tlsCfg *tls.Service
	if instance.Spec.ManilaAPI.TLS.Ca.CaBundleSecretName != "" {
		tlsCfg = &tls.Service{}
	}

	// customData hold any customization for the service.
	// custom.conf is going to /etc/<service>/<service>.conf.d
	// all other files get placed into /etc/<service> to allow overwrite of e.g. policy.json
	customData := map[string]string{
		manila.CustomConfigFileName: instance.Spec.CustomServiceConfig,
		"my.cnf":                    db.GetDatabaseClientConfig(tlsCfg), //(mschuppert) for now just get the default my.cnf
	}

	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, h, instance.Namespace, map[string]string{})
	if err != nil {
		return err
	}
	keystonePublicURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointPublic)
	if err != nil {
		return err
	}
	keystoneInternalURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointInternal)
	if err != nil {
		return err
	}

	ospSecret, _, err := secret.GetSecret(ctx, h, instance.Spec.Secret, instance.Namespace)
	if err != nil {
		return err
	}

	transportURLSecret, _, err := secret.GetSecret(ctx, h, instance.Status.TransportURLSecret, instance.Namespace)
	if err != nil {
		return err
	}

	databaseAccount := db.GetAccount()
	databaseSecret := db.GetSecret()

	// templateParameters := make(map[string]interface{})
	templateParameters := map[string]interface{}{
		"ServiceUser":         instance.Spec.ServiceUser,
		"ServicePassword":     string(ospSecret.Data[instance.Spec.PasswordSelectors.Service]),
		"KeystonePublicURL":   keystonePublicURL,
		"KeystoneInternalURL": keystoneInternalURL,
		"TransportURL":        string(transportURLSecret.Data["transport_url"]),
		"DatabaseConnection": fmt.Sprintf("mysql+pymysql://%s:%s@%s/%s?read_default_file=/etc/my.cnf",
			databaseAccount.Spec.UserName,
			string(databaseSecret.Data[mariadbv1.DatabasePasswordSelector]),
			instance.Status.DatabaseHostname,
			manila.DatabaseCRName),
		"MemcachedServersWithInet": memcached.GetMemcachedServerListWithInetString(),
		"TimeOut":                  instance.Spec.APITimeout,
	}

	// create httpd  vhost template parameters
	httpdVhostConfig := map[string]interface{}{}
	for _, endpt := range []service.Endpoint{service.EndpointInternal, service.EndpointPublic} {
		endptConfig := map[string]interface{}{}
		endptConfig["ServerName"] = fmt.Sprintf("%s-%s.%s.svc", manila.ServiceName, endpt.String(), instance.Namespace)
		endptConfig["TLS"] = false // default TLS to false, and set it bellow to true if enabled
		if instance.Spec.ManilaAPI.TLS.API.Enabled(endpt) {
			endptConfig["TLS"] = true
			endptConfig["SSLCertificateFile"] = fmt.Sprintf("/etc/pki/tls/certs/%s.crt", endpt.String())
			endptConfig["SSLCertificateKeyFile"] = fmt.Sprintf("/etc/pki/tls/private/%s.key", endpt.String())
		}
		httpdVhostConfig[endpt.String()] = endptConfig
	}
	templateParameters["VHosts"] = httpdVhostConfig

	configTemplates := []util.Template{
		// ScriptsConfigMap
		{
			Name:         fmt.Sprintf("%s-scripts", instance.Name),
			Namespace:    instance.Namespace,
			Type:         util.TemplateTypeScripts,
			InstanceType: instance.Kind,
			Labels:       labels,
		},
		// ConfigMap
		{
			Name:          fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:     instance.Namespace,
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			CustomData:    customData,
			ConfigOptions: templateParameters,
			Labels:        labels,
		},
	}

	return secret.EnsureSecrets(ctx, h, instance, configTemplates, envVars)
}

// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
//
// returns the hash, whether the hash changed (as a bool) and any error
func (r *ManilaReconciler) createHashOfInputHashes(
	instance *manilav1beta1.Manila,
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

func (r *ManilaReconciler) apiDeploymentCreateOrUpdate(ctx context.Context, instance *manilav1beta1.Manila) (*manilav1beta1.ManilaAPI, controllerutil.OperationResult, error) {
	deployment := &manilav1beta1.ManilaAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-api", instance.Name),
			Namespace: instance.Namespace,
		},
	}

	apiSpec := manilav1beta1.ManilaAPISpec{
		ManilaTemplate:     instance.Spec.ManilaTemplate,
		ManilaAPITemplate:  instance.Spec.ManilaAPI,
		ExtraMounts:        instance.Spec.ExtraMounts,
		DatabaseHostname:   instance.Status.DatabaseHostname,
		TransportURLSecret: instance.Status.TransportURLSecret,
		ServiceAccount:     instance.RbacResourceName(),
	}

	if apiSpec.NodeSelector == nil {
		apiSpec.NodeSelector = instance.Spec.NodeSelector
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec = apiSpec

		deployment.Spec.TransportURLSecret = instance.Status.TransportURLSecret

		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	return deployment, op, err
}

func (r *ManilaReconciler) schedulerDeploymentCreateOrUpdate(ctx context.Context, instance *manilav1beta1.Manila) (*manilav1beta1.ManilaScheduler, controllerutil.OperationResult, error) {
	deployment := &manilav1beta1.ManilaScheduler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-scheduler", instance.Name),
			Namespace: instance.Namespace,
		},
	}

	schedulerSpec := manilav1beta1.ManilaSchedulerSpec{
		ManilaTemplate:          instance.Spec.ManilaTemplate,
		ManilaSchedulerTemplate: instance.Spec.ManilaScheduler,
		ExtraMounts:             instance.Spec.ExtraMounts,
		DatabaseHostname:        instance.Status.DatabaseHostname,
		TransportURLSecret:      instance.Status.TransportURLSecret,
		ServiceAccount:          instance.RbacResourceName(),
		TLS:                     instance.Spec.ManilaAPI.TLS.Ca,
	}

	if schedulerSpec.NodeSelector == nil {
		schedulerSpec.NodeSelector = instance.Spec.NodeSelector
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec = schedulerSpec
		deployment.Spec.TransportURLSecret = instance.Status.TransportURLSecret

		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	return deployment, op, err
}

func (r *ManilaReconciler) shareDeploymentCreateOrUpdate(
	ctx context.Context,
	instance *manilav1beta1.Manila,
	name string,
	share manilav1beta1.ManilaShareTemplate,
	serviceLabels map[string]string,
) (*manilav1beta1.ManilaShare, controllerutil.OperationResult, error) {

	// Add the ShareName to the ManilaShare instance as a label
	serviceLabels[manilav1beta1.ShareNameLabel] = name
	deployment := &manilav1beta1.ManilaShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-share-%s", instance.Name, name),
			Namespace: instance.Namespace,
			Labels:    serviceLabels,
		},
	}

	shareSpec := manilav1beta1.ManilaShareSpec{
		ManilaTemplate:      instance.Spec.ManilaTemplate,
		ManilaShareTemplate: share,
		ExtraMounts:         instance.Spec.ExtraMounts,
		DatabaseHostname:    instance.Status.DatabaseHostname,
		TransportURLSecret:  instance.Status.TransportURLSecret,
		ServiceAccount:      instance.RbacResourceName(),
		TLS:                 instance.Spec.ManilaAPI.TLS.Ca,
	}

	if shareSpec.NodeSelector == nil {
		shareSpec.NodeSelector = instance.Spec.NodeSelector
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec = shareSpec
		deployment.Spec.TransportURLSecret = instance.Status.TransportURLSecret

		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	return deployment, op, err
}

func (r *ManilaReconciler) transportURLCreateOrUpdate(ctx context.Context, instance *manilav1beta1.Manila) (*rabbitmqv1.TransportURL, controllerutil.OperationResult, error) {
	transportURL := &rabbitmqv1.TransportURL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-manila-transport", instance.Name),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, transportURL, func() error {
		transportURL.Spec.RabbitmqClusterName = instance.Spec.RabbitMqClusterName

		err := controllerutil.SetControllerReference(instance, transportURL, r.Scheme)
		return err
	})

	return transportURL, op, err
}

func (r *ManilaReconciler) ensureDB(
	ctx context.Context,
	h *helper.Helper,
	instance *manilav1beta1.Manila,
) (*mariadbv1.Database, ctrl.Result, error) {
	// ensure MariaDBAccount exists.  This account record may be created by
	// openstack-operator or the cloud operator up front without a specific
	// MariaDBDatabase configured yet.   Otherwise, a MariaDBAccount CR is
	// created here with a generated username as well as a secret with
	// generated password.   The MariaDBAccount is created without being
	// yet associated with any MariaDBDatabase.
	_, _, err := mariadbv1.EnsureMariaDBAccount(
		ctx, h, instance.Spec.DatabaseAccount,
		instance.Namespace, false, manila.DatabaseUsernamePrefix,
	)

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			mariadbv1.MariaDBAccountReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			mariadbv1.MariaDBAccountNotReadyMessage,
			err.Error()))

		return nil, ctrl.Result{}, err
	}
	instance.Status.Conditions.MarkTrue(
		mariadbv1.MariaDBAccountReadyCondition,
		mariadbv1.MariaDBAccountReadyMessage)

	//
	// create service DB instance
	//
	db := mariadbv1.NewDatabaseForAccount(
		instance.Spec.DatabaseInstance, // mariadb/galera service to target
		manila.DatabaseName,            // name used in CREATE DATABASE in mariadb
		manila.DatabaseCRName,          // CR name for MariaDBDatabase
		instance.Spec.DatabaseAccount,  // CR name for MariaDBAccount
		instance.Namespace,             // namespace
	)

	// create or patch the DB
	ctrlResult, err := db.CreateOrPatchAll(ctx, h)

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return db, ctrl.Result{}, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return db, ctrlResult, nil
	}
	// wait for the DB to be setup
	// (ksambor) should we use WaitForDBCreatedWithTimeout instead?
	ctrlResult, err = db.WaitForDBCreated(ctx, h)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return db, ctrlResult, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return db, ctrlResult, nil
	}

	// update Status.DatabaseHostname, used to config the service
	instance.Status.DatabaseHostname = db.GetDatabaseHostname()
	instance.Status.Conditions.MarkTrue(condition.DBReadyCondition, condition.DBReadyMessage)
	return db, ctrlResult, nil
}

// checkManilaAPIGeneration -
func (r *ManilaReconciler) checkManilaAPIGeneration(
	instance *manilav1beta1.Manila,
) (bool, error) {
	api := &manilav1beta1.ManilaAPIList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := r.Client.List(context.Background(), api, listOpts...); err != nil {
		r.Log.Error(err, "Unable to retrieve ManilaAPI %w")
		return false, err
	}
	for _, item := range api.Items {
		if item.Generation != item.Status.ObservedGeneration {
			return false, nil
		}
	}
	return true, nil
}

// checkManilaSchedulerGeneration -
func (r *ManilaReconciler) checkManilaSchedulerGeneration(
	instance *manilav1beta1.Manila,
) (bool, error) {
	sched := &manilav1beta1.ManilaSchedulerList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := r.Client.List(context.Background(), sched, listOpts...); err != nil {
		r.Log.Error(err, "Unable to retrieve ManilaScheduler %w")
		return false, err
	}
	for _, item := range sched.Items {
		if item.Generation != item.Status.ObservedGeneration {
			return false, nil
		}
	}
	return true, nil
}

// checkManilaShareGeneration -
func (r *ManilaReconciler) checkManilaShareGeneration(
	instance *manilav1beta1.Manila,
) (bool, error) {
	share := &manilav1beta1.ManilaShareList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := r.Client.List(context.Background(), share, listOpts...); err != nil {
		r.Log.Error(err, "Unable to retrieve ManilaShare %w")
		return false, err
	}
	for _, item := range share.Items {
		if item.Generation != item.Status.ObservedGeneration {
			return false, nil
		}
	}
	return true, nil
}

// shareCleanup - Delete manila share instances that no longer appears in the spec.
func (r *ManilaReconciler) shareCleanup(
	ctx context.Context,
	instance *manilav1beta1.Manila,
) (bool, string, error) {
	cleanJob := false
	var deletedShares = []string{}
	// Generate a list of share CRs
	shares := &manilav1beta1.ManilaShareList{}

	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := r.Client.List(ctx, shares, listOpts...); err != nil {
		r.Log.Error(err, "Unable to retrieve Manila Share CRs %v")
		return cleanJob, "", nil
	}
	for _, share := range shares.Items {
		// Skip shares CRs that we don't own
		if manila.GetOwningManilaName(&share) != instance.Name {
			continue
		}
		// Delete the manilaShare if it's no longer in the spec
		_, exists := instance.Spec.ManilaShares[share.ShareName()]
		if !exists && share.DeletionTimestamp.IsZero() {
			err := r.Client.Delete(ctx, &share)
			if err != nil && !k8s_errors.IsNotFound(err) {
				err = fmt.Errorf("Error cleaning up %s: %w", share.Name, err)
				return cleanJob, "", err
			}
			delete(instance.Status.ManilaSharesReadyCounts, share.ShareName())
			deletedShares = append(deletedShares, share.ShareName())
			cleanJob = true
		}
	}
	hash, err := manila.SharesListHash(deletedShares)
	if err != nil {
		return false, hash, err
	}
	return cleanJob, hash, nil
}
