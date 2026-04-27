package manila

import (
	"math"

	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"
	"github.com/openstack-k8s-operators/lib-common/modules/common/probes"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetOwningManilaName - Given a ManilaAPI, ManilaScheduler, ManilaBackup or ManilaVolume
// object, returning the parent Manila object that created it (if any)
func GetOwningManilaName(instance client.Object) string {
	for _, ownerRef := range instance.GetOwnerReferences() {
		if ownerRef.Kind == "Manila" {
			return ownerRef.Name
		}
	}
	return ""
}

// manilaDefaultSecurityContext - Returns the right set of SecurityContext that
// does not violate the k8s requirements
func manilaDefaultSecurityContext() *corev1.SecurityContext {
	falseVal := false
	trueVal := true
	runAsUser := ManilaUserID
	runAsGroup := ManilaGroupID
	return &corev1.SecurityContext{
		RunAsUser:                &runAsUser,
		RunAsGroup:               &runAsGroup,
		RunAsNonRoot:             &trueVal,
		AllowPrivilegeEscalation: &falseVal,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// GetPodAffinity - Returns a corev1.Affinity reference for the specified component.
func GetPodAffinity(componentName string) *corev1.Affinity {
	// If possible two pods of the same component (e.g manila-share) should not
	// run on the same worker node. If this is not possible they get still
	// created on the same worker node.
	return affinity.DistributePods(
		common.ComponentSelector,
		[]string{
			componentName,
		},
		corev1.LabelHostname,
	)
}

// SharesListHash - Given a list of share names passed as an array of strings,
// it returns an hash that is currently used to build the job name
func SharesListHash(shareNames []string) (string, error) {
	hash, err := util.ObjectHash(shareNames)
	if err != nil {
		return hash, err
	}
	return hash, err
}

// GetDefaultProbesAPI returns default probe configuration for the ManilaAPI
// service. Probe periods are computed from the apiTimeout parameter so that
// Kubernetes detects an unresponsive API within the configured timeout window.
func GetDefaultProbesAPI(apiTimeout int) probes.OverrideSpec {
	const failureCount = 3
	period := int32(math.Floor(float64(apiTimeout) / float64(failureCount)))
	startupPeriod := int32(math.Max(5, float64(period)/2))

	return probes.OverrideSpec{
		LivenessProbes: &probes.ProbeConf{
			Path:                "/healthcheck",
			TimeoutSeconds:      10,
			PeriodSeconds:       period,
			InitialDelaySeconds: 5,
		},
		ReadinessProbes: &probes.ProbeConf{
			Path:                "/healthcheck",
			TimeoutSeconds:      10,
			PeriodSeconds:       period,
			InitialDelaySeconds: 5,
		},
		StartupProbes: &probes.ProbeConf{
			TimeoutSeconds:      10,
			PeriodSeconds:       startupPeriod,
			InitialDelaySeconds: 5,
			FailureThreshold:    12,
		},
	}
}

// GetDefaultProbesRPCWorker returns default probe configuration for RPC worker
// processes (manila-scheduler, manila-share). These processes expose no HTTP
// healthcheck endpoint, so Path is intentionally omitted. A dedicated
// serviceDownTime field is not currently exposed in the operator's API, and we
// rely on the default provided by manila to compute the default values.
// https://opendev.org/openstack/manila/src/branch/master/manila/common/config.py
func GetDefaultProbesRPCWorker(serviceDownTime int) probes.OverrideSpec {
	const failureCount = 3
	period := int32(math.Floor(float64(serviceDownTime) / float64(failureCount)))
	startupPeriod := int32(math.Max(5, float64(period)/2))

	return probes.OverrideSpec{
		LivenessProbes: &probes.ProbeConf{
			TimeoutSeconds:      10,
			PeriodSeconds:       period,
			InitialDelaySeconds: 15,
		},
		ReadinessProbes: &probes.ProbeConf{
			TimeoutSeconds:      10,
			PeriodSeconds:       period,
			InitialDelaySeconds: 15,
		},
		StartupProbes: &probes.ProbeConf{
			TimeoutSeconds:      10,
			PeriodSeconds:       startupPeriod,
			InitialDelaySeconds: period,
			FailureThreshold:    12,
		},
	}
}
