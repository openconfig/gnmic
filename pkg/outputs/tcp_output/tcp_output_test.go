// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package tcp_output

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func newStore() store.Store[any] {
	return gomap.NewMemStore(store.StoreOptions[any]{})
}

func freeTCPListener(t *testing.T) (string, func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				buf := make([]byte, 4096)
				for {
					if _, err := c.Read(buf); err != nil {
						c.Close()
						return
					}
				}
			}(c)
		}
	}()
	return l.Addr().String(), func() { l.Close() }
}

func TestTCP_SetDefaults(t *testing.T) {
	c := &config{}
	setDefaultsFor(c)
	if c.RetryInterval != defaultRetryTimer {
		t.Errorf("retry interval default")
	}
	if c.NumWorkers != defaultNumWorkers {
		t.Errorf("num workers default")
	}
}

func TestTCP_Validate(t *testing.T) {
	t1 := &tcpOutput{}
	if err := t1.Validate(map[string]any{}); err == nil {
		t.Errorf("expected missing address error")
	}
	if err := t1.Validate(map[string]any{"address": "bad"}); err == nil {
		t.Errorf("expected bad address")
	}
	// missing target-template
	if err := t1.Validate(map[string]any{"address": "127.0.0.1:1"}); err == nil {
		t.Errorf("expected target-template error")
	}
	if err := t1.Validate(map[string]any{
		"address":         "127.0.0.1:1",
		"target-template": "foo",
	}); err != nil {
		t.Errorf("valid: %v", err)
	}
	// decode failure
	if err := t1.Validate(map[string]any{"buffer-size": "x"}); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestTCP_InitAndUpdate(t *testing.T) {
	addr, stop := freeTCPListener(t)
	defer stop()

	o := &tcpOutput{}
	cfg := map[string]any{
		"address":         addr,
		"format":          "event",
		"buffer-size":     16,
		"rate":            "10ms",
		"delimiter":       "\n",
		"num-workers":     1,
		"target-template": "{{ .target }}",
	}
	if err := o.Init(context.Background(), "tcp1", cfg, outputs.WithConfigStore(newStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer o.Close()

	if !strings.Contains(o.String(), addr) {
		t.Errorf("String missing address: %s", o.String())
	}

	// Non-restarting Update: change format/rate/delimiter only.
	cfg2 := map[string]any{
		"address":         addr,
		"format":          "json",
		"buffer-size":     16,
		"rate":            "20ms",
		"delimiter":       "|",
		"num-workers":     1,
		"target-template": "{{ .target }}",
	}
	if err := o.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update no-op: %v", err)
	}
	// Update changing buffer-size triggers swap + restart.
	cfg3 := map[string]any{
		"address":         addr,
		"format":          "json",
		"buffer-size":     32,
		"num-workers":     1,
		"target-template": "{{ .target }}",
	}
	if err := o.Update(context.Background(), cfg3); err != nil {
		t.Fatalf("Update swap: %v", err)
	}
	// Update changing num-workers triggers restart only.
	cfg4 := map[string]any{
		"address":         addr,
		"format":          "json",
		"buffer-size":     32,
		"num-workers":     2,
		"target-template": "{{ .target }}",
	}
	if err := o.Update(context.Background(), cfg4); err != nil {
		t.Fatalf("Update restart: %v", err)
	}
	// Decode error.
	if err := o.Update(context.Background(), map[string]any{"buffer-size": "x"}); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestTCP_InitErrors(t *testing.T) {
	o := &tcpOutput{}
	if err := o.Init(context.Background(), "tcp1", map[string]any{
		"address": "bad",
	}, outputs.WithConfigStore(newStore())); err == nil {
		t.Errorf("expected bad address")
	}
	o = &tcpOutput{}
	if err := o.Init(context.Background(), "tcp1", map[string]any{
		"buffer-size": "x",
	}, outputs.WithConfigStore(newStore())); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestTCP_Predicates(t *testing.T) {
	a := &config{BufferSize: 1, NumWorkers: 1}
	b := &config{BufferSize: 2, NumWorkers: 1}
	if !channelNeedsSwap(a, b) {
		t.Errorf("swap on bs change")
	}
	if channelNeedsSwap(a, a) {
		t.Errorf("no swap same")
	}
	if !channelNeedsSwap(nil, a) {
		t.Errorf("swap on nil")
	}
	c := &config{NumWorkers: 2}
	if !needsWorkerRestart(a, c) {
		t.Errorf("restart on workers change")
	}
	if !needsWorkerRestart(nil, a) {
		t.Errorf("restart on nil")
	}
}
