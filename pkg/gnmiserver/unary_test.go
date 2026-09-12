// SPDX-License-Identifier: Apache-2.0

package gnmiserver

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmic/pkg/cache"
)

type readCall struct {
	sub, target, path string
}

// readCache records Read calls and answers them from a canned map keyed by
// target name.
type readCache struct {
	cache.Cache // panic on unimplemented methods

	mu    sync.Mutex
	calls []readCall
	data  map[string]map[string][]*gnmi.Notification // target -> sub -> notifications
	err   error
}

func (c *readCache) Read(sub, target string, p *gnmi.Path) (map[string][]*gnmi.Notification, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elems := make([]string, 0, len(p.GetElem()))
	for _, e := range p.GetElem() {
		elems = append(elems, e.GetName())
	}
	path := p.GetOrigin() + ":/" + strings.Join(elems, "/")
	c.calls = append(c.calls, readCall{sub: sub, target: target, path: path})
	if c.err != nil {
		return nil, c.err
	}
	if c.data == nil {
		return map[string][]*gnmi.Notification{}, nil
	}
	return c.data[target], nil
}

func notif(target, leaf string) *gnmi.Notification {
	return &gnmi.Notification{
		Timestamp: 42,
		Prefix:    &gnmi.Path{Target: target},
		Update: []*gnmi.Update{{
			Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}, {Name: leaf}}},
			Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: target}},
		}},
	}
}

func getRequest(target string, paths ...*gnmi.Path) *gnmi.GetRequest {
	return &gnmi.GetRequest{
		Prefix:   &gnmi.Path{Target: target},
		Path:     paths,
		Encoding: gnmi.Encoding_JSON_IETF,
		Type:     gnmi.GetRequest_STATE,
	}
}

func systemPath() *gnmi.Path {
	return &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}}}
}

