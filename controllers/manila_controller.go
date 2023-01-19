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

	"github.com/go-logr/logr"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/job"
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/database"
	manilav1beta1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

//+kubebuilder:rbac:groups=manila.openstack.org,resources=manilas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=manila.openstack.org,resources=manilas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=manila.openstack.org,resources=manilas/finalizers,verbs=update
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaapis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaapis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaapis/finalizers,verbs=update
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaschedulers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaschedulers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilaschedulers/finalizers,verbs=update
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilabackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilabackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilabackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilavolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilavolumes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilavolumes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbdatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch

// Reconcile -
func (r *ManilaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	instance := &manilav1beta1.Manila{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// initialize status

	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		// initialize conditions used later as Status=Unknown
		cl := condition.CreateList(
			condition.UnknownCondition(condition.DBReadyCondition, condition.InitReason, condition.DBReadyInitMessage),
			condition.UnknownCondition(condition.DBSyncReadyCondition, condition.InitReason, condition.DBSyncReadyInitMessage),
			condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
			condition.UnknownCondition(manilav1beta1.ManilaAPIReadyCondition, condition.InitReason, manilav1beta1.ManilaAPIReadyInitMessage),
			condition.UnknownCondition(manilav1beta1.ManilaSchedulerReadyCondition, condition.InitReason, manilav1beta1.ManilaSchedulerReadyInitMessage),
			condition.UnknownCondition(manilav1beta1.ManilaShareReadyCondition, condition.InitReason, manilav1beta1.ManilaShareReadyInitMessage),
			condition.UnknownCondition(condition.DeploymentReadyCondition, condition.InitReason, condition.DeploymentReadyInitMessage),
		)

		instance.Status.Conditions.Init(&cl)

		// Register overall status immediately to have an early feedback e.g. in the cli
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}
	if instance.Status.APIEndpoints == nil {
		instance.Status.APIEndpoints = map[string]map[string]string{}
	}
	if instance.Status.ManilaSharesReadyCounts == nil {
		instance.Status.ManilaSharesReadyCounts = map[string]int32{}
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
		// update the overall status condition if service is ready
		if instance.IsReady() {
			instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		}

		if err := helper.SetAfter(instance); err != nil {
			util.LogErrorForObject(helper, err, "Set after and calc patch/diff", instance)
		}

		if changed := helper.GetChanges()["status"]; changed {
			patch := client.MergeFrom(helper.GetBeforeObject())

			if err := r.Status().Patch(ctx, instance, patch); err != nil && !k8s_errors.IsNotFound(err) {
				util.LogErrorForObject(helper, err, "Update status", instance)
			}
		}
	}()

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManilaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&manilav1beta1.Manila{}).
		Owns(&mariadbv1.MariaDBDatabase{}).
		Owns(&manilav1beta1.ManilaAPI{}).
		Owns(&manilav1beta1.ManilaScheduler{}).
		Owns(&manilav1beta1.ManilaShare{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func (r *ManilaReconciler) reconcileDelete(ctx context.Context, instance *manilav1beta1.Manila, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))
	if err := r.Update(ctx, instance); err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ManilaReconciler) reconcileInit(
	ctx context.Context,
	instance *manilav1beta1.Manila,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' init", instance.Name))

	//
	// create service DB instance
	//
	db := database.NewDatabase(
		instance.Name,
		instance.Spec.DatabaseUser,
		instance.Spec.Secret,
		map[string]string{
			"dbName": instance.Spec.DatabaseInstance,
		},
	)
	// create or patch the DB
	ctrlResult, err := db.CreateOrPatchDB(
		ctx,
		helper,
	)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return ctrlResult, nil
	}
	// wait for the DB to be setup
	ctrlResult, err = db.WaitForDBCreated(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return ctrlResult, nil
	}
	// update Status.DatabaseHostname, used to config the service
	instance.Status.DatabaseHostname = db.GetDatabaseHostname()
	instance.Status.Conditions.MarkTrue(condition.DBReadyCondition, condition.DBReadyMessage)
	// create service DB - end

	//
	// run manila db sync
	//
	dbSyncHash := instance.Status.Hash[manilav1beta1.DbSyncHash]
	jobDef := manila.DbSyncJob(instance, serviceLabels)
	dbSyncjob := job.NewJob(
		jobDef,
		manilav1beta1.DbSyncHash,
		instance.Spec.PreserveJobs,
		5,
		dbSyncHash,
	)
	ctrlResult, err = dbSyncjob.DoJob(
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
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info(fmt.Sprintf("Service '%s' - Job %s hash added - %s", instance.Name, jobDef.Name, instance.Status.Hash[manilav1beta1.DbSyncHash]))
	}
	instance.Status.Conditions.MarkTrue(condition.DBSyncReadyCondition, condition.DBSyncReadyMessage)

	// run Manila db sync - end

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' init successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaReconciler) reconcileNormal(ctx context.Context, instance *manilav1beta1.Manila, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))

	// If the service object doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(instance, helper.GetFinalizer())
	// Register the finalizer immediately to avoid orphaning resources on delete
	if err := r.Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// ConfigMap
	configMapVars := make(map[string]env.Setter)

	//
	// check for required OpenStack secret holding passwords for service/admin user and add hash to the vars map
	//
	ospSecret, hash, err := secret.GetSecret(ctx, helper, instance.Spec.Secret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.InputReadyWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("OpenStack secret %s not found", instance.Spec.Secret)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	configMapVars[ospSecret.Name] = env.SetValue(hash)

	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)
	// run check OpenStack secret - end

	//
	// Create ConfigMaps and Secrets required as input for the Service and calculate an overall hash of hashes
	//

	//
	// create Configmap required for manila input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal manila config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the OpenStack secret via the init container
	//
	err = r.generateServiceConfigMaps(ctx, helper, instance, &configMapVars)
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
	_, err = r.createHashOfInputHashes(ctx, instance, configMapVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	// Create ConfigMaps and Secrets - end

	instance.Status.Conditions.MarkTrue(condition.ServiceConfigReadyCondition, condition.ServiceConfigReadyMessage)

	//
	// TODO check when/if Init, Update, or Upgrade should/could be skipped
	//

	serviceLabels := map[string]string{
		common.AppSelector: manila.ServiceName,
	}

	// Handle service init
	ctrlResult, err := r.reconcileInit(ctx, instance, helper, serviceLabels)
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

	// deploy manila-api
	manilaAPI, op, err := r.apiDeploymentCreateOrUpdate(instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			manilav1beta1.ManilaAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			manilav1beta1.ManilaAPIReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Mirror ManilaAPI status' APIEndpoints and ReadyCount to this parent CR
	instance.Status.APIEndpoints = manilaAPI.Status.APIEndpoints
	instance.Status.ServiceIDs = manilaAPI.Status.ServiceIDs
	instance.Status.ManilaAPIReadyCount = manilaAPI.Status.ReadyCount

	// Mirror ManilaAPI's condition status
	c := manilaAPI.Status.Conditions.Mirror(manilav1beta1.ManilaAPIReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

	// TODO: These will not work without rabbit yet
	// deploy manila-scheduler
	manilaScheduler, op, err := r.schedulerDeploymentCreateOrUpdate(instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			manilav1beta1.ManilaSchedulerReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			manilav1beta1.ManilaSchedulerReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Mirror ManilaScheduler status' ReadyCount to this parent CR
	instance.Status.ManilaSchedulerReadyCount = manilaScheduler.Status.ReadyCount

	// Mirror ManilaScheduler's condition status
	c = manilaScheduler.Status.Conditions.Mirror(manilav1beta1.ManilaSchedulerReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

	// TODO: This requires some sort of backend and rabbit, and will not work without them
	// deploy manila-share
	var shareCondition *condition.Condition
	for name, share := range instance.Spec.ManilaShares {
		manilaShare, op, err := r.shareDeploymentCreateOrUpdate(instance, name, share)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				manilav1beta1.ManilaShareReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				manilav1beta1.ManilaShareReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
		}

		// Mirror ManilaShare status' ReadyCount to this parent CR
		// TODO: Somehow this status map can be nil here despite being initialized
		//       in the Reconcile function above
		if instance.Status.ManilaSharesReadyCounts == nil {
			instance.Status.ManilaSharesReadyCounts = map[string]int32{}
		}
		instance.Status.ManilaSharesReadyCounts[name] = manilaShare.Status.ReadyCount

		// If this manilaShare is not IsReady, mirror the condition to get the latest step it is in.
		// Could also check the overall ReadyCondition of the manilaShare.
		if !manilaShare.IsReady() {
			c = manilaShare.Status.Conditions.Mirror(manilav1beta1.ManilaShareReadyCondition)
			// Get the condition with higher priority for volumeCondition.
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

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaReconciler) reconcileUpdate(ctx context.Context, instance *manilav1beta1.Manila, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' update", instance.Name))

	// TODO: should have minor update tasks if required
	// - delete dbsync hash from status to rerun it?

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' update successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaReconciler) reconcileUpgrade(ctx context.Context, instance *manilav1beta1.Manila, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' upgrade", instance.Name))

	// TODO: should have major version upgrade tasks
	// -delete dbsync hash from status to rerun it?

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' upgrade successfully", instance.Name))
	return ctrl.Result{}, nil
}

//
// generateServiceConfigMaps - create create configmaps which hold scripts and service configuration
// TODO add DefaultConfigOverwrite
//
func (r *ManilaReconciler) generateServiceConfigMaps(
	ctx context.Context,
	h *helper.Helper,
	instance *manilav1beta1.Manila,
	envVars *map[string]env.Setter,
) error {
	//
	// create Configmap/Secret required for manila input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal manila config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the ospSecret via the init container
	//

	cmLabels := labels.GetLabels(instance, labels.GetGroupLabel(manila.ServiceName), map[string]string{})

	// customData hold any customization for the service.
	// custom.conf is going to /etc/<service>/<service>.conf.d
	// all other files get placed into /etc/<service> to allow overwrite of e.g. policy.json
	// TODO: make sure custom.conf can not be overwritten
	customData := map[string]string{common.CustomServiceConfigFileName: instance.Spec.CustomServiceConfig}

	for key, data := range instance.Spec.DefaultConfigOverwrite {
		customData[key] = data
	}

	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, h, instance.Namespace, map[string]string{})
	if err != nil {
		return err
	}
	authURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointPublic)
	if err != nil {
		return err
	}
	templateParameters := make(map[string]interface{})
	templateParameters["ServiceUser"] = instance.Spec.ServiceUser
	templateParameters["KeystonePublicURL"] = authURL

	cms := []util.Template{
		// ScriptsConfigMap
		{
			Name:               fmt.Sprintf("%s-scripts", instance.Name),
			Namespace:          instance.Namespace,
			Type:               util.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{"common.sh": "/common/common.sh"},
			Labels:             cmLabels,
		},
		// ConfigMap
		{
			Name:          fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:     instance.Namespace,
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			CustomData:    customData,
			ConfigOptions: templateParameters,
			Labels:        cmLabels,
		},
	}

	err = configmap.EnsureConfigMaps(ctx, h, instance, cms, envVars)
	if err != nil {
		return err
	}

	return nil
}

//
// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
//
func (r *ManilaReconciler) createHashOfInputHashes(
	ctx context.Context,
	instance *manilav1beta1.Manila,
	envVars map[string]env.Setter,
) (string, error) {
	mergedMapVars := env.MergeEnvs([]corev1.EnvVar{}, envVars)
	hash, err := util.ObjectHash(mergedMapVars)
	if err != nil {
		return hash, err
	}
	if hashMap, changed := util.SetHash(instance.Status.Hash, common.InputHashName, hash); changed {
		instance.Status.Hash = hashMap
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			return hash, err
		}
		r.Log.Info(fmt.Sprintf("Service '%s' - Input maps hash %s - %s", instance.Name, common.InputHashName, hash))
	}
	return hash, nil
}

func (r *ManilaReconciler) apiDeploymentCreateOrUpdate(instance *manilav1beta1.Manila) (*manilav1beta1.ManilaAPI, controllerutil.OperationResult, error) {
	deployment := &manilav1beta1.ManilaAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-api", instance.Name),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, deployment, func() error {
		deployment.Spec = instance.Spec.ManilaAPI
		// Add in transfers from umbrella Manila (this instance) spec
		// TODO: Add logic to determine when to set/overwrite, etc
		deployment.Spec.ServiceUser = instance.Spec.ServiceUser
		deployment.Spec.DatabaseHostname = instance.Status.DatabaseHostname
		deployment.Spec.DatabaseUser = instance.Spec.DatabaseUser
		deployment.Spec.Secret = instance.Spec.Secret

		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	return deployment, op, err
}

func (r *ManilaReconciler) schedulerDeploymentCreateOrUpdate(instance *manilav1beta1.Manila) (*manilav1beta1.ManilaScheduler, controllerutil.OperationResult, error) {
	deployment := &manilav1beta1.ManilaScheduler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-scheduler", instance.Name),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, deployment, func() error {
		deployment.Spec = instance.Spec.ManilaScheduler
		// Add in transfers from umbrella Manila CR (this instance) spec
		// TODO: Add logic to determine when to set/overwrite, etc
		deployment.Spec.ServiceUser = instance.Spec.ServiceUser
		deployment.Spec.DatabaseHostname = instance.Status.DatabaseHostname
		deployment.Spec.DatabaseUser = instance.Spec.DatabaseUser
		deployment.Spec.Secret = instance.Spec.Secret

		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	return deployment, op, err
}

func (r *ManilaReconciler) shareDeploymentCreateOrUpdate(instance *manilav1beta1.Manila, name string, volume manilav1beta1.ManilaShareSpec) (*manilav1beta1.ManilaShare, controllerutil.OperationResult, error) {
	deployment := &manilav1beta1.ManilaShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-share-%s", instance.Name, name),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, deployment, func() error {
		deployment.Spec = volume
		// Add in transfers from umbrella Manila CR (this instance) spec
		// TODO: Add logic to determine when to set/overwrite, etc
		deployment.Spec.ServiceUser = instance.Spec.ServiceUser
		deployment.Spec.DatabaseHostname = instance.Status.DatabaseHostname
		deployment.Spec.DatabaseUser = instance.Spec.DatabaseUser
		deployment.Spec.Secret = instance.Spec.Secret

		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	return deployment, op, err
}
