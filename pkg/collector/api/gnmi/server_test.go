// SPDX-License-Identifier: Apache-2.0

package gnmiserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/cache"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
	"github.com/openconfig/gnmic/pkg/collector/managers/testutil"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/config"
	handlers "github.com/openconfig/gnmic/pkg/gnmiserver"
	"github.com/openconfig/gnmic/pkg/pipeline"
)

func newTestServer(t *testing.T, ctx context.Context) (*Server, *collstore.Store) {
	t.Helper()
	st := testutil.NewTestStore(t)
	reg := prometheus.NewRegistry()
	tm := targets_manager.NewTargetsManager(ctx, st, make(chan *pipeline.Msg, 16), reg)
	s := NewServer(ctx, st, tm, reg)
	s.logger = slog.New(slog.DiscardHandler)
	s.cfg = &config.GNMIServer{}
	s.cfg.SetDefaults()
	return s, st
}

func seedTarget(t *testing.T, st *collstore.Store, name, address string) {
	t.Helper()
	_, err := st.Config.Set("targets", name, &types.TargetConfig{
		Name:     name,
		Address:  address,
		Insecure: pointer(true),
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}
}

func pointer[T any](v T) *T { return &v }

func TestStart_NoConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, _ := newTestServer(t, ctx)
	wg := new(sync.WaitGroup)
	if err := s.Start(nil, wg); err != nil {
		t.Fatalf("Start() with no gnmi-server config: %v", err)
	}
	if s.cancel != nil {
		t.Fatal("server started without gnmi-server config")
	}
	s.Stop() // must not panic
	wg.Wait()
}

func TestStart_TypedNilConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, st := newTestServer(t, ctx)
	var nilCfg *config.GNMIServer
	if _, err := st.Config.Set("gnmi-server", "gnmi-server", nilCfg); err != nil {
		t.Fatalf("seed gnmi-server: %v", err)
	}
	wg := new(sync.WaitGroup)
	if err := s.Start(nil, wg); err != nil {
		t.Fatalf("Start() with typed nil gnmi-server config: %v", err)
	}
	if s.cancel != nil {
		t.Fatal("server started with a nil gnmi-server config")
	}
	wg.Wait()
}

func TestSelectTargets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, st := newTestServer(t, ctx)
	seedTarget(t, st, "router1:57400", "127.0.0.1:1")
	seedTarget(t, st, "router2", "127.0.0.1:2")

	tests := []struct {
		name string
		tn   string
		want []string
	}{
		{name: "all", tn: "*", want: []string{"router1:57400", "router2"}},
		{name: "empty", tn: "", want: []string{"router1:57400", "router2"}},
		{name: "by name", tn: "router2", want: []string{"router2"}},
		{name: "by host", tn: "router1", want: []string{"router1:57400"}},
		{name: "list", tn: "router1,router2", want: []string{"router1:57400", "router2"}},
		{name: "unknown", tn: "nope", want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets, cleanup, err := s.selectTargets(ctx, tt.tn)
			if err != nil {
				t.Fatalf("selectTargets(%q): %v", tt.tn, err)
			}
			defer cleanup()
			if len(targets) != len(tt.want) {
				t.Fatalf("selectTargets(%q) returned %d targets, want %d: %v", tt.tn, len(targets), len(tt.want), targets)
			}
			for _, name := range tt.want {
				if _, ok := targets[name]; !ok {
					t.Errorf("selectTargets(%q) missing target %q", tt.tn, name)
				}
			}
		})
	}
}

