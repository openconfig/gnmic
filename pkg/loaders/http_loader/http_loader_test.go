// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package http_loader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/openconfig/gnmic/pkg/actions"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/loaders"
)

// fakeAction is a minimal implementation of actions.Action for testing.
type fakeAction struct {
	name  string
	delay time.Duration
	fail  bool
}

func (f *fakeAction) Init(cfg map[string]interface{}, opts ...actions.Option) error {
	if v, ok := cfg["name"].(string); ok {
		f.name = v
	}
	return nil
}
func (f *fakeAction) Run(ctx context.Context, aCtx *actions.Context) (interface{}, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
		}
	}
	if f.fail {
		return nil, errors.New("forced failure")
	}
	return "ok", nil
}
func (f *fakeAction) NName() string                              { return f.name }
func (f *fakeAction) WithTargets(map[string]*types.TargetConfig) {}
func (f *fakeAction) WithLogger(*log.Logger)                     {}

func newTestLoader(t *testing.T) *httpLoader {
	t.Helper()
	return &httpLoader{
		cfg:            &cfg{Interval: 500 * time.Millisecond},
		m:              new(sync.RWMutex),
		lastTargets:    make(map[string]*types.TargetConfig),
		targetConfigFn: func(tc *types.TargetConfig) error { return nil },
		logger:         log.New(io.Discard, "", 0),
	}
}

