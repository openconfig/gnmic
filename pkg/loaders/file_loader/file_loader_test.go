// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package file_loader

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/loaders"
	"github.com/openconfig/gnmic/pkg/logging"
)

func newLoader() *fileLoader {
	return &fileLoader{
		cfg:         &cfg{},
		m:           new(sync.RWMutex),
		lastTargets: make(map[string]*types.TargetConfig),
		logger:      logging.DiscardLogger(),
	}
}

func writeTargets(t *testing.T, dir, name string, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestFileLoader_InitErrors(t *testing.T) {
	f := newLoader()
	if err := f.Init(context.Background(), map[string]any{}, nil); err == nil {
		t.Errorf("expected missing path error")
	}
	// decode error
	f = newLoader()
	if err := f.Init(context.Background(), map[string]any{"interval": "x"}, nil); err == nil {
		t.Errorf("expected decode error")
	}
	// bad template
	f = newLoader()
	err := f.Init(context.Background(), map[string]any{
		"path":     "/tmp/x",
		"template": "{{ ", // bad template
	}, nil)
	if err == nil {
		t.Errorf("expected template error")
	}
	// vars-file missing
	f = newLoader()
	err = f.Init(context.Background(), map[string]any{
		"path":      "/tmp/x",
		"vars-file": "/no/such/file/here",
	}, nil)
	if err == nil {
		t.Errorf("expected vars-file error")
	}
	// unknown OnAdd action
	f = newLoader()
	err = f.Init(context.Background(), map[string]any{
		"path":   "/tmp/x",
		"on-add": []any{"missing"},
	}, nil)
	if err == nil {
		t.Errorf("expected unknown action error")
	}
}

func TestFileLoader_InitMinimal(t *testing.T) {
	f := newLoader()
	err := f.Init(context.Background(), map[string]any{
		"path":     "/tmp/x",
		"interval": "1s",
	}, nil)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if f.cfg.Interval != time.Second {
		t.Errorf("interval = %v", f.cfg.Interval)
	}
	if got := f.String(); got == "" {
		t.Errorf("String empty")
	}
}

func TestFileLoader_GetTargets(t *testing.T) {
	dir := t.TempDir()
	body := `
target1:
  username: admin
  password: pass
target2:
`
	p := writeTargets(t, dir, "targets.yaml", body)
	f := newLoader()
	f.cfg.Path = p
	f.cfg.Interval = time.Second
	got, err := f.getTargets(context.Background())
	if err != nil {
		t.Fatalf("getTargets: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d targets", len(got))
	}
	if got["target1"].Name != "target1" {
		t.Errorf("target1 name: %+v", got["target1"])
	}
	if got["target2"] == nil || got["target2"].Address != "target2" {
		t.Errorf("target2 default addr: %+v", got["target2"])
	}
}

func TestFileLoader_GetTargetsErrors(t *testing.T) {
	f := newLoader()
	f.cfg.Path = "/no/such/file"
	f.cfg.Interval = time.Second
	if _, err := f.getTargets(context.Background()); err == nil {
		t.Errorf("expected read error")
	}
	// invalid yaml
	dir := t.TempDir()
	p := writeTargets(t, dir, "bad.yaml", "::: not yaml :::")
	f.cfg.Path = p
	if _, err := f.getTargets(context.Background()); err == nil {
		t.Errorf("expected unmarshal error")
	}
}

func TestFileLoader_RunOnce(t *testing.T) {
	dir := t.TempDir()
	p := writeTargets(t, dir, "t.yaml", "t1:\n")
	f := newLoader()
	f.cfg.Path = p
	f.cfg.Interval = time.Second
	f.cfg.Debug = true
	got, err := f.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d targets", len(got))
	}
}

func TestFileLoader_StartProducesOps(t *testing.T) {
	dir := t.TempDir()
	p := writeTargets(t, dir, "t.yaml", "t1:\n")
	f := newLoader()
	f.cfg.Path = p
	f.cfg.Interval = 50 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	ch := f.Start(ctx)
	var got *loaders.TargetOperation
	select {
	case got = <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no operation received")
	}
	if got == nil || len(got.Add) != 1 {
		t.Errorf("op: %+v", got)
	}
}

func TestFileLoader_InitializeAction(t *testing.T) {
	f := newLoader()
	if _, err := f.initializeAction(nil); err == nil {
		t.Errorf("expected missing definition err")
	}
	if _, err := f.initializeAction(map[string]any{"foo": "bar"}); err == nil {
		t.Errorf("expected missing type err")
	}
	if _, err := f.initializeAction(map[string]any{"type": 123}); err == nil {
		t.Errorf("expected unexpected type err")
	}
	if _, err := f.initializeAction(map[string]any{"type": "unknown-action-type"}); err == nil {
		t.Errorf("expected unknown action type err")
	}
}

func TestFileLoader_ReadVarsFromFile(t *testing.T) {
	dir := t.TempDir()
	p := writeTargets(t, dir, "vars.yaml", "foo: bar\n")
	f := newLoader()
	f.cfg.VarsFile = p
	f.cfg.Vars = map[string]any{"baz": "qux"}
	if err := f.readVars(context.Background()); err != nil {
		t.Fatalf("readVars: %v", err)
	}
	if f.vars["foo"] != "bar" || f.vars["baz"] != "qux" {
		t.Errorf("vars=%+v", f.vars)
	}
}

func TestFileLoader_RegisterMetricsDisabled(t *testing.T) {
	f := newLoader()
	// Should not panic even with nil registry when disabled
	f.RegisterMetrics(nil)
	f.cfg.EnableMetrics = true
	f.RegisterMetrics(nil) // should log, not panic
}

func TestFileLoader_WithActionsAndDefaults(t *testing.T) {
	f := newLoader()
	f.WithActions(map[string]map[string]any{"a1": {"type": "x"}})
	if len(f.actionsConfig) != 1 {
		t.Errorf("WithActions: %v", f.actionsConfig)
	}
	called := false
	f.WithTargetsDefaults(func(tc *types.TargetConfig) error {
		called = true
		return nil
	})
	if f.targetConfigFn == nil {
		t.Fatalf("targetConfigFn nil")
	}
	_ = f.targetConfigFn(&types.TargetConfig{Name: "n"})
	if !called {
		t.Errorf("fn not called")
	}
}
