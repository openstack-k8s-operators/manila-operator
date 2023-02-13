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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"

	routev1 "github.com/openshift/api/route/v1"
	keystonev1 "github.com/***REMOVED***-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/condition"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/configmap"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/deployment"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/env"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/helper"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/labels"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/secret"
	"github.com/***REMOVED***-k8s-operators/lib-common/modules/common/util"
	manilav1beta1 "github.com/***REMOVED***-k8s-operators/manila-operator/api/v1beta1"
	"github.com/***REMOVED***-k8s-operators/manila-operator/pkg/manila"
	manilaapi "github.com/***REMOVED***-k8s-operators/manila-operator/pkg/manilaapi"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ManilaAPIReconciler reconciles a ManilaAPI object
type ManilaAPIReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
	Log     logr.Logger
}

var (
	keystoneServices = []map[string]string{
		{
			"type": manila.ServiceTypeV2,
			"name": manila.ServiceNameV2,
			"desc": "Manila V2 Service",
		},
		{
			"type": manila.ServiceType,
			"name": manila.ServiceName,
			"desc": "Manila V3 Service",
		},
	}
)

//+kubebuilder:rbac:groups=manila.***REMOVED***.org,resources=manilaapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=manila.***REMOVED***.org,resources=manilaapis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=manila.***REMOVED***.org,resources=manilaapis/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=keystone.***REMOVED***.org,resources=keystoneservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keystone.***REMOVED***.org,resources=keystoneendpoints,verbs=get;list;watch;create;update;patch;delete