func TestRunActions_AddAndDelete_NoDeadlock(t *testing.T) {
	hl := newTestLoader(t)
	// ensure actions are present to exercise the concurrent paths
	hl.addActions = []actions.Action{&fakeAction{name: "add1", delay: 10 * time.Millisecond}}
	hl.delActions = []actions.Action{&fakeAction{name: "del1", delay: 10 * time.Millisecond}}
	hl.numActions = len(hl.addActions) + len(hl.delActions)

	tcs := map[string]*types.TargetConfig{
		"t-add": {Name: "t-add", Address: "10.0.0.1"},
		"t-del": {Name: "t-del", Address: "10.0.0.2"},
	}
	op := &loaders.TargetOperation{
		Add: map[string]*types.TargetConfig{
			"t-add": tcs["t-add"],
		},
		Del: []string{"t-del"},
	}

	ctx := context.Background()

	done := make(chan struct{})
	var res *loaders.TargetOperation
	var err error
	go func() {
		res, err = hl.runActions(ctx, tcs, op)
		close(done)
	}()

	select {
	case <-done:
		if err != nil {
			t.Fatalf("runActions returned error: %v", err)
		}
		if _, ok := res.Add["t-add"]; !ok {
			t.Fatalf("expected add for 't-add', got: %+v", res.Add)
		}
		if !slices.Contains(res.Del, "t-del") {
			t.Fatalf("expected delete for 't-del', got: %+v", res.Del)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runActions timed out (possible deadlock)")
	}
}

func TestRunActions_ReplaceSameName_NoDeadlock(t *testing.T) {
	hl := newTestLoader(t)
	hl.addActions = []actions.Action{&fakeAction{name: "add1", delay: 10 * time.Millisecond}}
	hl.delActions = []actions.Action{&fakeAction{name: "del1", delay: 10 * time.Millisecond}}
	hl.numActions = len(hl.addActions) + len(hl.delActions)

	oldTC := &types.TargetConfig{Name: "t1", Address: "10.0.0.1"}
	newTC := &types.TargetConfig{Name: "t1", Address: "10.0.0.1"}

	tcs := map[string]*types.TargetConfig{
		"t1": newTC,
	}
	op := &loaders.TargetOperation{
		Add: map[string]*types.TargetConfig{
			"t1": newTC,
		},
		Del: []string{"t1"},
	}
	// seed lastTargets to emulate prior state (not directly used by runActions but mirrors scenario)
	hl.lastTargets["t1"] = oldTC

	ctx := context.Background()

	done := make(chan struct{})
	var res *loaders.TargetOperation
	var err error
	go func() {
		res, err = hl.runActions(ctx, tcs, op)
		close(done)
	}()

	select {
	case <-done:
		if err != nil {
			t.Fatalf("runActions returned error: %v", err)
		}
		if _, ok := res.Add["t1"]; !ok {
			t.Fatalf("expected add for 't1', got: %+v", res.Add)
		}
		if !slices.Contains(res.Del, "t1") {
			t.Fatalf("expected delete for 't1', got: %+v", res.Del)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("runActions timed out (possible deadlock)")
	}
}

func TestSetDefaults(t *testing.T) {
	// missing URL
	hl := newTestLoader(t)
	hl.cfg.URL = ""
	if err := hl.setDefaults(); err == nil {
		t.Fatal("expected error for missing URL")
	}
	// valid URL sets default interval/timeout
	hl = newTestLoader(t)
	hl.cfg.URL = "http://localhost"
	hl.cfg.Interval = 0
	hl.cfg.Timeout = 0
	if err := hl.setDefaults(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hl.cfg.Interval <= 0 || hl.cfg.Timeout <= 0 {
		t.Fatal("expected defaults for interval and timeout to be set")
	}
}

func TestReadVars_FromFileAndMerge(t *testing.T) {
	// create temp vars yaml
	dir := t.TempDir()
	varsPath := filepath.Join(dir, "vars.yaml")
	orig := map[string]interface{}{"a": 1, "b": map[string]interface{}{"x": "y"}}
	b, _ := yaml.Marshal(orig)
	if err := os.WriteFile(varsPath, b, 0600); err != nil {
		t.Fatalf("write vars file: %v", err)
	}
	hl := newTestLoader(t)
	hl.cfg.VarsFile = varsPath
	hl.cfg.Vars = map[string]interface{}{"b": map[string]interface{}{"x": "z", "k": "v"}, "c": 3}
	if err := hl.readVars(context.Background()); err != nil {
		t.Fatalf("readVars error: %v", err)
	}
	// merged expectations: b.x overridden to z, b.k added, c added, a kept
	if fmt.Sprint(hl.vars["a"]) != "1" {
		t.Fatalf("expected a=1, got %v", hl.vars["a"])
	}
	if m, ok := hl.vars["b"].(map[string]interface{}); !ok || m["x"] != "z" || m["k"] != "v" {
		t.Fatalf("unexpected b: %#v", hl.vars["b"])
	}
	if fmt.Sprint(hl.vars["c"]) != "3" {
		t.Fatalf("expected c=3, got %v", hl.vars["c"])
	}
}

func TestGetTargets_JSONAndTemplateAndNilEntries(t *testing.T) {
	plain := `{"t1": {"name":"t1","address":"1.1.1.1"}, "t2": null}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(plain))
	}))
	defer ts.Close()

	// no template
	hl := newTestLoader(t)
	hl.cfg.URL = ts.URL
	res, err := hl.getTargets()
	if err != nil {
		t.Fatalf("getTargets error: %v", err)
	}
	if res["t1"].Name != "t1" || res["t1"].Address != "1.1.1.1" {
		t.Fatalf("unexpected t1: %#v", res["t1"])
	}
	// t2 is nil in input, should be auto-filled name/address
	if res["t2"].Name != "t2" || res["t2"].Address != "t2" {
		t.Fatalf("unexpected t2: %#v", res["t2"])
	}

	// template path: server returns array of objects -> map
	arr := `[{"n":"a","a":"10.0.0.1"},{"n":"b"}]`
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(arr))
	}))
	defer ts2.Close()
	hl2 := newTestLoader(t)
	hl2.cfg.URL = ts2.URL
	// static template
	hl2.cfg.Template = `{"a":{"name":"a","address":"10.0.0.1"},"b":{"name":"b"}}`
	// initialize template via Init
	if err := hl2.Init(context.Background(), map[string]interface{}{"url": hl2.cfg.URL, "template": hl2.cfg.Template}, hl2.logger); err != nil {
		t.Fatalf("init with template: %v", err)
	}
	res2, err := hl2.getTargets()
	if err != nil {
		t.Fatalf("getTargets with template error: %v", err)
	}
	if res2["a"].Name != "a" || res2["a"].Address != "10.0.0.1" {
		t.Fatalf("unexpected a: %#v", res2["a"])
	}
	// address missing -> should default to name
	if res2["b"].Name != "b" || res2["b"].Address != "b" {
		t.Fatalf("unexpected b: %#v", res2["b"])
	}
}

func TestInitializeAction(t *testing.T) {
	hl := newTestLoader(t)
	// register a temporary action type
	orig := actions.Actions["fake"]
	actions.Actions["fake"] = func() actions.Action { return &fakeAction{} }
	defer func() { actions.Actions["fake"] = orig }()
	// success
	a, err := hl.initializeAction(map[string]interface{}{"type": "fake", "name": "x"})
	if err != nil || a == nil {
		t.Fatalf("expected success, got err=%v action=%v", err, a)
	}
	// unknown type
	a, err = hl.initializeAction(map[string]interface{}{"type": "does-not-exist"})
	if err == nil || a != nil {
		t.Fatalf("expected error for unknown type")
	}
	// missing type
	a, err = hl.initializeAction(map[string]interface{}{})
	if err == nil || a != nil {
		t.Fatalf("expected error for missing type")
	}
}

func TestRunOnAddActions_ErrorRemovesTarget(t *testing.T) {
	hl := newTestLoader(t)
	hl.addActions = []actions.Action{&fakeAction{name: "bad", fail: true}}
	hl.numActions = len(hl.addActions)
	hl.lastTargets["t1"] = &types.TargetConfig{Name: "t1"}
	ctx := context.Background()
	// should return error and remove t1 from lastTargets
	if err := hl.runOnAddActions(ctx, "t1", map[string]*types.TargetConfig{"t1": {Name: "t1"}}); err == nil {
		t.Fatal("expected error from failing action")
	}
	hl.m.RLock()
	_, exists := hl.lastTargets["t1"]
	hl.m.RUnlock()
	if exists {
		t.Fatal("expected t1 to be removed from lastTargets")
	}
}

func TestUpdateTargets_NoChange_NoOp(t *testing.T) {
	hl := newTestLoader(t)
	hl.numActions = 0
	// two identical targets in lastTargets and tcs
	t1 := &types.TargetConfig{Name: "t1", Address: "1.1.1.1"}
	t2 := &types.TargetConfig{Name: "t2", Address: "2.2.2.2"}
	hl.lastTargets["t1"] = &types.TargetConfig{Name: "t1", Address: "1.1.1.1"}
	hl.lastTargets["t2"] = &types.TargetConfig{Name: "t2", Address: "2.2.2.2"}
	called := 0
	hl.targetConfigFn = func(tc *types.TargetConfig) error { called++; return nil }
	ch := make(chan *loaders.TargetOperation, 1)
	hl.updateTargets(context.Background(), map[string]*types.TargetConfig{"t1": t1, "t2": t2}, ch)
	select {
	case op := <-ch:
		t.Fatalf("unexpected op received: %+v", op)
	default:
		// ok, no op expected
	}
	if called != 2 {
		t.Fatalf("expected targetConfigFn to be called twice, got %d", called)
	}
}

func TestUpdateTargets_Add(t *testing.T) {
	hl := newTestLoader(t)
	hl.numActions = 0
	t1 := &types.TargetConfig{Name: "t1", Address: "1.1.1.1"}
	ch := make(chan *loaders.TargetOperation, 1)
	hl.updateTargets(context.Background(), map[string]*types.TargetConfig{"t1": t1}, ch)
	select {
	case op := <-ch:
		if len(op.Add) != 1 || len(op.Del) != 0 {
			t.Fatalf("unexpected op: %+v", op)
		}
		if _, ok := hl.lastTargets["t1"]; !ok {
			t.Fatal("expected t1 to be added to lastTargets")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for op")
	}
}

func TestUpdateTargets_Delete(t *testing.T) {
	hl := newTestLoader(t)
	hl.numActions = 0
	hl.lastTargets["t1"] = &types.TargetConfig{Name: "t1", Address: "1.1.1.1"}
	ch := make(chan *loaders.TargetOperation, 1)
	hl.updateTargets(context.Background(), map[string]*types.TargetConfig{}, ch)
	select {
	case op := <-ch:
		if len(op.Add) != 0 || len(op.Del) != 1 || op.Del[0] != "t1" {
			t.Fatalf("unexpected op: %+v", op)
		}
		if _, ok := hl.lastTargets["t1"]; ok {
			t.Fatal("expected t1 to be deleted from lastTargets")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for op")
	}
}

func TestUpdateTargets_Change_TriggersDelAndAdd(t *testing.T) {
	hl := newTestLoader(t)
	hl.numActions = 0
	hl.lastTargets["t1"] = &types.TargetConfig{Name: "t1", Address: "1.1.1.1"}
	newT1 := &types.TargetConfig{Name: "t1", Address: "1.1.1.2"}
	ch := make(chan *loaders.TargetOperation, 1)
	hl.updateTargets(context.Background(), map[string]*types.TargetConfig{"t1": newT1}, ch)
	select {
	case op := <-ch:
		if len(op.Add) != 1 || len(op.Del) != 1 || op.Del[0] != "t1" {
			t.Fatalf("unexpected op: %+v", op)
		}
		if lt, ok := hl.lastTargets["t1"]; !ok || lt.Address != "1.1.1.2" {
			t.Fatalf("expected lastTargets to have updated address, got: %+v", lt)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for op")
	}
}

func TestUpdateTargets_Change_TriggersRename(t *testing.T) {
	hl := newTestLoader(t)
	hl.numActions = 0
	hl.lastTargets["t1"] = &types.TargetConfig{Name: "t1", Address: "1.1.1.1"}
	newT2 := &types.TargetConfig{Name: "t2", Address: "1.1.1.1"}
	ch := make(chan *loaders.TargetOperation, 1)
	hl.updateTargets(context.Background(), map[string]*types.TargetConfig{"t2": newT2}, ch)
	select {
	case op := <-ch:
		if len(op.Add) != 1 || len(op.Del) != 1 || op.Del[0] != "t1" {
			t.Fatalf("unexpected op: %+v", op)
		}
		if _, ok := hl.lastTargets["t2"]; !ok {
			t.Fatal("expected t2 to be added to lastTargets")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for op")
	}
}
