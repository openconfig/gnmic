package targets_manager

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/collector/managers/testutil"
	"github.com/openconfig/grpctunnel/tunnel"
)

func TestTunnelServer_startAndRegisterTarget(t *testing.T) {
	st := testutil.NewTestStore(t)
	sock := filepath.Join(t.TempDir(), "tunnel.sock")

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
