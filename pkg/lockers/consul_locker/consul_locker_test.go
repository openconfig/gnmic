// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package consul_locker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
)

// fakeConsul serves the KV delete and session destroy endpoints used by Unlock
// and records the requests it receives.
type fakeConsul struct {
	mu        sync.Mutex
	kvDeletes []string
	destroys  []string
	failKV    bool
	hang      bool
}

func (f *fakeConsul) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.hang {
		<-r.Context().Done()
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/kv/"):
		f.kvDeletes = append(f.kvDeletes, strings.TrimPrefix(r.URL.Path, "/v1/kv/"))
		if f.failKV {
			http.Error(w, "kv unavailable", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("true"))
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/session/destroy/"):
		f.destroys = append(f.destroys, strings.TrimPrefix(r.URL.Path, "/v1/session/destroy/"))
		w.Write([]byte("true"))
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeConsul) calls() (kvDeletes, destroys []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.kvDeletes...), append([]string(nil), f.destroys...)
}

func newTestLocker(t *testing.T, f *fakeConsul) *ConsulLocker {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	client, err := api.NewClient(&api.Config{Address: strings.TrimPrefix(srv.URL, "http://")})
	if err != nil {
		t.Fatalf("create consul client: %v", err)
	}
	return &ConsulLocker{
		client:          client,
		m:               &sync.Mutex{},
		acquiredlocks:   map[string]*locks{},
		attemptinglocks: map[string]*locks{},
	}
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestUnlock_attemptingLockIsForgotten(t *testing.T) {
	f := &fakeConsul{}
	c := newTestLocker(t, f)
	done := make(chan struct{})
	c.attemptinglocks["target"] = &locks{sessionID: "s1", doneChan: done}

	if err := c.Unlock(context.Background(), "target"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if !isClosed(done) {
		t.Fatal("Unlock did not close doneChan of the pending attempt")
	}
	if _, ok := c.attemptinglocks["target"]; ok {
		t.Fatal("Unlock left the attempting lock entry behind")
	}
	if _, destroys := f.calls(); len(destroys) != 1 || destroys[0] != "s1" {
		t.Fatalf("session destroys = %v, want [s1]", destroys)
	}
	// A second Unlock used to close doneChan again and panic.
	if err := c.Unlock(context.Background(), "target"); err == nil {
		t.Fatal("second Unlock of a released key returned no error")
	}
}

func TestUnlock_acquiredLockTwice(t *testing.T) {
	f := &fakeConsul{}
	c := newTestLocker(t, f)
	done := make(chan struct{})
	c.acquiredlocks["target"] = &locks{sessionID: "s1", doneChan: done}

	if err := c.Unlock(context.Background(), "target"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if !isClosed(done) {
		t.Fatal("Unlock did not close doneChan")
	}
	kvDeletes, destroys := f.calls()
	if len(kvDeletes) != 1 || kvDeletes[0] != "target" {
		t.Fatalf("kv deletes = %v, want [target]", kvDeletes)
	}
	if len(destroys) != 1 || destroys[0] != "s1" {
		t.Fatalf("session destroys = %v, want [s1]", destroys)
	}
	if err := c.Unlock(context.Background(), "target"); err == nil {
		t.Fatal("second Unlock of a released key returned no error")
	}
}

func TestUnlock_failedKVDeleteStillReleases(t *testing.T) {
	f := &fakeConsul{failKV: true}
	c := newTestLocker(t, f)
	c.acquiredlocks["target"] = &locks{sessionID: "s1", doneChan: make(chan struct{})}

	if err := c.Unlock(context.Background(), "target"); err == nil {
		t.Fatal("Unlock hid the failed kv delete")
	}
	if _, destroys := f.calls(); len(destroys) != 1 || destroys[0] != "s1" {
		t.Fatalf("session destroys = %v, want [s1]: the session must be destroyed even if the key delete fails", destroys)
	}
	if _, ok := c.acquiredlocks["target"]; ok {
		t.Fatal("failed Unlock left an entry with a closed doneChan")
	}
	// Used to panic: the entry stayed and its doneChan was closed again.
	if err := c.Unlock(context.Background(), "target"); err == nil {
		t.Fatal("second Unlock of a released key returned no error")
	}
}

// Lock re-registers the shared doneChan on every retry, so an attempt can be
// listed again after Unlock already closed its channel.
func TestUnlock_reRegisteredClosedChannel(t *testing.T) {
	f := &fakeConsul{}
	c := newTestLocker(t, f)
	done := make(chan struct{})
	close(done)
	c.attemptinglocks["target"] = &locks{sessionID: "s2", doneChan: done}

	if err := c.Unlock(context.Background(), "target"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestUnlock_honorsContext(t *testing.T) {
	f := &fakeConsul{hang: true}
	c := newTestLocker(t, f)
	c.attemptinglocks["target"] = &locks{sessionID: "s1", doneChan: make(chan struct{})}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- c.Unlock(ctx, "target") }()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Unlock against an unresponsive consul returned no error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Unlock ignored its context while destroying the session")
	}
}

// TestKeepLockDoesNotLeakWhenErrNotConsumed is a regression test for a
// goroutine leak: KeepLock spawns a goroutine that sends on errChan and, on the
// unknown-key path, closes doneChan afterwards. If errChan is unbuffered and the
// caller stops reading it (which happens as soon as the target owning the lock
// is deleted and its context is canceled), the send blocks forever and the
// goroutine never returns — leaking one goroutine per deleted target.
//
// The unknown-key branch does not touch the consul client, so it exercises the
// exact channel-blocking behavior without needing a consul server. With the
// unbuffered channel the goroutine blocks on the send and doneChan is never
// closed; with the buffered channel the send succeeds, doneChan is closed and
// the goroutine returns.
func TestKeepLockDoesNotLeakWhenErrNotConsumed(t *testing.T) {
	c := &ConsulLocker{
		m:               &sync.Mutex{},
		acquiredlocks:   map[string]*locks{},
		attemptinglocks: map[string]*locks{},
	}

	// No lock is registered for this key, so KeepLock takes the "unknown key"
	// path: it must send on errChan and then close doneChan. We deliberately do
	// NOT read errChan first, mimicking a consumer that has already gone away.
	doneChan, errChan := c.KeepLock(context.Background(), "no-such-key")

	select {
	case <-doneChan:
		// doneChan was closed => the goroutine ran to completion and did not
		// leak. Drain the buffered error so nothing is left dangling.
		select {
		case <-errChan:
		default:
		}
	case <-time.After(2 * time.Second):
		t.Fatal("KeepLock goroutine leaked: it blocked sending on errChan and never closed doneChan")
	}
}
