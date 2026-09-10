package k8s_locker

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	"github.com/openconfig/gnmic/pkg/lockers"
)

func receivePeers(t *testing.T, ch <-chan []*lockers.Service, want ...*lockers.Service) {
	t.Helper()
	if want == nil {
		want = []*lockers.Service{}
	}
	select {
	case got := <-ch:
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("snapshot = %#v; want %#v", got, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no peer snapshot received")
	}
}

func startDiscovery(t *testing.T, client *fake.Clientset) (chan []*lockers.Service, context.CancelFunc, chan error) {
	t.Helper()
	k := &k8sLocker{clientset: client, Cfg: &config{Namespace: testNamespace}}
	ctx, cancel := context.WithCancel(t.Context())
	changes := make(chan []*lockers.Service)
	done := make(chan error, 1)
	go func() { done <- k.WatchServices(ctx, testService, nil, changes, time.Second) }()
	t.Cleanup(cancel)
	return changes, cancel, done
}

func stoppedDiscovery(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("WatchServices() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("discovery did not stop")
	}
}

func TestWatchServicesSliceLifecycle(t *testing.T) {
	a := testSlice("a", testEndpoint("gnmic-0", "10.0.0.1"))
	b := testSlice("b", testEndpoint("gnmic-1", "10.0.0.2"))
	client := fake.NewClientset(a, b)
	changes, cancel, done := startDiscovery(t, client)
	receivePeers(t, changes, testPeer("gnmic-0", "10.0.0.1:7890"), testPeer("gnmic-1", "10.0.0.2:7890"))
	endpointSlices := client.DiscoveryV1().EndpointSlices(testNamespace)
	a = a.DeepCopy()
	a.Endpoints[0].Conditions.Ready = ptr.To(false)
	if _, err := endpointSlices.Update(t.Context(), a, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	receivePeers(t, changes, testPeer("gnmic-1", "10.0.0.2:7890"))
	c := testSlice("c", testEndpoint("gnmic-2", "10.0.0.3"))
	if _, err := endpointSlices.Create(t.Context(), c, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	receivePeers(t, changes, testPeer("gnmic-1", "10.0.0.2:7890"), testPeer("gnmic-2", "10.0.0.3:7890"))
	if err := endpointSlices.Delete(t.Context(), b.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	receivePeers(t, changes, testPeer("gnmic-2", "10.0.0.3:7890"))
	if err := endpointSlices.Delete(t.Context(), c.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	receivePeers(t, changes)
	cancel()
	stoppedDiscovery(t, done)
}

func TestWatchServicesInitiallyEmpty(t *testing.T) {
	client := fake.NewClientset()
	changes, cancel, done := startDiscovery(t, client)
	receivePeers(t, changes)
	if _, err := client.DiscoveryV1().EndpointSlices(testNamespace).Create(t.Context(), testSlice("a", testEndpoint("gnmic-0", "10.0.0.1")), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	receivePeers(t, changes, testPeer("gnmic-0", "10.0.0.1:7890"))
	cancel()
	stoppedDiscovery(t, done)
}

func TestWatchServicesRelistsExpiredVersion(t *testing.T) {
	a := testSlice("a", testEndpoint("gnmic-0", "10.0.0.1"))
	client := fake.NewClientset(a)
	watches := make(chan *watch.RaceFreeFakeWatcher, 10)
	client.PrependWatchReactor("endpointslices", func(action ktesting.Action) (bool, watch.Interface, error) {
		opts := action.(ktesting.WatchAction).GetWatchRestrictions()
		if opts.Labels.String() != serviceSelector(testService) {
			t.Errorf("watch selector = %s", opts.Labels)
		}
		w := watch.NewRaceFreeFake()
		watches <- w
		return true, w, nil
	})
	changes, cancel, done := startDiscovery(t, client)
	receivePeers(t, changes, testPeer("gnmic-0", "10.0.0.1:7890"))
	var first *watch.RaceFreeFakeWatcher
	select {
	case first = <-watches:
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not start")
	}
	a = a.DeepCopy()
	a.Endpoints = []discoveryv1.Endpoint{testEndpoint("gnmic-1", "10.0.0.2")}
	if err := client.Tracker().Update(discoveryv1.SchemeGroupVersion.WithResource("endpointslices"), a, testNamespace); err != nil {
		t.Fatal(err)
	}
	first.Error(&metav1.Status{Status: metav1.StatusFailure, Reason: metav1.StatusReasonExpired, Code: 410})
	first.Stop()
	receivePeers(t, changes, testPeer("gnmic-1", "10.0.0.2:7890"))
	var second *watch.RaceFreeFakeWatcher
	select {
	case second = <-watches:
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not restart")
	}
	second.Delete(a)
	receivePeers(t, changes)
	cancel()
	stoppedDiscovery(t, done)
}

func TestWatchServicesCancellation(t *testing.T) {
	for _, name := range []string{"blocked-publisher", "list-retry"} {
		t.Run(name, func(t *testing.T) {
			client := fake.NewClientset()
			attempted := make(chan struct{}, 1)
			client.PrependReactor("list", "endpointslices", func(ktesting.Action) (bool, runtime.Object, error) {
				select {
				case attempted <- struct{}{}:
				default:
				}
				if name == "list-retry" {
					return true, nil, errors.New("temporary list error")
				}
				return false, nil, nil
			})
			_, cancel, done := startDiscovery(t, client)
			select {
			case <-attempted:
			case <-time.After(5 * time.Second):
				t.Fatal("list did not start")
			}
			cancel()
			stoppedDiscovery(t, done)
		})
	}
}

func TestWatchServicesReconnectsWithoutDroppingPeers(t *testing.T) {
	a := testSlice("a", testEndpoint("gnmic-0", "10.0.0.1"))
	client := fake.NewClientset(a)
	watches := make(chan *watch.RaceFreeFakeWatcher, 10)
	client.PrependWatchReactor("endpointslices", func(ktesting.Action) (bool, watch.Interface, error) {
		w := watch.NewRaceFreeFake()
		watches <- w
		return true, w, nil
	})
	changes, cancel, done := startDiscovery(t, client)
	receivePeers(t, changes, testPeer("gnmic-0", "10.0.0.1:7890"))
	var first *watch.RaceFreeFakeWatcher
	select {
	case first = <-watches:
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not start")
	}
	first.Stop()
	var second *watch.RaceFreeFakeWatcher
	select {
	case second = <-watches:
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not reconnect")
	}
	a = a.DeepCopy()
	a.Endpoints[0].Addresses = []string{"10.0.0.2"}
	second.Modify(a)
	receivePeers(t, changes, testPeer("gnmic-0", "10.0.0.2:7890"))
	cancel()
	stoppedDiscovery(t, done)
}
