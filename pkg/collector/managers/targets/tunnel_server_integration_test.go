package targets_manager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/collector/managers/testutil"
	"github.com/openconfig/grpctunnel/tunnel"
)

// startTunnelServer retries a failing listener every second. That retry must
// stay cancellable: a permanent bind failure previously spun forever and the
// goroutine could not be stopped, hanging shutdown and config reload.
func TestTunnelServer_startReturnsOnCancelWhenListenFails(t *testing.T) {
	st := testutil.NewTestStore(t)

	// a path that can never bind
	if _, err := st.Config.Set("tunnel-server", "tunnel-server", &config.TunnelServer{
		Address: "unix:///nonexistent-dir/tunnel.sock",
	}); err != nil {
		t.Fatalf("seed tunnel-server: %v", err)
	}

	ts := newTunnelServer(st.Config, nil)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- ts.startTunnelServer(ctx) }()

	// let the retry loop iterate at least once, then cancel
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("startTunnelServer did not return after cancellation")
	}
}

func TestTunnelServer_startAndRegisterTarget(t *testing.T) {
	st := testutil.NewTestStore(t)

	// Not t.TempDir(): it embeds the test name, and the resulting path exceeds
	// the sockaddr_un limit (104 bytes on darwin, 108 on linux), so the bind
	// fails with "invalid argument".
	sockDir, err := os.MkdirTemp("", "gnmic")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	sock := filepath.Join(sockDir, "tunnel.sock")

	if _, err := st.Config.Set("tunnel-server", "tunnel-server", &config.TunnelServer{
		Address: "unix://" + sock,
	}); err != nil {
		t.Fatalf("seed tunnel-server: %v", err)
	}

	match := &config.TunnelTargetMatch{
		ID:   "srl1",
		Type: "GNMI_GNOI",
		Config: types.TargetConfig{
			Subscriptions: []string{"ifaces"},
		},
	}
	if _, err := st.Config.Set("tunnel-target-matches", "policy", match); err != nil {
		t.Fatalf("seed match: %v", err)
	}
	seedSubscription(t, st, "ifaces")

	ts := newTunnelServer(st.Config, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ts.startTunnelServer(ctx)
	}()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ts.tunServer != nil && ts.grpcTunnelSrv != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ts.tunServer == nil || ts.grpcTunnelSrv == nil {
		t.Fatal("tunnel gRPC server did not start")
	}

	if err := ts.addTargetHandler(tunnel.Target{ID: "srl1", Type: "GNMI_GNOI"}); err != nil {
		t.Fatalf("addTargetHandler: %v", err)
	}

	got, ok, err := st.Config.Get("targets", "srl1")
	if err != nil {
		t.Fatalf("get target: %v", err)
	}
	if !ok {
		t.Fatal("expected tunnel target in store")
	}
	cfg, ok := got.(*types.TargetConfig)
	if !ok || cfg.TunnelTargetType != "GNMI_GNOI" {
		t.Fatalf("target config: %#v", got)
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("tunnel server did not shut down")
	}
}