func TestSelectTargets_TunnelTargetNotConnected(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, st := newTestServer(t, ctx)
	if _, err := st.Config.Set("targets", "tun1", &types.TargetConfig{
		Name:             "tun1",
		Address:          "tun1",
		TunnelTargetType: "GNMI_GNOI",
		Timeout:          time.Second,
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	_, cleanup, err := s.selectTargets(ctx, "tun1")
	if err == nil {
		cleanup()
		t.Fatal("expected an error selecting a non connected tunnel target")
	}
}

func TestInternalGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, st := newTestServer(t, ctx)
	seedTarget(t, st, "router1", "10.0.0.1:57400")
	seedTarget(t, st, "router2", "10.0.0.2:57400")
	// seeded without the Name field set, like configs loaded from file:
	// the store key must be used as the subscription name.
	if _, err := st.Config.Set("subscriptions", "sub1", &types.SubscriptionConfig{
		Paths: []string{"/interfaces"},
	}); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	// route through the shared handlers to exercise
	// the `gnmic` origin dispatch as well.
	h := handlers.New(handlers.Config{}, nil, nil, s.selectTargets, s.handleInternalGet)

	// all targets
	rsp, err := h.Get(ctx, &gnmi.GetRequest{
		Path: []*gnmi.Path{
			{Origin: "gnmic", Elem: []*gnmi.PathElem{{Name: "targets"}}},
		},
		Encoding: gnmi.Encoding_JSON,
	})
	if err != nil {
		t.Fatalf("internal Get targets: %v", err)
	}
	if len(rsp.GetNotification()) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(rsp.GetNotification()))
	}
	tc := new(types.TargetConfig)
	err = json.Unmarshal(rsp.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonVal(), tc)
	if err != nil {
		t.Fatalf("failed to unmarshal target config: %v", err)
	}
	if tc.Name != "router1" {
		t.Errorf("expected first target to be router1, got %q", tc.Name)
	}

	// single target by key
	rsp, err = h.Get(ctx, &gnmi.GetRequest{
		Path: []*gnmi.Path{
			{Origin: "gnmic", Elem: []*gnmi.PathElem{{Name: "targets", Key: map[string]string{"name": "router2"}}}},
		},
		Encoding: gnmi.Encoding_JSON,
	})
	if err != nil {
		t.Fatalf("internal Get target by name: %v", err)
	}
	if len(rsp.GetNotification()) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(rsp.GetNotification()))
	}

	// subscriptions
	rsp, err = h.Get(ctx, &gnmi.GetRequest{
		Path: []*gnmi.Path{
			{Origin: "gnmic", Elem: []*gnmi.PathElem{{Name: "subscriptions"}}},
		},
		Encoding: gnmi.Encoding_JSON,
	})
	if err != nil {
		t.Fatalf("internal Get subscriptions: %v", err)
	}
	if len(rsp.GetNotification()) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(rsp.GetNotification()))
	}
	subName := rsp.GetNotification()[0].GetUpdate()[0].GetPath().GetElem()[0].GetKey()["name"]
	if subName != "sub1" {
		t.Errorf("expected subscription name sub1, got %q", subName)
	}

	// unknown path element
	_, err = h.Get(ctx, &gnmi.GetRequest{
		Path: []*gnmi.Path{
			{Origin: "gnmic", Elem: []*gnmi.PathElem{{Name: "outputs"}}},
		},
		Encoding: gnmi.Encoding_JSON,
	})
	if err == nil {
		t.Fatal("expected an error for unknown gnmic origin path")
	}

	// subscriptions only support JSON/JSON_IETF encodings,
	// other encodings are rejected with an Unimplemented status.
	_, err = h.Get(ctx, &gnmi.GetRequest{
		Path: []*gnmi.Path{
			{Origin: "gnmic", Elem: []*gnmi.PathElem{{Name: "subscriptions"}}},
		},
		Encoding: gnmi.Encoding_ASCII,
	})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("expected Unimplemented for ASCII encoded subscriptions Get, got: %v", err)
	}
}

