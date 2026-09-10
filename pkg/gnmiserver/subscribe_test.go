// SPDX-License-Identifier: Apache-2.0

package gnmiserver

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/peer"

	"github.com/openconfig/gnmic/pkg/cache"
)

// roSnapshot records the fields of a cache.ReadOpts at Subscribe call time,
// the handlers mutate the same ReadOpts between calls.
type roSnapshot struct {
	Mode              string
	UpdatesOnly       bool
	SampleInterval    time.Duration
	HeartbeatInterval time.Duration
}

// fakeCache records Subscribe calls and replies to each with a single
// notification carrying the call index as timestamp.
type fakeCache struct {
	cache.Cache // panic on unimplemented methods

	mu    sync.Mutex
	calls []roSnapshot
}

func (c *fakeCache) Subscribe(ctx context.Context, ro *cache.ReadOpts) chan *cache.Notification {
	c.mu.Lock()
	c.calls = append(c.calls, roSnapshot{
		Mode:              ro.Mode,
		UpdatesOnly:       ro.UpdatesOnly,
		SampleInterval:    ro.SampleInterval,
		HeartbeatInterval: ro.HeartbeatInterval,
	})
	idx := len(c.calls)
	c.mu.Unlock()

	ch := make(chan *cache.Notification)
	go func() {
		defer close(ch)
		n := &cache.Notification{
			Notification: &gnmi.Notification{
				Timestamp: int64(idx),
				Prefix:    &gnmi.Path{Target: "target1"},
			},
		}
		select {
		case ch <- n:
		case <-ctx.Done():
		}
	}()
	return ch
}

func (c *fakeCache) snapshot() []roSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]roSnapshot(nil), c.calls...)
}

// fakeStream implements the server side of the Subscribe RPC stream.
type fakeStream struct {
	gnmi.GNMI_SubscribeServer // panic on unimplemented methods

	ctx context.Context

	mu   sync.Mutex
	sent []*gnmi.SubscribeResponse
}

func (f *fakeStream) Context() context.Context { return f.ctx }

func (f *fakeStream) Send(rsp *gnmi.SubscribeResponse) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, rsp)
	return nil
}

func (f *fakeStream) snapshot() []*gnmi.SubscribeResponse {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*gnmi.SubscribeResponse(nil), f.sent...)
}

func peerCtx(ctx context.Context) context.Context {
	return peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}})
}

func streamRequest(updatesOnly bool) *gnmi.SubscribeRequest {
	return &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{
				Mode:        gnmi.SubscriptionList_STREAM,
				UpdatesOnly: updatesOnly,
				Subscription: []*gnmi.Subscription{
					{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}}},
						Mode: gnmi.SubscriptionMode_ON_CHANGE,
					},
				},
			},
		},
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", msg)
}

// TestSubscribeStreamInitialSync verifies that a STREAM subscription first
// receives the current state, then a sync_response, then streamed updates.
func TestSubscribeStreamInitialSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fc := &fakeCache{}
	stream := &fakeStream{ctx: peerCtx(ctx)}
	h := New(Config{}, fc, nil, nil, nil)

	done := make(chan error, 1)
	go func() { done <- h.Subscribe(streamRequest(false), stream) }()

	// initial state, sync_response, then the streamed update.
	waitFor(t, 5*time.Second, func() bool { return len(stream.snapshot()) >= 3 }, "3 responses on the stream")
	cancel()
	<-done

	sent := stream.snapshot()
	if sent[0].GetUpdate() == nil || sent[0].GetUpdate().GetTimestamp() != 1 {
		t.Fatalf("first response must be the initial state from the once read, got: %v", sent[0])
	}
	if !sent[1].GetSyncResponse() {
		t.Fatalf("second response must be sync_response, got: %v", sent[1])
	}
	if sent[2].GetUpdate() == nil || sent[2].GetUpdate().GetTimestamp() != 2 {
		t.Fatalf("third response must be the streamed update, got: %v", sent[2])
	}

	calls := fc.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected 2 cache subscribe calls, got %d: %v", len(calls), calls)
	}
	if calls[0].Mode != cache.ReadMode_Once || calls[0].UpdatesOnly {
		t.Fatalf("first cache call must be a full once read, got: %+v", calls[0])
	}
	if calls[1].Mode != cache.ReadMode_StreamOnChange || !calls[1].UpdatesOnly {
		t.Fatalf("second cache call must be an updates-only stream read, got: %+v", calls[1])
	}
}

