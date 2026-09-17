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
	"sync"
	"testing"
	"time"
)

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
