// controllers/backtest/resources/rbac.go
package resources

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SidecarServiceAccountName is the P6-2 results-collection sidecar's
// ServiceAccount - one per namespace, shared by every Backtest's Job in
// that namespace, not one per run. Fixed rather than derived from any
// single Backtest's name specifically so it can be applied unowned
// (see shared.ApplyUnowned) without any one Backtest's deletion tearing it
// out from under every other Backtest still using it.
const SidecarServiceAccountName = "freqtrade-backtest-sidecar"

// ResultsConfigMapName is where the sidecar writes what it extracted from
// a run's own result file, and where the operator's own reconcile loop
// reads it back from - the one name both sides need to agree on.
func ResultsConfigMapName(backtestName string) string {
	return backtestName + "-results"
}

// BuildSidecarServiceAccount creates the ServiceAccount the results
// sidecar runs as - deliberately not the manager's own ServiceAccount
// (P1-2): the sidecar runs inside a Job pod a Backtest's own spec.pod
// overrides can otherwise influence, so it gets the narrowest possible
// permissions of its own rather than inheriting the manager's.
func BuildSidecarServiceAccount(namespace string) corev1.ServiceAccount {
	return corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: SidecarServiceAccountName, Namespace: namespace},
	}
}

// BuildSidecarRole grants exactly what the sidecar needs: create/update its
// own results ConfigMap, get to check whether one it's about to create
// already exists (a retry after a partial failure must not error on
// AlreadyExists), and get on pods - collectresults.waitForMainContainerExit
// polls the sidecar's own Pod's containerStatuses to learn when the main
// freqtrade container has exited, which is how a native sidecar (no exit
// hook of its own) knows the run is over and it's time to read the result
// file. Not scoped to specific resourceNames for either resource: this Role
// is shared across every Backtest that will ever run in the namespace, so a
// fixed resourceNames list naming only today's Backtests/pods would reject
// tomorrow's. Found and fixed directly: a real run's sidecar crash-looped
// forever on "cannot get resource \"pods\"" before this was added - nothing
// in this project's envtest/unit coverage exercises the sidecar's actual
// ServiceAccount identity making a real API call, only its logic.
func BuildSidecarRole(namespace string) rbacv1.Role {
	return rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: SidecarServiceAccountName, Namespace: namespace},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "create", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			},
		},
	}
}

// BuildSidecarRoleBinding binds BuildSidecarRole to BuildSidecarServiceAccount.
func BuildSidecarRoleBinding(namespace string) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: SidecarServiceAccountName, Namespace: namespace},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName, Kind: "Role", Name: SidecarServiceAccountName,
		},
		Subjects: []rbacv1.Subject{{
			Kind: rbacv1.ServiceAccountKind, Name: SidecarServiceAccountName, Namespace: namespace,
		}},
	}
}
