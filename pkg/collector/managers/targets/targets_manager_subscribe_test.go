package targets_manager

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"google.golang.org/grpc"
)

// streamServer answers Capabilities and holds every Subscribe stream open
// after one update and a sync response, like a device streaming on change.
type streamServer struct {
	gnmi.UnimplementedGNMIServer
}

func (streamServer) Capabilities(context.Context, *gnmi.CapabilityRequest) (*gnmi.CapabilityResponse, error) {
	return &gnmi.CapabilityResponse{GNMIVersion: "0.10.0"}, nil
}

func (streamServer) Subscribe(stream gnmi.GNMI_SubscribeServer) error {
	if _, err := stream.Recv(); err != nil {
		return err
	}
	update := &gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
		Timestamp: time.Now().UnixNano(),
		Update: []*gnmi.Update{{
			Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
			Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 1}},
		}},
	}}}
	if err := stream.Send(update); err != nil {
		return err
	}
	if err := stream.Send(&gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true}}); err != nil {
		return err
	}
	<-stream.Context().Done()
	return nil
}

func serveStreamGNMI(t *testing.T, addr string) {
	t.Helper()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	s := grpc.NewServer()
	gnmi.RegisterGNMIServer(s, streamServer{})
	go func() { _ = s.Serve(l) }()
	t.Cleanup(s.Stop)
}

func streamSubscription(name string) *types.SubscriptionConfig {
	return &types.SubscriptionConfig{
		Name:       name,
		Paths:      []string{"/a"},
		Mode:       "stream",
		StreamMode: "on_change",
	}
}

// waitForSubscriptions waits until every named subscription of mt has an
// open Subscribe stream.
func waitForSubscriptions(t *testing.T, mt *ManagedTarget, names []string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	var states map[string]bool
	for time.Now().Before(deadline) {
		states = mt.T.SubscribeClientStates()
		open := 0
		for _, n := range names {
			if states[n] {
				open++
			}
		}
		if open == len(names) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("subscriptions with an open stream: %v, want all of %v", states, names)
}

// A subscription re-applied while its target is down must not stand in for
// the target's start: once the device answers, every subscription the target
// lists streams, not only the re-applied one.
func TestRetry_startsEverySubscriptionAfterOneIsAppliedWhileDown(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)
	subs := []string{"s1", "s2"}
	tm.mu.Lock()
	for _, n := range subs {
		tm.subscriptions[n] = streamSubscription(n)
	}
	tm.mu.Unlock()

	cfg := retryTestConfig("t1", addr)
	cfg.RetryTimer = 2 * time.Second
	cfg.Subscriptions = subs
	tm.apply("t1", cfg)
	if st := tm.getTargetStateStr("t1"); st != collstore.StateFailed {
		t.Fatalf("first start: state %q, want %q", st, collstore.StateFailed)
	}

	serveStreamGNMI(t, addr)
	tm.applySubscription("s1", *streamSubscription("s1"))

	mt := tm.Lookup("t1")
	waitForState(t, tm, "t1", collstore.StateRunning, 6*time.Second)
	waitForNoRetry(t, mt, 6*time.Second)
	waitForSubscriptions(t, mt, subs, 6*time.Second)
}