// GetClient -
func (r *ManilaAPIReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *ManilaAPIReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *ManilaAPIReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *ManilaAPIReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// Reconcile -
func (r *ManilaAPIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch the ManilaAPI instance
	instance := &manilav1beta1.ManilaAPI{}
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

	//
	// initialize status
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		// initialize conditions used later as Status=Unknown
		cl := condition.CreateList(
			condition.UnknownCondition(condition.ExposeServiceReadyCondition, condition.InitReason, condition.ExposeServiceReadyInitMessage),
			condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
			condition.UnknownCondition(condition.ServiceConfigReadyCondition, condition.InitReason, condition.ServiceConfigReadyInitMessage),
			condition.UnknownCondition(condition.DeploymentReadyCondition, condition.InitReason, condition.DeploymentReadyInitMessage),
			// right now we have no dedicated KeystoneServiceReadyInitMessage and KeystoneEndpointReadyInitMessage
			condition.UnknownCondition(condition.KeystoneServiceReadyCondition, condition.InitReason, ""),
			condition.UnknownCondition(condition.KeystoneEndpointReadyCondition, condition.InitReason, ""),
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
	if instance.Status.ServiceIDs == nil {
		instance.Status.ServiceIDs = map[string]string{}
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
func (r *ManilaAPIReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// watch for configmap where the CM owner label AND the CR.Spec.ManagingCrName label matches
	configMapFn := func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all API CRs
		apis := &manilav1beta1.ManilaAPIList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), apis, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve API CRs %v")
			return nil
		}

		label := o.GetLabels()
		// TODO: Just trying to verify that the CM is owned by this CR's managing CR
		if l, ok := label[labels.GetOwnerNameLabelSelector(labels.GetGroupLabel(manila.ServiceName))]; ok {
			for _, cr := range apis.Items {
				// return reconcil event for the CR where the CM owner label AND the parentManilaName matches
				if l == manila.GetOwningManilaName(&cr) {
					// return namespace and Name of CR
					name := client.ObjectKey{
						Namespace: o.GetNamespace(),
						Name:      cr.Name,
					}
					r.Log.Info(fmt.Sprintf("ConfigMap object %s and CR %s marked with label: %s", o.GetName(), cr.Name, l))
					result = append(result, reconcile.Request{NamespacedName: name})
				}
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&manilav1beta1.ManilaAPI{}).
		Owns(&keystonev1.KeystoneService{}).
		Owns(&keystonev1.KeystoneEndpoint{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Owns(&routev1.Route{}).
		Owns(&corev1.Service{}).
		// watch the config CMs we don't own
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(configMapFn)).
		Complete(r)
}

func (r *ManilaAPIReconciler) reconcileDelete(ctx context.Context, instance *manilav1beta1.ManilaAPI, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// It's possible to get here before the endpoints have been set in the status, so check for this
	if instance.Status.APIEndpoints != nil {
		for _, ksSvc := range keystoneServices {

			// Remove the finalizer from our KeystoneEndpoint CR
			keystoneEndpoint, err := keystonev1.GetKeystoneEndpointWithName(ctx, helper, ksSvc["name"], instance.Namespace)
			if err != nil && !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}

			if err == nil {
				controllerutil.RemoveFinalizer(keystoneEndpoint, helper.GetFinalizer())
				if err = helper.GetClient().Update(ctx, keystoneEndpoint); err != nil && !k8s_errors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
				util.LogForObject(helper, "Removed finalizer from our KeystoneEndpoint", instance)
			}

			// Remove the finalizer from our KeystoneService CR
			keystoneService, err := keystonev1.GetKeystoneServiceWithName(ctx, helper, ksSvc["name"], instance.Namespace)
			if err != nil && !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}

			if err == nil {
				controllerutil.RemoveFinalizer(keystoneService, helper.GetFinalizer())
				if err = helper.GetClient().Update(ctx, keystoneService); err != nil && !k8s_errors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
				util.LogForObject(helper, "Removed finalizer from our KeystoneService", instance)
			}
		}
	}

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))
	if err := r.Update(ctx, instance); err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ManilaAPIReconciler) reconcileInit(
	ctx context.Context,
	instance *manilav1beta1.ManilaAPI,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' init", instance.Name))

	//
	// expose the service (create service, route and return the created endpoint URLs)
	//

	// V2
	adminEndpointData := endpoint.Data{
		Port: manila.ManilaAdminPort,
		Path: "/v2",
	}
	publicEndpointData := endpoint.Data{
		Port: manila.ManilaPublicPort,
		Path: "/v2",
	}
	internalEndpointData := endpoint.Data{
		Port: manila.ManilaInternalPort,
		Path: "/v2",
	}
	data := map[endpoint.Endpoint]endpoint.Data{
		endpoint.EndpointAdmin:    adminEndpointData,
		endpoint.EndpointPublic:   publicEndpointData,
		endpoint.EndpointInternal: internalEndpointData,
	}

	apiEndpointsV2, ctrlResult, err := endpoint.ExposeEndpoints(
		ctx,
		helper,
		manila.ServiceName,
		serviceLabels,
		data,
	)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ExposeServiceReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ExposeServiceReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ExposeServiceReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.ExposeServiceReadyRunningMessage))
		return ctrlResult, nil
	}

	//
	// Update instance status with service endpoint url from route host information for v2
	//
	// TODO: need to support https default here
	if instance.Status.APIEndpoints == nil {
		instance.Status.APIEndpoints = map[string]map[string]string{}
	}
	instance.Status.APIEndpoints[manila.ServiceNameV2] = apiEndpointsV2
	// V2 - end

	// V1 - Legacy
	adminEndpointData = endpoint.Data{
		Port: manila.ManilaAdminPort,
		Path: "/v1/%(tenant_id)s",
	}
	publicEndpointData = endpoint.Data{
		Port: manila.ManilaPublicPort,
		Path: "/v1/%(tenant_id)s",
	}
	internalEndpointData = endpoint.Data{
		Port: manila.ManilaInternalPort,
		Path: "/v1/%(tenant_id)s",
	}
	data = map[endpoint.Endpoint]endpoint.Data{
		endpoint.EndpointAdmin:    adminEndpointData,
		endpoint.EndpointPublic:   publicEndpointData,
		endpoint.EndpointInternal: internalEndpointData,
	}

	apiEndpointsV1, ctrlResult, err := endpoint.ExposeEndpoints(
		ctx,
		helper,
		manila.ServiceName,
		serviceLabels,
		data,
	)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ExposeServiceReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ExposeServiceReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ExposeServiceReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.ExposeServiceReadyRunningMessage))
		return ctrlResult, nil
	}

	//
	// Update instance status with service endpoint url from route host information for v2
	//
	// TODO: need to support https default here
	instance.Status.APIEndpoints[manila.ServiceName] = apiEndpointsV1
	// V1 - end

	instance.Status.Conditions.MarkTrue(condition.ExposeServiceReadyCondition, condition.ExposeServiceReadyMessage)

	// expose service - end

	//
	// create service and user in keystone - - https://docs.***REMOVED***.org/manila/ocata/adminref/quick_start.html
	//
	if instance.Status.ServiceIDs == nil {
		instance.Status.ServiceIDs = map[string]string{}
	}

	for _, ksSvc := range keystoneServices {
		ksSvcSpec := keystonev1.KeystoneServiceSpec{
			ServiceType:        ksSvc["type"],
			ServiceName:        ksSvc["name"],
			ServiceDescription: ksSvc["desc"],
			Enabled:            true,
			ServiceUser:        instance.Spec.ServiceUser,
			Secret:             instance.Spec.Secret,
			PasswordSelector:   instance.Spec.PasswordSelectors.Service,
		}

		ksSvcObj := keystonev1.NewKeystoneService(ksSvcSpec, instance.Namespace, serviceLabels, 10)
		ctrlResult, err = ksSvcObj.CreateOrPatch(ctx, helper)
		if err != nil {
			return ctrlResult, err
		}

		// mirror the Status, Reason, Severity and Message of the latest keystoneservice condition
		// into a local condition with the type condition.KeystoneServiceReadyCondition
		c := ksSvcObj.GetConditions().Mirror(condition.KeystoneServiceReadyCondition)
		if c != nil {
			instance.Status.Conditions.Set(c)
		}

		if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		instance.Status.ServiceIDs[ksSvc["name"]] = ksSvcObj.GetServiceID()

		ksEndptSpec := keystonev1.KeystoneEndpointSpec{
			ServiceName: ksSvc["name"],
			Endpoints:   instance.Status.APIEndpoints[ksSvc["name"]],
		}

		ksEndptObj := keystonev1.NewKeystoneEndpoint(
			ksSvc["name"],
			instance.Namespace,
			ksEndptSpec,
			serviceLabels,
			10)
		ctrlResult, err = ksEndptObj.CreateOrPatch(ctx, helper)
		if err != nil {
			return ctrlResult, err
		}

		// mirror the Status, Reason, Severity and Message of the latest keystoneendpoint condition
		// into a local condition with the type condition.KeystoneEndpointReadyCondition
		c = ksEndptObj.GetConditions().Mirror(condition.KeystoneEndpointReadyCondition)
		if c != nil {
			instance.Status.Conditions.Set(c)
		}

		if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
	}

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' init successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaAPIReconciler) reconcileNormal(ctx context.Context, instance *manilav1beta1.ManilaAPI, helper *helper.Helper) (ctrl.Result, error) {
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
	// run check OpenStack secret - end

	//
	// check for required Manila config maps that should have been created by parent Manila CR
	//

	parentManilaName := manila.GetOwningManilaName(instance)

	configMaps := []string{
		fmt.Sprintf("%s-scripts", parentManilaName),     //ScriptsConfigMap
		fmt.Sprintf("%s-config-data", parentManilaName), //ConfigMap
	}

	_, err = configmap.GetConfigMaps(ctx, helper, instance, configMaps, instance.Namespace, &configMapVars)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.InputReadyWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("Could not find all config maps for parent Manila CR %s", parentManilaName)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)
	// run check parent Manila CR config maps - end

	//
	// Create ConfigMaps required as input for the Service and calculate an overall hash of hashes
	//

	//
	// create custom Configmap for this manila-api service
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
	// Create ConfigMaps - end

	//
	// create hash over all the different input resources to identify if any those changed
	// and a restart/recreate is required.
	//
	inputHash, err := r.createHashOfInputHashes(ctx, instance, configMapVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	instance.Status.Conditions.MarkTrue(condition.ServiceConfigReadyCondition, condition.ServiceConfigReadyMessage)
	// Create ConfigMaps and Secrets - end

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

	// Define a new Deployment object
	depl := deployment.NewDeployment(
		manilaapi.Deployment(instance, inputHash, serviceLabels),
		5,
	)

	ctrlResult, err = depl.CreateOrPatch(ctx, helper)
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
	instance.Status.ReadyCount = depl.GetDeployment().Status.ReadyReplicas
	if instance.Status.ReadyCount > 0 {
		instance.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition, condition.DeploymentReadyMessage)
	}
	// create Deployment - end

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaAPIReconciler) reconcileUpdate(ctx context.Context, instance *manilav1beta1.ManilaAPI, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' update", instance.Name))

	// TODO: should have minor update tasks if required
	// - delete dbsync hash from status to rerun it?

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' update successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *ManilaAPIReconciler) reconcileUpgrade(ctx context.Context, instance *manilav1beta1.ManilaAPI, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' upgrade", instance.Name))

	// TODO: should have major version upgrade tasks
	// -delete dbsync hash from status to rerun it?

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' upgrade successfully", instance.Name))
	return ctrl.Result{}, nil
}

