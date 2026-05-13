// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package udp_output

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func newStore() store.Store[any] {
	return gomap.NewMemStore(store.StoreOptions[any]{})
}

func freeUDPAddr(t *testing.T) string {
	t.Helper()
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := c.LocalAddr().String()
	c.Close()
	return addr
}

func TestUDP_SetDefaults(t *testing.T) {
	c := &Config{}
	setDefaultsFor(c)
	if c.RetryInterval != defaultRetryTimer {
		t.Errorf("default retry interval = %v", c.RetryInterval)
	}
}

func TestUDP_Validate(t *testing.T) {
	u := &udpSock{}
	if err := u.Validate(map[string]any{}); err == nil {
		t.Errorf("expected error: missing address")
	}
	if err := u.Validate(map[string]any{"address": "bad"}); err == nil {
		t.Errorf("expected error: bad address format")
	}
	if err := u.Validate(map[string]any{"address": "127.0.0.1:1234"}); err != nil {
		t.Errorf("valid address: %v", err)
	}
	// decode failure
	if err := u.Validate(map[string]any{"buffer-size": "not-a-number"}); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestUDP_InitAndUpdateLifecycle(t *testing.T) {
	addr := freeUDPAddr(t)
	u := &udpSock{}
	cfg := map[string]any{
		"address":     addr,
		"format":      "event",
		"buffer-size": 16,
		"rate":        "10ms",
	}
	if err := u.Init(context.Background(), "udp1", cfg, outputs.WithConfigStore(newStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer u.Close()

	// String reflects config
	if !strings.Contains(u.String(), addr) {
		t.Errorf("String missing address: %s", u.String())
	}

	// Non-restarting Update: change format and rate.
	cfg2 := map[string]any{
		"address":     addr,
		"format":      "json",
		"buffer-size": 16,
		"rate":        "20ms",
	}
	if err := u.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update format/rate: %v", err)
	}
	// Update dropping rate (still no restart).
	cfg3 := map[string]any{
		"address":     addr,
		"format":      "json",
		"buffer-size": 16,
	}
	if err := u.Update(context.Background(), cfg3); err != nil {
		t.Fatalf("Update no rate: %v", err)
	}
	// Update changing buffer-size triggers channel swap + worker restart.
	cfg4 := map[string]any{
		"address":     addr,
		"format":      "json",
		"buffer-size": 32,
	}
	if err := u.Update(context.Background(), cfg4); err != nil {
		t.Fatalf("Update swap: %v", err)
	}
	// Update changing address triggers worker restart without channel swap.
	addr2 := freeUDPAddr(t)
	cfg5 := map[string]any{
		"address":     addr2,
		"format":      "json",
		"buffer-size": 32,
	}
	if err := u.Update(context.Background(), cfg5); err != nil {
		t.Fatalf("Update restart: %v", err)
	}
	// Update with bad config
	if err := u.Update(context.Background(), map[string]any{"buffer-size": "x"}); err == nil {
		t.Errorf("expected decode error on Update")
	}

	// UpdateProcessor with non-existent processor (should be no-op or error)
	if err := u.UpdateProcessor("missing", map[string]any{}); err != nil {
		// either error or no-op is acceptable - just exercise the path
		t.Logf("UpdateProcessor: %v", err)
	}
}

func TestUDP_InitErrors(t *testing.T) {
	u := &udpSock{}
	// bad address
	err := u.Init(context.Background(), "udp1", map[string]any{
		"address": "bad",
	}, outputs.WithConfigStore(newStore()))
	if err == nil {
		t.Errorf("expected error")
	}
	// decode error
	err = u.Init(context.Background(), "udp1", map[string]any{
		"buffer-size": "x",
	}, outputs.WithConfigStore(newStore()))
	if err == nil {
		t.Errorf("expected decode error")
	}
	// option error
	badOpt := func(*outputs.OutputOptions) error { return errBadOpt }
	err = u.Init(context.Background(), "udp1", map[string]any{
		"address": "127.0.0.1:1234",
	}, badOpt)
	if err == nil {
		t.Errorf("expected option error")
	}
}

func TestUDP_HelperPredicates(t *testing.T) {
	a := &Config{BufferSize: 1, Address: "a"}
	b := &Config{BufferSize: 2, Address: "a"}
	if !channelNeedsSwap(a, b) {
		t.Errorf("channelNeedsSwap(buffer change)")
	}
	if channelNeedsSwap(a, a) {
		t.Errorf("channelNeedsSwap(same)")
	}
	if !channelNeedsSwap(nil, a) {
		t.Errorf("nil left should swap")
	}
	c := &Config{Address: "x"}
	if !needsWorkerRestart(a, c) {
		t.Errorf("needsWorkerRestart(addr change)")
	}
	if !needsWorkerRestart(nil, a) {
		t.Errorf("nil left should restart")
	}
}

var errBadOpt = simpleErr("bad option")

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

// Touch Write/WriteEvent on a closed/empty output to ensure no panic.
func TestUDP_WriteEarlyReturn(t *testing.T) {
	u := &udpSock{}
	u.init()
	u.Write(context.Background(), nil, nil)
	u.WriteEvent(context.Background(), nil)
	// Close with no cancelFn / wg
	if err := u.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	// add small delay to ensure backgrounds settle
	time.Sleep(10 * time.Millisecond)
}
