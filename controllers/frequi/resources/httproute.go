package resources

import (
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	tradebotresources "github.com/ark-sys/freqtrade-operator/controllers/tradebot/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// managedByLabelValue and frequiRouteLabelKey stamp every HTTPRoute this
// operator generates, so pruneHTTPRoutes (G2-2) can select on them without
// touching routes anything else created. Deliberately not appLabelKey
// (that key selects Pods for the Deployment/Service, a different concept).
const (
	frequiRouteLabelKey  = "freqtrade.io/frequi"
	managedByLabelKey    = "app.kubernetes.io/managed-by"
	managedByLabelValue  = "freqtrade-operator"
	pathPrefixMatchValue = "/"
)

// BuildFreqUIHTTPRoutes returns the full desired set of HTTPRoutes for a
// Gateway-mode FreqUI: one for the UI itself, plus one per eligible
// TradeBot API route. It is one-per-hostname rather than one-object-many-
// hostnames because HTTPRoute.spec.hostnames is route-level and applies to
// every rule in the route - unlike Ingress, where each rule carries its own
// host. The returned slice is the complete desired state: anything else
// carrying this FreqUI's ownership label is garbage (see reconcileHTTPRoutes
// in controllers/frequi/main.go).
//
// skippedRoutes names any TradeBot API route whose generated HTTPRoute name
// would exceed the 253-char DNS-1123 subdomain limit - rather than emit an
// object the API server would refuse forever, that route is left out of the
// returned slice and reported here instead, mirroring how
// reconcileAllResources already reports unresolved TradeBotRefs.
func BuildFreqUIHTTPRoutes(
	frequi freqtradev1beta1.FreqUI, tradeBotAPIRoutes []TradeBotAPIRoute,
) (routes []gatewayv1.HTTPRoute, skippedRoutes []string) {
	spec := frequi.Spec
	mainHost := resolveMainHost(frequi)

	parentRefs := []gatewayv1.ParentReference{}
	labels := map[string]string{}
	annotations := map[string]string(nil)
	if spec.Gateway != nil {
		parentRefs = spec.Gateway.ParentRefs
		annotations = spec.Gateway.Annotations
		for k, v := range spec.Gateway.Labels {
			labels[k] = v
		}
	}
	// Operator labels win over any user-supplied spec.gateway.labels of the same key -
	// pruneHTTPRoutes depends on frequiRouteLabelKey actually naming this FreqUI.
	labels[frequiRouteLabelKey] = frequi.Name
	labels[managedByLabelKey] = managedByLabelValue

	uiHostnames := []gatewayv1.Hostname{gatewayv1.Hostname(mainHost)}
	if spec.Gateway != nil && len(spec.Gateway.Hostnames) > 0 {
		uiHostnames = spec.Gateway.Hostnames
	}

	routes = append(routes, buildHTTPRoute(frequi.Name, frequi.Namespace, uiHostnames, parentRefs, labels, annotations,
		frequi.Name, 80))

	for _, apiRoute := range tradeBotAPIRoutes {
		name := frequi.Name + "-" + apiRoute.Name
		if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
			skippedRoutes = append(skippedRoutes, apiRoute.Name)
			continue
		}
		apiHost := apiRoute.Name + "." + mainHost
		routes = append(routes, buildHTTPRoute(name, frequi.Namespace, []gatewayv1.Hostname{gatewayv1.Hostname(apiHost)},
			parentRefs, labels, annotations, apiRoute.ServiceName, tradebotresources.FreqtradeAPIPort))
	}

	return routes, skippedRoutes
}

func buildHTTPRoute(
	name, namespace string, hostnames []gatewayv1.Hostname, parentRefs []gatewayv1.ParentReference,
	labels, annotations map[string]string, backendServiceName string, backendPort int32,
) gatewayv1.HTTPRoute {
	pathType := gatewayv1.PathMatchPathPrefix
	pathValue := pathPrefixMatchValue
	port := gatewayv1.PortNumber(backendPort)

	return gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: parentRefs},
			Hostnames:       hostnames,
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{Path: &gatewayv1.HTTPPathMatch{Type: &pathType, Value: &pathValue}},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(backendServiceName),
									Port: &port,
								},
							},
						},
					},
				},
			},
		},
	}
}