// generateServiceConfigMaps - create custom configmap to hold service-specific config
// TODO add DefaultConfigOverwrite
func (r *ManilaAPIReconciler) generateServiceConfigMaps(
	ctx context.Context,
	h *helper.Helper,
	instance *manilav1beta1.ManilaAPI,
	envVars *map[string]env.Setter,
) error {
	//
	// create custom Configmap for manila-api-specific config input
	// - %-config-data configmap holding custom config for the service's manila.conf
	//

	cmLabels := labels.GetLabels(instance, labels.GetGroupLabel(manila.ServiceName), map[string]string{})

	// customData hold any customization for the service.
	// custom.conf is going to be merged into /etc/manila/manila.conf
	// TODO: make sure custom.conf can not be overwritten
	customData := map[string]string{common.CustomServiceConfigFileName: instance.Spec.CustomServiceConfig}

	for key, data := range instance.Spec.DefaultConfigOverwrite {
		customData[key] = data
	}

	customData[common.CustomServiceConfigFileName] = instance.Spec.CustomServiceConfig

	cms := []util.Template{
		// Custom ConfigMap
		{
			Name:         fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:    instance.Namespace,
			Type:         util.TemplateTypeConfig,
			InstanceType: instance.Kind,
			CustomData:   customData,
			Labels:       cmLabels,
		},
	}

	err := configmap.EnsureConfigMaps(ctx, h, instance, cms, envVars)
	if err != nil {
		return nil
	}

	return nil
}

// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
func (r *ManilaAPIReconciler) createHashOfInputHashes(
	ctx context.Context,
	instance *manilav1beta1.ManilaAPI,
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
		r.Log.Info(fmt.Sprintf("Service '%s' Input maps hash %s - %s", instance.Name, common.InputHashName, hash))
	}
	return hash, nil
}
