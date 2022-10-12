package manila

import "sigs.k8s.io/controller-runtime/pkg/client"

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