// TestServer_EndToEnd starts the gNMI server on a random port,
// writes a notification into the cache and verifies that:
//   - a ONCE subscription returns the notification and a sync response.
//   - a Get request with origin gnmic returns the configured targets.
func TestServer_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, st := newTestServer(t, ctx)
	seedTarget(t, st, "router1", "10.0.0.1:57400")

	addr := freeAddr(t)
	if _, err := st.Config.Set("gnmi-server", "gnmi-server", &config.GNMIServer{
		Address: addr,
	}); err != nil {
		t.Fatalf("seed gnmi-server config: %v", err)
	}

	c, err := cache.New(nil)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer c.Stop()
	c.Write(ctx, "sub1", &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Timestamp: time.Now().UnixNano(),
				Prefix:    &gnmi.Path{Target: "router1"},
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{
							Elem: []*gnmi.PathElem{
								{Name: "system"},
								{Name: "name"},
								{Name: "host-name"},
							},
						},
						Val: &gnmi.TypedValue{
							Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "router1"},
						},
					},
				},
			},
		},
	})

	wg := new(sync.WaitGroup)
	if err := s.Start(c, wg); err != nil {
		t.Fatalf("failed to start gNMI server: %v", err)
	}
	defer func() {
		s.Stop()
		wg.Wait()
	}()

	conn := dialWithRetry(t, ctx, addr)
	defer conn.Close()
	client := gnmi.NewGNMIClient(conn)

	// Subscribe ONCE
	subCtx, subCancel := context.WithTimeout(ctx, 10*time.Second)
	defer subCancel()
	stream, err := client.Subscribe(subCtx)
	if err != nil {
		t.Fatalf("failed to create subscribe stream: %v", err)
	}
	err = stream.Send(&gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{
				Prefix: &gnmi.Path{Target: "router1"},
				Mode:   gnmi.SubscriptionList_ONCE,
				Subscription: []*gnmi.Subscription{
					{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}}}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to send subscribe request: %v", err)
	}
	var updates []*gnmi.Notification
	sawSync := false
	for {
		rsp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("subscribe stream receive error: %v", err)
		}
		switch r := rsp.GetResponse().(type) {
		case *gnmi.SubscribeResponse_Update:
			updates = append(updates, r.Update)
		case *gnmi.SubscribeResponse_SyncResponse:
			sawSync = true
		}
		if sawSync {
			break
		}
	}
	if !sawSync {
		t.Fatal("did not receive a sync response")
	}
	if len(updates) == 0 {
		t.Fatal("did not receive any update from the cache")
	}
	if tgt := updates[0].GetPrefix().GetTarget(); tgt != "router1" {
		t.Errorf("expected update for target router1, got %q", tgt)
	}

	// Get with origin gnmic
	getCtx, getCancel := context.WithTimeout(ctx, 10*time.Second)
	defer getCancel()
	rsp, err := client.Get(getCtx, &gnmi.GetRequest{
		Path: []*gnmi.Path{
			{Origin: "gnmic", Elem: []*gnmi.PathElem{{Name: "targets"}}},
		},
		Encoding: gnmi.Encoding_JSON,
	})
	if err != nil {
		t.Fatalf("gnmic origin Get failed: %v", err)
	}
	if len(rsp.GetNotification()) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(rsp.GetNotification()))
	}

	// Get served from the cache
	cacheRsp, err := client.Get(getCtx, &gnmi.GetRequest{
		Prefix: &gnmi.Path{Target: "router1"},
		Path: []*gnmi.Path{
			{Elem: []*gnmi.PathElem{{Name: "system"}, {Name: "name"}, {Name: "host-name"}}},
		},
		Encoding: gnmi.Encoding_JSON,
	})
	if err != nil {
		t.Fatalf("cache Get failed: %v", err)
	}
	if len(cacheRsp.GetNotification()) != 1 {
		t.Fatalf("cache Get: expected 1 notification, got %d: %v", len(cacheRsp.GetNotification()), cacheRsp)
	}
	if got := cacheRsp.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal(); got != "router1" {
		t.Fatalf("cache Get: value %q, want router1", got)
	}
	// unknown target: empty response, no error
	emptyRsp, err := client.Get(getCtx, &gnmi.GetRequest{
		Prefix: &gnmi.Path{Target: "nope"},
		Path:   []*gnmi.Path{{Elem: []*gnmi.PathElem{{Name: "system"}}}},
	})
	if err != nil {
		t.Fatalf("cache Get for an unknown target failed: %v", err)
	}
	if len(emptyRsp.GetNotification()) != 0 {
		t.Fatalf("cache Get for an unknown target returned %d notifications", len(emptyRsp.GetNotification()))
	}

	// Capabilities
	capRsp, err := client.Capabilities(getCtx, &gnmi.CapabilityRequest{})
	if err != nil {
		t.Fatalf("Capabilities failed: %v", err)
	}
	if capRsp.GetGNMIVersion() == "" {
		t.Error("expected a gNMI version in the capabilities response")
	}
	if len(capRsp.GetSupportedEncodings()) == 0 {
		t.Error("expected supported encodings in the capabilities response")
	}

	// Set: the server is read-only by default,
	// Set RPCs must be rejected with an Unimplemented status.
	_, err = client.Set(getCtx, &gnmi.SetRequest{
		Prefix: &gnmi.Path{Target: "router1"},
		Update: []*gnmi.Update{
			{
				Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}}},
				Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "x"}},
			},
		},
	})
	if status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented error for Set on a read-only server, got %v", err)
	}
}

// TestServer_ReadOnlyDisabled verifies that setting read-only: false
// registers the Set handler.
func TestServer_ReadOnlyDisabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, st := newTestServer(t, ctx)

	addr := freeAddr(t)
	if _, err := st.Config.Set("gnmi-server", "gnmi-server", &config.GNMIServer{
		Address:  addr,
		ReadOnly: pointer(false),
	}); err != nil {
		t.Fatalf("seed gnmi-server config: %v", err)
	}

	wg := new(sync.WaitGroup)
	if err := s.Start(nil, wg); err != nil {
		t.Fatalf("failed to start gNMI server: %v", err)
	}
	defer func() {
		s.Stop()
		wg.Wait()
	}()

	conn := dialWithRetry(t, ctx, addr)
	defer conn.Close()
	client := gnmi.NewGNMIClient(conn)

	setCtx, setCancel := context.WithTimeout(ctx, 10*time.Second)
	defer setCancel()
	// the Set handler is registered: a Set request to an unknown target
	// must fail with NotFound, not Unimplemented.
	_, err := client.Set(setCtx, &gnmi.SetRequest{
		Prefix: &gnmi.Path{Target: "nope"},
		Update: []*gnmi.Update{
			{
				Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}}},
				Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "x"}},
			},
		},
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound error for Set to an unknown target, got %v", err)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func dialWithRetry(t *testing.T, ctx context.Context, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create a gRPC client for %s: %v", addr, err)
	}
	// NewClient connects lazily, wait until the connection is
	// ready so the tests don't race with the server startup.
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for state := conn.GetState(); state != connectivity.Ready; state = conn.GetState() {
		conn.Connect()
		if !conn.WaitForStateChange(waitCtx, state) {
			conn.Close()
			t.Fatalf("failed to connect to gNMI server at %s: last state %v", addr, state)
		}
	}
	return conn
}
