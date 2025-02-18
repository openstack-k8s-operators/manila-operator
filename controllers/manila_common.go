/*
Copyright 2024.

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

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"k8s.io/apimachinery/pkg/types"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	nad "github.com/openstack-k8s-operators/lib-common/modules/common/networkattachment"
	"github.com/openstack-k8s-operators/manila-operator/pkg/manila"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type conditionUpdater interface {
	Set(c *condition.Condition)
	MarkTrue(t condition.Type, messageFormat string, messageArgs ...interface{})
}

// ensureManilaTopology - when a Topology CR is referenced, remove the
// finalizer from a previous referenced Topology (if any), and retrieve the
// newly referenced topology object
func ensureManilaTopology(
	ctx context.Context,
	helper *helper.Helper,
	tpRef *topologyv1.TopoRef,
	lastAppliedTopology *topologyv1.TopoRef,
	finalizer string,
	selector string,
) (*topologyv1.Topology, error) {

	var podTopology *topologyv1.Topology
	var err error

	// Remove (if present) the finalizer from a previously referenced topology
	//
	// 1. a topology reference is removed (tpRef == nil) from the Manila Component
	//    subCR and the finalizer should be deleted from the last applied topology
	//    (lastAppliedTopology != "")
	// 2. a topology reference is updated in the Manila Component CR (tpRef != nil)
	//    and the finalizer should be removed from the previously
	//    referenced topology (tpRef.Name != lastAppliedTopology.Name)
	if (tpRef == nil && lastAppliedTopology.Name != "") ||
		(tpRef != nil && tpRef.Name != lastAppliedTopology.Name) {
		_, err = topologyv1.EnsureDeletedTopologyRef(
			ctx,
			helper,
			lastAppliedTopology,
			finalizer,
		)
		if err != nil {
			return nil, err
		}
	}
	// TopologyRef is passed as input, get the Topology object
	if tpRef != nil {
		// no Namespace is provided, default to instance.Namespace
		if tpRef.Namespace == "" {
			tpRef.Namespace = helper.GetBeforeObject().GetNamespace()
		}
		// Build a defaultLabelSelector (component=manila-[api|scheduler|share])
		defaultLabelSelector := labels.GetSingleLabelSelector(
			common.ComponentSelector,
			selector,
		)
		// Retrieve the referenced Topology
		podTopology, _, err = topologyv1.EnsureTopologyRef(
			ctx,
			helper,
			tpRef,
			finalizer,
			&defaultLabelSelector,
		)
		if err != nil {
			return nil, err
		}
	}
	return podTopology, nil
}

// ensureNAD - common function called in the Manila controllers that GetNAD based
// on the string[] passed as input and produces the required Annotation for each
// Manila component
func ensureNAD(
	ctx context.Context,
	conditionUpdater conditionUpdater,
	nAttach []string,
	helper *helper.Helper,
) (map[string]string, ctrl.Result, error) {

	var serviceAnnotations map[string]string
	var err error
	// Iterate over the []networkattachment, get the corresponding NAD and create
	// the required annotation
	nadList := []networkv1.NetworkAttachmentDefinition{}
	for _, netAtt := range nAttach {
		nad, err := nad.GetNADWithName(ctx, helper, netAtt, helper.GetBeforeObject().GetNamespace())
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				log.FromContext(ctx).Info(fmt.Sprintf("network-attachment-definition %s not found", netAtt))
				conditionUpdater.Set(condition.FalseCondition(
					condition.NetworkAttachmentsReadyCondition,
					condition.RequestedReason,
					condition.SeverityInfo,
					condition.NetworkAttachmentsReadyWaitingMessage,
					netAtt))
				return serviceAnnotations, manila.ResultRequeue, nil
			}
			conditionUpdater.Set(condition.FalseCondition(
				condition.NetworkAttachmentsReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.NetworkAttachmentsReadyErrorMessage,
				err.Error()))
			return serviceAnnotations, manila.ResultRequeue, nil
		}

		if nad != nil {
			nadList = append(nadList, *nad)
		}
	}
	// Create NetworkAnnotations
	serviceAnnotations, err = nad.EnsureNetworksAnnotation(nadList)
	if err != nil {
		err := fmt.Errorf("failed create network annotation from %s: %w", nAttach, err)
		conditionUpdater.Set(condition.FalseCondition(
			condition.NetworkAttachmentsReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.NetworkAttachmentsReadyErrorMessage,
			err.Error()))
	}
	return serviceAnnotations, ctrl.Result{}, err
}

// verifyServiceSecret - ensures that the Secret object exists and the expected
// fields are in the Secret. It also sets a hash of the values of the expected
// fields passed as input.
func verifyServiceSecret(
	ctx context.Context,
	secretName types.NamespacedName,
	expectedFields []string,
	reader client.Reader,
	conditionUpdater conditionUpdater,
	requeueTimeout time.Duration,
	envVars *map[string]env.Setter,
) (ctrl.Result, error) {

	hash, res, err := secret.VerifySecret(ctx, secretName, expectedFields, reader, requeueTimeout)
	if err != nil {
		conditionUpdater.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return res, err
	} else if (res != ctrl.Result{}) {
		log.FromContext(ctx).Info(fmt.Sprintf("OpenStack secret %s not found", secretName))
		conditionUpdater.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.InputReadyWaitingMessage))
		return res, nil
	}
	(*envVars)[secretName.Name] = env.SetValue(hash)
	return ctrl.Result{}, nil
}

// verifyConfigSecrets - It iterates over the secretNames passed as input and
// sets the hash of values in the envVars map.
func verifyConfigSecrets(
	ctx context.Context,
	h *helper.Helper,
	conditionUpdater conditionUpdater,
	secretNames []string,
	namespace string,
	envVars *map[string]env.Setter,
) (ctrl.Result, error) {
	var hash string
	var err error
	for _, secretName := range secretNames {
		_, hash, err = secret.GetSecret(ctx, h, secretName, namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				log.FromContext(ctx).Info(fmt.Sprintf("Secret %s not found", secretName))
				conditionUpdater.Set(condition.FalseCondition(
					condition.InputReadyCondition,
					condition.RequestedReason,
					condition.SeverityInfo,
					condition.InputReadyWaitingMessage))
				return manila.ResultRequeue, nil
			}
			conditionUpdater.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.InputReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
		// Add a prefix to the var name to avoid accidental collision with other non-secret
		// vars. The secret names themselves will be unique.
		(*envVars)["secret-"+secretName] = env.SetValue(hash)
	}

	return ctrl.Result{}, nil
}
