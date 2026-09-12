package shared

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

// gatewayAPIGroupVersion is gateway.networking.k8s.io/v1 - the version this
// operator's HTTPRoute support (G2, GATEWAY-API-PLAN.md) targets.
const gatewayAPIGroupVersion = "gateway.networking.k8s.io/v1"

// gatewayAPIHTTPRouteResource is the plural resource name discovery reports
// for HTTPRoute - checked explicitly because the group can exist (e.g. a
// partial/older install) while this particular CRD does not.
const gatewayAPIHTTPRouteResource = "httproutes"

// GatewayAPIAvailable reports whether gateway.networking.k8s.io/v1
// HTTPRoute is served by this cluster (D9, G3-1: Gateway API is optional -
// most clusters don't have it installed, and this operator must keep
// reconciling Ingress-mode FreqUIs normally either way). Detection is a
// point-in-time discovery call, meant to be made once at startup
// (cmd/main.go) - installing Gateway API into the cluster afterwards
// requires an operator restart to be noticed.
//
// A discovery hiccup (RBAC denied, apiserver unreachable) is reported as an
// error so the caller can log it, but is not distinguished from "genuinely
// absent" in the returned bool - both mean HTTPRoute reconciliation must be
// skipped, and the caller must not fail startup either way: a transient
// discovery problem must not stop this operator from managing Ingress-mode
// FreqUIs.
func GatewayAPIAvailable(cfg *rest.Config) (bool, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return false, err
	}
	return gatewayAPIAvailable(dc)
}

// gatewayAPIAvailable is GatewayAPIAvailable's own logic, split out to take
// a discovery.DiscoveryInterface directly so tests can exercise it against
// a fake discovery client instead of a real apiserver.
func gatewayAPIAvailable(dc discovery.DiscoveryInterface) (bool, error) {
	resources, err := dc.ServerResourcesForGroupVersion(gatewayAPIGroupVersion)
	switch {
	case err == nil:
		// fall through to the resource-list check below
	case errors.IsNotFound(err), discovery.IsGroupDiscoveryFailedError(err):
		return false, nil
	default:
		return false, err
	}

	for _, r := range resources.APIResources {
		if r.Name == gatewayAPIHTTPRouteResource {
			return true, nil
		}
	}
	return false, nil
}
