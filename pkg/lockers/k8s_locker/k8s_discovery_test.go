package k8s_locker

import (
	"context"
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	"github.com/openconfig/gnmic/pkg/lockers"
)

const testNamespace = "telemetry"
const testService = "test-gnmic-api"

func testSlice(name string, endpoints ...discoveryv1.Endpoint) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta:  metav1.ObjectMeta{Name: name, Namespace: testNamespace, Labels: map[string]string{discoveryv1.LabelServiceName: testService}},
		AddressType: discoveryv1.AddressTypeIPv4,
		Ports:       []discoveryv1.EndpointPort{{Port: ptr.To(int32(7890)), Protocol: ptr.To(corev1.ProtocolTCP)}},
		Endpoints:   endpoints,
	}
}

func testEndpoint(name, address string) discoveryv1.Endpoint {
	return discoveryv1.Endpoint{
		Addresses:  []string{address},
		TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: name},
		Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
	}
}

func testPeer(name, address string) *lockers.Service {
	return &lockers.Service{ID: name + "-api", Address: address, Tags: []string{"instance-name=" + name}}
}

func TestGetServicesEndpointSlices(t *testing.T) {
	a := testEndpoint("gnmic-0", "10.0.0.1")
	b := testEndpoint("gnmic-1", "10.0.0.2")
	unready := testEndpoint("unready", "10.0.0.3")
	unready.Conditions.Ready = ptr.To(false)
	notServing := testEndpoint("not-serving", "10.0.0.4")
	notServing.Conditions.Serving = ptr.To(false)
	terminating := testEndpoint("terminating", "10.0.0.5")
	terminating.Conditions.Terminating = ptr.To(true)
	unknown := testEndpoint("unknown", "10.0.0.6")
	unknown.Conditions = discoveryv1.EndpointConditions{}
	foreign := testSlice("foreign", testEndpoint("foreign", "10.0.1.1"))
	foreign.Labels[discoveryv1.LabelServiceName] = "another-service"
	otherNamespace := testSlice("other-namespace", testEndpoint("other-namespace", "10.0.1.2"))
	otherNamespace.Namespace = "other"
	tests := []struct {
		name    string
		objects []runtime.Object
		want    []*lockers.Service
	}{
		{name: "absent", want: []*lockers.Service{}},
		{name: "empty", objects: []runtime.Object{testSlice("empty")}, want: []*lockers.Service{}},
		{name: "single", objects: []runtime.Object{testSlice("one", a)}, want: []*lockers.Service{testPeer("gnmic-0", "10.0.0.1:7890")}},
		{name: "multi-slice", objects: []runtime.Object{testSlice("two", b, a), testSlice("one", a), foreign, otherNamespace},
			want: []*lockers.Service{testPeer("gnmic-0", "10.0.0.1:7890"), testPeer("gnmic-1", "10.0.0.2:7890")}},
		{name: "conditions", objects: []runtime.Object{testSlice("conditions", unready, notServing, terminating, unknown)},
			want: []*lockers.Service{testPeer("unknown", "10.0.0.6:7890")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(tt.objects...)
			k := &k8sLocker{clientset: client, Cfg: &config{Namespace: testNamespace}}
			got, err := k.GetServices(t.Context(), testService, nil)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("GetServices() = %#v, %v; want %#v", got, err, tt.want)
			}
			for _, action := range client.Actions() {
				if action.GetResource().Group != discoveryv1.GroupName || action.GetResource().Resource != "endpointslices" {
					t.Fatalf("unexpected API action: %#v", action)
				}
			}
		})
	}
}

func TestEndpointSliceAddressesAndPorts(t *testing.T) {
	ipv6 := testSlice("ipv6", testEndpoint("gnmic-0", "2001:db8::1"), testEndpoint("gnmic-1", "2001:db8::2"))
	ipv6.AddressType = discoveryv1.AddressTypeIPv6
	ipv4 := testSlice("ipv4", testEndpoint("gnmic-0", "10.0.0.1"))
	ipv4.Ports = []discoveryv1.EndpointPort{
		{Port: ptr.To(int32(53)), Protocol: ptr.To(corev1.ProtocolUDP)},
		{Port: ptr.To(int32(7890))},
	}
	missingPort := testSlice("missing-port", testEndpoint("missing-port", "10.0.0.3"))
	missingPort.Ports[0].Port = nil
	missingRef := testSlice("missing-ref", discoveryv1.Endpoint{Addresses: []string{"10.0.0.4", ""}})
	want := []*lockers.Service{testPeer("10.0.0.4", "10.0.0.4:7890"), testPeer("gnmic-0", "10.0.0.1:7890"), testPeer("gnmic-1", "[2001:db8::2]:7890")}
	for _, slices := range [][]*discoveryv1.EndpointSlice{{ipv6, ipv4, missingPort, missingRef}, {missingRef, missingPort, ipv4, ipv6}} {
		if got := endpointSliceServices(slices); !reflect.DeepEqual(got, want) {
			t.Fatalf("services = %#v; want %#v", got, want)
		}
	}
}

func TestGetServicesListError(t *testing.T) {
	client := fake.NewClientset()
	expected := errors.New("list unavailable")
	client.PrependReactor("list", "endpointslices", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, expected })
	k := &k8sLocker{clientset: client, Cfg: &config{Namespace: testNamespace}}
	if _, err := k.GetServices(context.Background(), testService, nil); !errors.Is(err, expected) {
		t.Fatalf("error = %v; want %v", err, expected)
	}
}
