package targets_manager

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
)

// capsOnlyServer answers Capabilities, which is all a target's start needs.
type capsOnlyServer struct {
	gnmi.UnimplementedGNMIServer
}

func (capsOnlyServer) Capabilities(context.Context, *gnmi.CapabilityRequest) (*gnmi.CapabilityResponse, error) {
	return &gnmi.CapabilityResponse{GNMIVersion: "0.10.0"}, nil
}

// freeAddr reserves a local port and releases it, so the first dial is refused.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func serveGNMI(t *testing.T, addr string) {
	t.Helper()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	s := grpc.NewServer()
	gnmi.RegisterGNMIServer(s, capsOnlyServer{})
	go func() { _ = s.Serve(l) }()
	t.Cleanup(s.Stop)
}

func retryTestConfig(name, addr string) *types.TargetConfig {
	insecure := true
	return &types.TargetConfig{
		Name:       name,
		Address:    addr,
		Insecure:   &insecure,
		Timeout:    time.Second,
		RetryTimer: 100 * time.Millisecond,
	}
}

func waitForState(t *testing.T, tm *TargetsManager, name, want string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if tm.getTargetStateStr(name) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("target %s: state %q, want %q after %s", name, tm.getTargetStateStr(name), want, within)
}

// waitForNoRetry waits for the retry loop to clear mt.retryCancel: the loop
// clears it right after start() returns, a moment after Running is published.
func waitForNoRetry(t *testing.T, mt *ManagedTarget, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		mt.RLock()
		pending := mt.retryCancel != nil
		mt.RUnlock()
		if !pending {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("retry still scheduled after the target started")
}

// silentListener accepts TCP connections and never answers, like a device
// whose gNMI server has not started yet.
func silentListener(t *testing.T, addr string) {
	t.Helper()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = c.Close() })
		}
	}()
}

func TestApply_retriesTargetWhoseFirstStartFailed(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)

	// Nothing listens yet: the first start fails, as against a switch still booting.
	tm.apply("t1", retryTestConfig("t1", addr))
	if st := tm.getTargetStateStr("t1"); st != collstore.StateFailed {
		t.Fatalf("first start: state %q, want %q", st, collstore.StateFailed)
	}

	// The device comes up. No new config is applied: the retry alone must start it.
	serveGNMI(t, addr)
	waitForState(t, tm, "t1", collstore.StateRunning, 5*time.Second)
	waitForNoRetry(t, tm.Lookup("t1"), time.Second)
}

func TestRemove_stopsRetry(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)

	tm.apply("t1", retryTestConfig("t1", addr))
	if st := tm.getTargetStateStr("t1"); st != collstore.StateFailed {
		t.Fatalf("first start: state %q, want %q", st, collstore.StateFailed)
	}
	tm.remove("t1")

	// A device appearing after removal must not resurrect the target.
	serveGNMI(t, addr)
	time.Sleep(500 * time.Millisecond)
	if tm.Lookup("t1") != nil {
		t.Fatal("removed target came back")
	}
	if st := tm.getTargetStateStr("t1"); st != "" {
		t.Fatalf("removed target has state %q", st)
	}
}

func TestApply_noRetryWhenStartSucceeds(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)
	serveGNMI(t, addr)

	tm.apply("t1", retryTestConfig("t1", addr))
	waitForState(t, tm, "t1", collstore.StateRunning, 5*time.Second)

	mt := tm.Lookup("t1")
	mt.RLock()
	pending := mt.retryCancel != nil
	mt.RUnlock()
	if pending {
		t.Fatal("a retry was scheduled for a target that started")
	}
}

func TestSetIntendedStateDisabled_stopsRetry(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)

	tm.apply("t1", retryTestConfig("t1", addr))
	if st := tm.getTargetStateStr("t1"); st != collstore.StateFailed {
		t.Fatalf("first start: state %q, want %q", st, collstore.StateFailed)
	}
	tm.SetIntendedState("t1", collstore.IntendedStateDisabled)

	// Checked before the retry's first tick (100ms), so only the cancel
	// itself can have cleared it.
	mt := tm.Lookup("t1")
	mt.RLock()
	pending := mt.retryCancel != nil
	mt.RUnlock()
	if pending {
		t.Fatal("retry still scheduled after the target was disabled")
	}

	// A disabled target must stay down when its device appears.
	serveGNMI(t, addr)
	time.Sleep(500 * time.Millisecond)
	if st := tm.getTargetStateStr("t1"); st == collstore.StateRunning {
		t.Fatalf("disabled target was started by a retry")
	}
}

