package targets_manager

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"google.golang.org/grpc"
)

// streamServer answers Capabilities and holds every Subscribe stream open
// after one update and a sync response, like a device streaming on change.
// It counts the open streams by subscribed path.
//
// With holdCaps set, the server answers the first Capabilities call (the
// retry's probe) and holds the second (the one inside start()): it closes
// capsHeld, then answers with the error read from releaseCaps, or with
// capabilities if that error is nil. Later calls are answered at once.
type streamServer struct {
	gnmi.UnimplementedGNMIServer

	holdCaps    bool
	capsHeld    chan struct{}
	releaseCaps chan error

	mu        sync.Mutex
	capsCalls int
	open      map[string]int
}

func newStreamServer() *streamServer {
	return &streamServer{open: make(map[string]int)}
}

func newCapsHoldingServer() *streamServer {
	s := newStreamServer()
	s.holdCaps = true
	s.capsHeld = make(chan struct{})
	s.releaseCaps = make(chan error)
	return s
}

func (s *streamServer) Capabilities(ctx context.Context, _ *gnmi.CapabilityRequest) (*gnmi.CapabilityResponse, error) {
	s.mu.Lock()
	s.capsCalls++
	hold := s.holdCaps && s.capsCalls == 2
	s.mu.Unlock()
	if hold {
		close(s.capsHeld)
		select {
		case err := <-s.releaseCaps:
			if err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &gnmi.CapabilityResponse{GNMIVersion: "0.10.0"}, nil
}

func (s *streamServer) Subscribe(stream gnmi.GNMI_SubscribeServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	var paths []string
	for _, sub := range req.GetSubscribe().GetSubscription() {
		paths = append(paths, pathString(sub.GetPath()))
	}
	key := strings.Join(paths, ",")
	s.mu.Lock()
	s.open[key]++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.open[key]--
		s.mu.Unlock()
	}()

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

// openStreams returns the number of open Subscribe streams for each path
// that has at least one.
func (s *streamServer) openStreams() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int)
	for k, n := range s.open {
		if n > 0 {
			out[k] = n
		}
	}
	return out
}

func pathString(p *gnmi.Path) string {
	var b strings.Builder
	for _, e := range p.GetElem() {
		b.WriteString("/")
		b.WriteString(e.GetName())
	}
	return b.String()
}

func serveStreamGNMI(t *testing.T, addr string, srv *streamServer) {
	t.Helper()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	s := grpc.NewServer()
	gnmi.RegisterGNMIServer(s, srv)
	go func() { _ = s.Serve(l) }()
	t.Cleanup(s.Stop)
}

func streamSubscription(name, path string) *types.SubscriptionConfig {
	return &types.SubscriptionConfig{
		Name:       name,
		Paths:      []string{path},
		Mode:       "stream",
		StreamMode: "on_change",
	}
}

// waitForOpenStreams waits until the server holds exactly the wanted open
// streams per path, then checks that they stay so for settle.
func waitForOpenStreams(t *testing.T, srv *streamServer, want map[string]int, within, settle time.Duration) {
	t.Helper()
	equal := func(got map[string]int) bool {
		if len(got) != len(want) {
			return false
		}
		for k, n := range want {
			if got[k] != n {
				return false
			}
		}
		return true
	}
	deadline := time.Now().Add(within)
	for !equal(srv.openStreams()) {
		if time.Now().After(deadline) {
			t.Fatalf("open streams by path: %v, want %v", srv.openStreams(), want)
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(settle)
	if got := srv.openStreams(); !equal(got) {
		t.Fatalf("open streams by path after %s: %v, want %v", settle, got, want)
	}
}

func hasReader(mt *ManagedTarget, name string) bool {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	_, ok := mt.readersCfn[name]
	return ok
}

// startFailedTarget registers subscriptions s1 (/s1) and s2 (/s2) and target
// t1 listing both, at an address nothing listens on, so the first start
// fails and the target retries every retryTimer.
func startFailedTarget(t *testing.T, retryTimer time.Duration) (*TargetsManager, string) {
	t.Helper()
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)
	tm.mu.Lock()
	tm.subscriptions["s1"] = streamSubscription("s1", "/s1")
	tm.subscriptions["s2"] = streamSubscription("s2", "/s2")
	tm.mu.Unlock()

	cfg := retryTestConfig("t1", addr)
	cfg.RetryTimer = retryTimer
	cfg.Subscriptions = []string{"s1", "s2"}
	tm.apply("t1", cfg)
	if st := tm.getTargetStateStr("t1"); st != collstore.StateFailed {
		t.Fatalf("first start: state %q, want %q", st, collstore.StateFailed)
	}
	return tm, addr
}

// A subscription re-applied while its target is down must not stand in for
// the target's start: once the device answers, every subscription the target
// lists streams, s1 with its re-applied path.
func TestRetry_startsEverySubscriptionAfterOneIsAppliedWhileDown(t *testing.T) {
	// The target's retry waits the 2 s retry timer, while a subscription
	// started on its own makes its first attempt at once. Without the state
	// gate, s1's first response would mark the target Running well before
	// the retry runs, and the retry would end without starting s2.
	tm, addr := startFailedTarget(t, 2*time.Second)
	srv := newStreamServer()
	serveStreamGNMI(t, addr, srv)

	tm.applySubscription("s1", *streamSubscription("s1", "/s1b"))
	mt := tm.Lookup("t1")
	if st := tm.getTargetStateStr("t1"); st != collstore.StateFailed {
		t.Fatalf("after applying s1: state %q, want %q", st, collstore.StateFailed)
	}
	if hasReader(mt, "s1") {
		t.Fatal("after applying s1: s1 has a reader on a Failed target")
	}

	waitForState(t, tm, "t1", collstore.StateRunning, 6*time.Second)
	waitForNoRetry(t, mt, 6*time.Second)
	waitForOpenStreams(t, srv, map[string]int{"/s1b": 1, "/s2": 1}, 6*time.Second, 500*time.Millisecond)
}

// A subscription applied while the target's start() is in flight waits for
// that start: start() reads the stored config and starts it once, and a
// failed start leaves it to the next retry, with every other subscription.
func TestRetry_subscriptionAppliedWhileStarting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		release error
	}{
		{name: "start succeeds"},
		{name: "start fails", release: errors.New("capabilities refused")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tm, addr := startFailedTarget(t, 200*time.Millisecond)
			srv := newCapsHoldingServer()
			serveStreamGNMI(t, addr, srv)

			select {
			case <-srv.capsHeld:
			case <-time.After(6 * time.Second):
				t.Fatal("the retry never reached start()'s capabilities request")
			}
			if st := tm.getTargetStateStr("t1"); st != collstore.StateStarting {
				t.Fatalf("capabilities held: state %q, want %q", st, collstore.StateStarting)
			}
			tm.applySubscription("s1", *streamSubscription("s1", "/s1b"))
			mt := tm.Lookup("t1")
			if hasReader(mt, "s1") {
				t.Fatal("after applying s1: s1 has a reader on a Starting target")
			}
			srv.releaseCaps <- tc.release

			waitForState(t, tm, "t1", collstore.StateRunning, 6*time.Second)
			waitForNoRetry(t, mt, 6*time.Second)
			waitForOpenStreams(t, srv, map[string]int{"/s1b": 1, "/s2": 1}, 6*time.Second, 500*time.Millisecond)
		})
	}
}
