// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
)

// These tests cover the on-change read path resolving subscription caches and
// targets that appear after the read started. The cache's send to a reader
// blocks until it is read, so writes made after Subscribe run in a goroutine
// and the reader drains the channel concurrently.

func subUpdate(target string, ts int64, leaf string) *gnmi.SubscribeResponse {
	return &gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
		Timestamp: ts,
		Prefix:    &gnmi.Path{Target: target, Elem: []*gnmi.PathElem{{Name: "system"}}},
		Update: []*gnmi.Update{{
			Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: leaf}}},
			// the value carries the timestamp: the openconfig cache does not
			// notify clients when a leaf is rewritten with an unchanged value.
			Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: fmt.Sprintf("%s-%d", target, ts)}},
		}},
	}}}
}

type received struct {
	sub, target string
	deleted     bool
}

// drain reads ch in the background and returns a function that reports
// what was received so far and whether ch was closed, after waiting for the
// channel to close or for the given duration, whichever comes first.
func drain(ch chan *Notification) func(wait time.Duration) ([]received, bool) {
	done := make(chan struct{})
	var (
		mu     sync.Mutex
		got    []received
		closed bool
	)
	go func() {
		defer close(done)
		for n := range ch {
			if n.Notification == nil {
				continue
			}
			mu.Lock()
			got = append(got, received{
				sub:     n.Name,
				target:  n.Notification.GetPrefix().GetTarget(),
				deleted: len(n.Notification.GetDelete()) > 0,
			})
			mu.Unlock()
		}
		mu.Lock()
		closed = true
		mu.Unlock()
	}()
	return func(wait time.Duration) ([]received, bool) {
		select {
		case <-done:
		case <-time.After(wait):
		}
		mu.Lock()
		defer mu.Unlock()
		return append([]received(nil), got...), closed
	}
}

func onChangeOpts(target string, updatesOnly bool, paths ...string) *ReadOpts {
	if len(paths) == 0 {
		paths = []string{"system"}
	}
	ro := &ReadOpts{Target: target, Mode: ReadMode_StreamOnChange, UpdatesOnly: updatesOnly}
	for _, p := range paths {
		ro.Paths = append(ro.Paths, &gnmi.Path{Elem: []*gnmi.PathElem{{Name: p}}})
	}
	return ro
}

func newTestCache() *gnmiCache { return newGNMICache(&Config{Type: "oc"}, "") }

const settle = 100 * time.Millisecond

func expect(t *testing.T, got []received, want ...received) {
	t.Helper()
	key := func(r received) string {
		return r.sub + "/" + r.target + "/" + map[bool]string{true: "del", false: "upd"}[r.deleted]
	}
	g := make([]string, 0, len(got))
	for _, r := range got {
		g = append(g, key(r))
	}
	w := make([]string, 0, len(want))
	for _, r := range want {
		w = append(w, key(r))
	}
	sort.Strings(g)
	sort.Strings(w)
	if len(g) != len(w) {
		t.Fatalf("received %v, want %v", g, w)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("received %v, want %v", g, w)
		}
	}
}

// target=* on a cache that has no data yet: the read must stay open and
// deliver the first update that arrives.
func TestOnChange_EmptyCache_WildcardTarget(t *testing.T) {
	c := newTestCache()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := drain(c.Subscribe(ctx, onChangeOpts("*", false)))
	time.Sleep(settle)
	go c.Write(context.Background(), "s1", subUpdate("t1", 1, "name"))
	got, closed := stop(settle)
	if closed {
		t.Fatalf("read ended on an empty cache")
	}
	expect(t, got, received{sub: "s1", target: "t1"})
}

