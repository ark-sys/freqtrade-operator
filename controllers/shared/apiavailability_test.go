package shared

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newFakeDiscovery(resources ...*metav1.APIResourceList) *fakediscovery.FakeDiscovery {
	return &fakediscovery.FakeDiscovery{Fake: &k8stesting.Fake{Resources: resources}}
}

func TestGatewayAPIAvailable_GroupAndResourcePresent(t *testing.T) {
	dc := newFakeDiscovery(&metav1.APIResourceList{
		GroupVersion: gatewayAPIGroupVersion,
		APIResources: []metav1.APIResource{{Name: gatewayAPIHTTPRouteResource}},
	})

	available, err := gatewayAPIAvailable(dc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !available {
		t.Error("expected available=true when the group and HTTPRoute resource are both present")
	}
}

func TestGatewayAPIAvailable_GroupAbsent(t *testing.T) {
	// No matching GroupVersion in Resources - FakeDiscovery.ServerResourcesForGroupVersion
	// falls through to a NotFound-shaped StatusError, mirroring a real apiserver's response
	// for an unregistered group. That must be reported as "absent, no error".
	dc := newFakeDiscovery()

	available, err := gatewayAPIAvailable(dc)
	if err != nil {
		t.Fatalf("expected a missing group to be reported as absent, not an error: %v", err)
	}
	if available {
		t.Error("expected available=false when the group is absent")
	}
}

func TestGatewayAPIAvailable_GroupPresentResourceMissing(t *testing.T) {
	// The group exists (e.g. a partial/older Gateway API install) but HTTPRoute itself isn't
	// in the resource list - must not be conflated with "the group is fully installed".
	dc := newFakeDiscovery(&metav1.APIResourceList{
		GroupVersion: gatewayAPIGroupVersion,
		APIResources: []metav1.APIResource{{Name: "gateways"}},
	})

	available, err := gatewayAPIAvailable(dc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if available {
		t.Error("expected available=false when the group exists but HTTPRoute doesn't")
	}
}

func TestGatewayAPIAvailable_TransportError(t *testing.T) {
	dc := newFakeDiscovery()
	dc.PrependReactor("get", "resource", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})

	available, err := gatewayAPIAvailable(dc)
	if err == nil {
		t.Fatal("expected a transport error to be returned, not swallowed")
	}
	if available {
		t.Error("expected available=false alongside a transport error")
	}
}
