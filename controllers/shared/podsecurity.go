package shared

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

// RestrictedSecurityContext satisfies the pod-security.kubernetes.io/enforce=restricted
// Pod Security Standard (P3-3): verified empirically against the real
// freqtradeorg/freqtrade image (both `trade` and `download-data`) that
// freqtrade starts and runs cleanly under all of these at once, including
// no writable root filesystem - its own internal chown-user_data attempt
// fails (no privilege to escalate) and is already handled as a harmless
// warning, not a fatal error, in freqtrade's own startup code. Shared
// between controllers/tradebot and controllers/backtest (P6-1) - every
// freqtrade pod this operator builds, live bot or one-shot run, needs the
// identical property, and a second copy would risk drifting from it.
func RestrictedSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		RunAsNonRoot:             ptr.To(true),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

// DefaultContainerResources keeps a freqtrade container out of BestEffort
// QoS (first in line for OOM-kill) without being so rigid a default that it
// fights every real workload's actual needs - callers' own override
// mechanisms replace this entirely for anything heavier (e.g. FreqAI).
func DefaultContainerResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
}