func TestGet_ServedFromCache(t *testing.T) {
	c := &readCache{data: map[string]map[string][]*gnmi.Notification{
		"t1": {"sub-b": {notif("t1", "b")}, "sub-a": {notif("t1", "a")}},
	}}
	h := New(Config{}, c, nil, nil, nil)
	rsp, err := h.Get(peerCtx(context.Background()), getRequest("t1", systemPath()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rsp.GetNotification()) != 2 {
		t.Fatalf("got %d notifications, want 2", len(rsp.GetNotification()))
	}
	// deterministic order: subscription caches sorted by name
	if got := rsp.GetNotification()[0].GetUpdate()[0].GetPath().GetElem()[1].GetName(); got != "a" {
		t.Fatalf("first notification from sub %q, want sub-a", got)
	}
	if len(c.calls) != 1 || c.calls[0] != (readCall{sub: "*", target: "t1", path: ":/system"}) {
		t.Fatalf("cache calls %+v", c.calls)
	}
}

func TestGet_WildcardAndEmptyTarget(t *testing.T) {
	for _, target := range []string{"", "*"} {
		c := &readCache{}
		h := New(Config{}, c, nil, nil, nil)
		if _, err := h.Get(peerCtx(context.Background()), getRequest(target, systemPath())); err != nil {
			t.Fatalf("target %q: %v", target, err)
		}
		if len(c.calls) != 1 || c.calls[0].target != "*" {
			t.Fatalf("target %q: cache calls %+v, want one call for target *", target, c.calls)
		}
	}
}

func TestGet_CommaSeparatedTargets(t *testing.T) {
	c := &readCache{}
	h := New(Config{}, c, nil, nil, nil)
	if _, err := h.Get(peerCtx(context.Background()), getRequest("t1,t2", systemPath())); err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != 2 || c.calls[0].target != "t1" || c.calls[1].target != "t2" {
		t.Fatalf("cache calls %+v", c.calls)
	}
}

func TestGet_PrefixAndPathAreMerged(t *testing.T) {
	c := &readCache{}
	h := New(Config{}, c, nil, nil, nil)
	req := &gnmi.GetRequest{
		Prefix: &gnmi.Path{Target: "t1", Origin: "openconfig", Elem: []*gnmi.PathElem{{Name: "interfaces"}}},
		Path: []*gnmi.Path{
			{Elem: []*gnmi.PathElem{{Name: "interface"}, {Name: "state"}}},
			{Elem: []*gnmi.PathElem{{Name: "interface"}, {Name: "config"}}},
		},
	}
	if _, err := h.Get(peerCtx(context.Background()), req); err != nil {
		t.Fatal(err)
	}
	want := []string{"openconfig:/interfaces/interface/state", "openconfig:/interfaces/interface/config"}
	if len(c.calls) != 2 || c.calls[0].path != want[0] || c.calls[1].path != want[1] {
		t.Fatalf("cache calls %+v, want paths %v", c.calls, want)
	}
	// the request prefix must not have grown
	if len(req.GetPrefix().GetElem()) != 1 {
		t.Fatalf("request prefix mutated: %v", req.GetPrefix())
	}
}

func TestGet_PrefixOnly(t *testing.T) {
	c := &readCache{}
	h := New(Config{}, c, nil, nil, nil)
	req := &gnmi.GetRequest{Prefix: &gnmi.Path{Target: "t1", Elem: []*gnmi.PathElem{{Name: "system"}}}}
	if _, err := h.Get(peerCtx(context.Background()), req); err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != 1 || c.calls[0].path != ":/system" {
		t.Fatalf("cache calls %+v", c.calls)
	}
}

func TestGet_EmptyResultIsNotAnError(t *testing.T) {
	h := New(Config{}, &readCache{}, nil, nil, nil)
	rsp, err := h.Get(peerCtx(context.Background()), getRequest("unknown", systemPath()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rsp.GetNotification()) != 0 {
		t.Fatalf("got %d notifications, want none", len(rsp.GetNotification()))
	}
}

func TestGet_Errors(t *testing.T) {
	t.Run("no path and no prefix", func(t *testing.T) {
		h := New(Config{}, &readCache{}, nil, nil, nil)
		_, err := h.Get(peerCtx(context.Background()), &gnmi.GetRequest{})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("got %v, want InvalidArgument", err)
		}
	})
	t.Run("no cache", func(t *testing.T) {
		h := New(Config{}, nil, nil, nil, nil)
		_, err := h.Get(peerCtx(context.Background()), getRequest("t1", systemPath()))
		if status.Code(err) != codes.Unimplemented {
			t.Fatalf("got %v, want Unimplemented", err)
		}
	})
	t.Run("cache read error", func(t *testing.T) {
		h := New(Config{}, &readCache{err: errors.New("boom")}, nil, nil, nil)
		_, err := h.Get(peerCtx(context.Background()), getRequest("t1", systemPath()))
		if status.Code(err) != codes.Internal {
			t.Fatalf("got %v, want Internal", err)
		}
	})
	t.Run("mixed origins", func(t *testing.T) {
		h := New(Config{}, &readCache{}, nil, nil, func(context.Context, *gnmi.GetRequest) (*gnmi.GetResponse, error) {
			return &gnmi.GetResponse{}, nil
		})
		req := getRequest("t1",
			systemPath(),
			&gnmi.Path{Origin: "gnmic", Elem: []*gnmi.PathElem{{Name: "targets"}}},
		)
		_, err := h.Get(peerCtx(context.Background()), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("got %v, want InvalidArgument", err)
		}
	})
}

func TestGet_GnmicOriginIsInternal(t *testing.T) {
	c := &readCache{}
	called := false
	h := New(Config{}, c, nil, nil, func(context.Context, *gnmi.GetRequest) (*gnmi.GetResponse, error) {
		called = true
		return &gnmi.GetResponse{}, nil
	})
	req := getRequest("", &gnmi.Path{Origin: "gnmic", Elem: []*gnmi.PathElem{{Name: "targets"}}})
	if _, err := h.Get(peerCtx(context.Background()), req); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("internal Get handler not called")
	}
	if len(c.calls) != 0 {
		t.Fatalf("cache must not be read for the gnmic origin, calls %+v", c.calls)
	}
	// and without an internal handler, Unimplemented
	h = New(Config{}, c, nil, nil, nil)
	if _, err := h.Get(peerCtx(context.Background()), req); status.Code(err) != codes.Unimplemented {
		t.Fatalf("got %v, want Unimplemented", err)
	}
}
