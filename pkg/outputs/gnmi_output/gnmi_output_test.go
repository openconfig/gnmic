// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package gnmi_output

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/match"
	"github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmic/pkg/outputs"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testUpdate(target string) *gnmi.SubscribeResponse {
	return &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Timestamp: time.Now().UnixNano(),
				Prefix:    &gnmi.Path{Target: target},
				Update: []*gnmi.Update{{
					Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}, {Name: "name"}}},
					Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "r1"}},
				}},
			},
		},
	}
}

func newTestOutput() *gNMIOutput {
	return &gNMIOutput{
		cfg:       &config{},
		logger:    discardLogger(),
		targetTpl: outputs.DefaultTargetTemplate,
		c:         cache.New(nil),
	}
}

// A sync-response used to be turned into a typed nil *gnmi.SubscribeResponse by
// AddSubscriptionTarget and crash Write on the first dereference.
func TestWriteSyncResponseDoesNotPanic(t *testing.T) {
	g := newTestOutput()
	meta := outputs.Meta{"source": "router1:57400", "subscription-name": "sub1"}
	g.Write(context.Background(), &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true},
	}, meta)
	g.Write(context.Background(), &gnmi.GetResponse{}, meta)
	if len(g.c.Metadata()) != 0 {
		t.Fatalf("no target should have been added to the cache")
	}
}

func TestWriteAddsTargetFromMeta(t *testing.T) {
	g := newTestOutput()
	meta := outputs.Meta{"source": "router1:57400", "subscription-name": "sub1"}
	g.Write(context.Background(), testUpdate(""), meta)
	if !g.c.HasTarget("router1") {
		t.Fatalf("expected target router1 to be added to the cache")
	}
	// an explicit target is kept as is
	g.Write(context.Background(), testUpdate("t2"), meta)
	if !g.c.HasTarget("t2") {
		t.Fatalf("expected target t2 to be added to the cache")
	}
}

// fakeSubscribeStream is a minimal gnmi.GNMI_SubscribeServer: it hands out a
// single request and records the responses.
type fakeSubscribeStream struct {
	grpc.ServerStream
	ctx context.Context
	//
	mu   sync.Mutex
	req  *gnmi.SubscribeRequest
	sent []*gnmi.SubscribeResponse
}

func (f *fakeSubscribeStream) Context() context.Context { return f.ctx }

func (f *fakeSubscribeStream) Recv() (*gnmi.SubscribeRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.req == nil {
		return nil, io.EOF
	}
	req := f.req
	f.req = nil
	return req, nil
}

func (f *fakeSubscribeStream) Send(rsp *gnmi.SubscribeResponse) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, rsp)
	return nil
}

func (f *fakeSubscribeStream) responses() []*gnmi.SubscribeResponse {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*gnmi.SubscribeResponse(nil), f.sent...)
}

func newTestServer(t *testing.T, maxSubscriptions int64) *server {
	t.Helper()
	c := cache.New(nil)
	c.Add("t1")
	if err := c.GnmiUpdate(testUpdate("t1").GetUpdate()); err != nil {
		t.Fatalf("failed to populate the cache: %v", err)
	}
	return &server{
		l:               discardLogger(),
		c:               c,
		m:               match.New(),
		subscribeRPCsem: semaphore.NewWeighted(maxSubscriptions),
		unaryRPCsem:     semaphore.NewWeighted(1),
		mu:              new(sync.RWMutex),
	}
}

func subscribeRequest(mode gnmi.SubscriptionList_Mode) *gnmi.SubscribeRequest {
	return &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{
				Prefix: &gnmi.Path{Target: "t1"},
				Mode:   mode,
				Subscription: []*gnmi.Subscription{{
					Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}}},
				}},
			},
		},
	}
}

// runSubscribe runs s.Subscribe and fails the test if it does not return.
func runSubscribe(t *testing.T, s *server, stream gnmi.GNMI_SubscribeServer) error {
	t.Helper()
	errCh := make(chan error, 1)
	go func() { errCh <- s.Subscribe(stream) }()
	select {
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Second):
		t.Fatalf("Subscribe did not return")
		return nil
	}
}

// An invalid subscription mode used to be rejected after a subscription
// spot was acquired, and the spot was never released.
func TestSubscribeInvalidModeDoesNotLeakSpot(t *testing.T) {
	s := newTestServer(t, 1)
	for i := 0; i < 3; i++ {
		stream := &fakeSubscribeStream{ctx: context.Background(), req: subscribeRequest(gnmi.SubscriptionList_Mode(42))}
		err := runSubscribe(t, s, stream)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("attempt %d: got %v, want InvalidArgument", i, err)
		}
	}
	if !s.subscribeRPCsem.TryAcquire(1) {
		t.Fatalf("the subscription spot was leaked")
	}
	s.subscribeRPCsem.Release(1)
}

// A ONCE subscription must end the RPC after the sync-response and release
// its subscription spot.
func TestSubscribeOnceReturns(t *testing.T) {
	s := newTestServer(t, 1)
	stream := &fakeSubscribeStream{ctx: context.Background(), req: subscribeRequest(gnmi.SubscriptionList_ONCE)}
	err := runSubscribe(t, s, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rsps := stream.responses()
	if len(rsps) != 2 {
		t.Fatalf("got %d responses, want an update and a sync-response: %v", len(rsps), rsps)
	}
	if rsps[0].GetUpdate() == nil {
		t.Fatalf("first response is not an update: %v", rsps[0])
	}
	if !rsps[1].GetSyncResponse() {
		t.Fatalf("second response is not a sync-response: %v", rsps[1])
	}
	if !s.subscribeRPCsem.TryAcquire(1) {
		t.Fatalf("the subscription spot was not released")
	}
	s.subscribeRPCsem.Release(1)
}

// A STREAM subscription must end when the client goes away, and release
// both its subscription spot and its match-tree registration.
func TestSubscribeStreamReturnsOnCancel(t *testing.T) {
	s := newTestServer(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	stream := &fakeSubscribeStream{ctx: ctx, req: subscribeRequest(gnmi.SubscriptionList_STREAM)}
	errCh := make(chan error, 1)
	go func() { errCh <- s.Subscribe(stream) }()
	// wait for the initial update and sync-response
	deadline := time.Now().Add(5 * time.Second)
	for len(stream.responses()) < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("initial responses not received: %v", stream.responses())
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("Subscribe did not return after the client went away")
	}
	if !s.subscribeRPCsem.TryAcquire(1) {
		t.Fatalf("the subscription spot was not released")
	}
	s.subscribeRPCsem.Release(1)
}