func TestSetIntendedStateEnabled_retriesWhenStartFails(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)

	tm.apply("t1", retryTestConfig("t1", addr))
	tm.SetIntendedState("t1", collstore.IntendedStateDisabled)
	// Re-enabled while the device is still down: this start fails too.
	tm.SetIntendedState("t1", collstore.IntendedStateEnabled)

	serveGNMI(t, addr)
	waitForState(t, tm, "t1", collstore.StateRunning, 5*time.Second)
}

func TestRetry_staysFailedWhileDeviceDown(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)

	tm.apply("t1", retryTestConfig("t1", addr))
	// Several retries run in this window; none may publish another state.
	deadline := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(deadline) {
		ts := tm.GetTargetState("t1")
		if ts.State != collstore.StateFailed {
			t.Fatalf("state %q while the device is down, want %q", ts.State, collstore.StateFailed)
		}
		if ts.IntendedState != collstore.IntendedStateEnabled {
			t.Fatalf("intended state %q while retrying, want %q", ts.IntendedState, collstore.IntendedStateEnabled)
		}
		if ts.FailedReason == "" {
			t.Fatal("failed reason cleared while retrying")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRetry_probeHoldsNoTargetLock(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)
	cfg := retryTestConfig("t1", addr)
	cfg.Timeout = 2 * time.Second

	tm.apply("t1", cfg)
	if st := tm.getTargetStateStr("t1"); st != collstore.StateFailed {
		t.Fatalf("first start: state %q, want %q", st, collstore.StateFailed)
	}
	// The device now accepts TCP but does not answer: a probe waits up to its
	// timeout. remove() must neither wait for it nor bring the target back.
	silentListener(t, addr)
	time.Sleep(300 * time.Millisecond) // a probe is in flight

	done := make(chan struct{})
	go func() {
		tm.remove("t1")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("remove() blocked behind a retry probe")
	}
	if tm.Lookup("t1") != nil {
		t.Fatal("removed target came back")
	}
}

func TestRetry_cancelledLoopKeepsNewerRetry(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)

	tm.apply("t1", retryTestConfig("t1", addr))
	mt := tm.Lookup("t1")
	// Disable and re-enable while the device is down: the first loop is
	// cancelled and the failed re-enable schedules a second one.
	tm.SetIntendedState("t1", collstore.IntendedStateDisabled)
	tm.SetIntendedState("t1", collstore.IntendedStateEnabled)
	time.Sleep(300 * time.Millisecond) // the cancelled loop has woken and exited

	mt.RLock()
	pending := mt.retryCancel != nil
	mt.RUnlock()
	if !pending {
		t.Fatal("the cancelled loop cleared the newer loop's retry")
	}
	serveGNMI(t, addr)
	waitForState(t, tm, "t1", collstore.StateRunning, 5*time.Second)
	waitForNoRetry(t, mt, time.Second)
}

func TestRetry_countsAttempts(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)

	tm.apply("t1", retryTestConfig("t1", addr))
	time.Sleep(450 * time.Millisecond) // retry-timer 100ms: several attempts
	if n := testutil.ToFloat64(tm.stats.targetStartRetries.WithLabelValues("t1")); n < 2 {
		t.Fatalf("start retries counted %v while the device is down, want at least 2", n)
	}
	tm.remove("t1")
	if n := testutil.CollectAndCount(tm.stats.targetStartRetries); n != 0 {
		t.Fatalf("%d start-retry series left after remove", n)
	}
}

// The sequence from review: a loop wakes and waits for mt.Lock while a
// disable holds it and cancels the loop; the re-enable's failed start then
// installs a newer loop. When the old loop gets the lock it must leave the
// newer loop's retryCancel alone and must not start the target.
func TestRetry_cancelledLoopLeavesNewerRetry(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)

	tm.apply("t1", retryTestConfig("t1", addr))
	mt := tm.Lookup("t1")
	mt.Lock()
	mt.cancelRetry() // stop the loop apply scheduled; this test runs its own
	mt.Unlock()
	serveGNMI(t, addr) // the device is up: an attempt would succeed

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		tm.retryLoop(ctx, mt)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)  // the loop has read its delay and sleeps
	mt.Lock()                          // the disable
	time.Sleep(300 * time.Millisecond) // the loop has woken and waits for mt
	cancel()
	newer := func() {}
	mt.retryCancel = newer // the re-enable's failed start schedules a new loop
	mt.Unlock()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the cancelled loop did not exit")
	}
	mt.RLock()
	kept := mt.retryCancel != nil
	mt.RUnlock()
	if !kept {
		t.Fatal("the cancelled loop cleared the newer loop's retryCancel")
	}
	if st := tm.getTargetStateStr("t1"); st != collstore.StateFailed {
		t.Fatalf("the cancelled loop changed the target to %q", st)
	}
}