// TestSubscribeStreamUpdatesOnly verifies that with updates_only set, the
// first response is the sync_response and the cache is never read for the
// initial state.
func TestSubscribeStreamUpdatesOnly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fc := &fakeCache{}
	stream := &fakeStream{ctx: peerCtx(ctx)}
	h := New(Config{}, fc, nil, nil, nil)

	done := make(chan error, 1)
	go func() { done <- h.Subscribe(streamRequest(true), stream) }()

	// sync_response then the streamed update.
	waitFor(t, 5*time.Second, func() bool { return len(stream.snapshot()) >= 2 }, "2 responses on the stream")
	cancel()
	<-done

	sent := stream.snapshot()
	if !sent[0].GetSyncResponse() {
		t.Fatalf("first response must be sync_response with updates_only, got: %v", sent[0])
	}
	if sent[1].GetUpdate() == nil {
		t.Fatalf("second response must be the streamed update, got: %v", sent[1])
	}

	calls := fc.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected 1 cache subscribe call, got %d: %v", len(calls), calls)
	}
	if calls[0].Mode != cache.ReadMode_StreamOnChange || !calls[0].UpdatesOnly {
		t.Fatalf("cache call must be an updates-only stream read, got: %+v", calls[0])
	}
}

// TestStreamReadOpts verifies the sample and heartbeat interval bounds.
func TestStreamReadOpts(t *testing.T) {
	h := New(Config{
		DefaultSampleInterval: 10 * time.Second,
		MinSampleInterval:     time.Second,
		MinHeartbeatInterval:  30 * time.Second,
	}, nil, nil, nil, nil)

	tests := []struct {
		name          string
		sub           *gnmi.Subscription
		wantMode      string
		wantSample    time.Duration
		wantHeartbeat time.Duration
	}{
		{
			name:     "on-change",
			sub:      &gnmi.Subscription{Mode: gnmi.SubscriptionMode_ON_CHANGE},
			wantMode: cache.ReadMode_StreamOnChange,
		},
		{
			name:          "on-change heartbeat clamped to minimum",
			sub:           &gnmi.Subscription{Mode: gnmi.SubscriptionMode_ON_CHANGE, HeartbeatInterval: uint64(time.Second)},
			wantMode:      cache.ReadMode_StreamOnChange,
			wantHeartbeat: 30 * time.Second,
		},
		{
			name:          "on-change heartbeat above minimum kept",
			sub:           &gnmi.Subscription{Mode: gnmi.SubscriptionMode_ON_CHANGE, HeartbeatInterval: uint64(time.Minute)},
			wantMode:      cache.ReadMode_StreamOnChange,
			wantHeartbeat: time.Minute,
		},
		{
			name:       "sample default interval",
			sub:        &gnmi.Subscription{Mode: gnmi.SubscriptionMode_SAMPLE},
			wantMode:   cache.ReadMode_StreamSample,
			wantSample: 10 * time.Second,
		},
		{
			name:       "sample interval clamped to minimum",
			sub:        &gnmi.Subscription{Mode: gnmi.SubscriptionMode_SAMPLE, SampleInterval: uint64(time.Millisecond)},
			wantMode:   cache.ReadMode_StreamSample,
			wantSample: time.Second,
		},
		{
			name:          "sample heartbeat clamped to minimum",
			sub:           &gnmi.Subscription{Mode: gnmi.SubscriptionMode_SAMPLE, SampleInterval: uint64(5 * time.Second), HeartbeatInterval: uint64(time.Second)},
			wantMode:      cache.ReadMode_StreamSample,
			wantSample:    5 * time.Second,
			wantHeartbeat: 30 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ro := h.streamReadOpts("target1", nil, tt.sub)
			if ro.Mode != tt.wantMode {
				t.Fatalf("mode: got %q, want %q", ro.Mode, tt.wantMode)
			}
			if ro.SampleInterval != tt.wantSample {
				t.Fatalf("sample interval: got %v, want %v", ro.SampleInterval, tt.wantSample)
			}
			if ro.HeartbeatInterval != tt.wantHeartbeat {
				t.Fatalf("heartbeat interval: got %v, want %v", ro.HeartbeatInterval, tt.wantHeartbeat)
			}
		})
	}
}