// a subscription cache created after the read started is delivered.
func TestOnChange_LateSubscriptionCache(t *testing.T) {
	c := newTestCache()
	c.Write(context.Background(), "s1", subUpdate("t1", 1, "name"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := drain(c.Subscribe(ctx, onChangeOpts("*", true)))
	time.Sleep(settle)
	go c.Write(context.Background(), "s2", subUpdate("t1", 2, "name"))
	got, closed := stop(settle)
	if closed {
		t.Fatalf("read ended")
	}
	expect(t, got, received{sub: "s2", target: "t1"})
}

// a named target that does not exist yet is delivered once it appears.
func TestOnChange_LateNamedTarget(t *testing.T) {
	c := newTestCache()
	c.Write(context.Background(), "s1", subUpdate("t1", 1, "name"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := drain(c.Subscribe(ctx, onChangeOpts("t2", false)))
	time.Sleep(settle)
	go c.Write(context.Background(), "s1", subUpdate("t1", 2, "name")) // other target, must not match
	go c.Write(context.Background(), "s1", subUpdate("t2", 3, "name"))
	got, closed := stop(settle)
	if closed {
		t.Fatalf("read ended on an unknown target")
	}
	expect(t, got, received{sub: "s1", target: "t2"})
}

// a target added to an existing subscription cache is delivered (this
// already worked, kept as a guard).
func TestOnChange_LateTargetSameSubscription(t *testing.T) {
	c := newTestCache()
	c.Write(context.Background(), "s1", subUpdate("t1", 1, "name"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := drain(c.Subscribe(ctx, onChangeOpts("*", true)))
	time.Sleep(settle)
	go c.Write(context.Background(), "s1", subUpdate("t2", 2, "name"))
	got, _ := stop(settle)
	expect(t, got, received{sub: "s1", target: "t2"})
}

// the initial state of the caches that exist is sent, then live updates.
func TestOnChange_InitialStateThenUpdates(t *testing.T) {
	c := newTestCache()
	c.Write(context.Background(), "s1", subUpdate("t1", 1, "name"))
	c.Write(context.Background(), "s2", subUpdate("t1", 1, "name"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := drain(c.Subscribe(ctx, onChangeOpts("*", false)))
	time.Sleep(settle)
	go c.Write(context.Background(), "s1", subUpdate("t1", 2, "name"))
	got, _ := stop(settle)
	expect(t, got,
		received{sub: "s1", target: "t1"}, // initial
		received{sub: "s2", target: "t1"}, // initial
		received{sub: "s1", target: "t1"}, // update
	)
}

// the subscription filter is honoured for both the initial state and the
// live updates.
func TestOnChange_SubscriptionFilter(t *testing.T) {
	c := newTestCache()
	c.Write(context.Background(), "s1", subUpdate("t1", 1, "name"))
	c.Write(context.Background(), "s2", subUpdate("t1", 1, "name"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ro := onChangeOpts("*", false)
	ro.Subscription = "s2"
	stop := drain(c.Subscribe(ctx, ro))
	time.Sleep(settle)
	go c.Write(context.Background(), "s1", subUpdate("t1", 2, "name"))
	go c.Write(context.Background(), "s2", subUpdate("t2", 2, "name"))
	got, _ := stop(settle)
	expect(t, got,
		received{sub: "s2", target: "t1"}, // initial
		received{sub: "s2", target: "t2"}, // update
	)
}

// a removed target produces a delete, and its data flows again when it comes back.
func TestOnChange_TargetRemovedAndReadded(t *testing.T) {
	c := newTestCache()
	c.Write(context.Background(), "s1", subUpdate("t1", 1, "name"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := drain(c.Subscribe(ctx, onChangeOpts("*", true)))
	time.Sleep(settle)
	go func() {
		c.DeleteTarget("t1")
		c.Write(context.Background(), "s1", subUpdate("t1", 2, "name"))
	}()
	got, _ := stop(settle)
	expect(t, got,
		received{sub: "s1", target: "t1", deleted: true},
		received{sub: "s1", target: "t1"},
	)
}

// every requested path is registered, also with a heartbeat (the previous
// implementation blocked on the heartbeat of the first path).
func TestOnChange_MultiplePathsWithHeartbeat(t *testing.T) {
	c := newTestCache()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ro := onChangeOpts("*", true, "system", "interfaces")
	ro.HeartbeatInterval = time.Hour
	stop := drain(c.Subscribe(ctx, ro))
	time.Sleep(settle)
	go c.Write(context.Background(), "s1", &gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
		Timestamp: 1,
		Prefix:    &gnmi.Path{Target: "t1", Elem: []*gnmi.PathElem{{Name: "interfaces"}}},
		Update: []*gnmi.Update{{
			Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interface", Key: map[string]string{"name": "eth0"}}, {Name: "state"}}},
			Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "up"}},
		}},
	}}})
	got, _ := stop(settle)
	expect(t, got, received{sub: "s1", target: "t1"})
}

// the read ends when the context does, and the query is removed from the tree.
func TestOnChange_ContextCancelClosesAndUnregisters(t *testing.T) {
	c := newTestCache()
	ctx, cancel := context.WithCancel(context.Background())
	stop := drain(c.Subscribe(ctx, onChangeOpts("*", true)))
	time.Sleep(settle)
	cancel()
	_, closed := stop(time.Second)
	if !closed {
		t.Fatalf("channel not closed after cancel")
	}
	// a write after the reader is gone must not block: no client is left in the tree.
	done := make(chan struct{})
	go func() {
		c.Write(context.Background(), "s1", subUpdate("t1", 1, "name"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("write blocked on a stale match client")
	}
}
