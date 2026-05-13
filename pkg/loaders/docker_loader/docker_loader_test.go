// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package docker_loader

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/logging"
)

func newLoader() *dockerLoader {
	return &dockerLoader{
		cfg:         new(cfg),
		wg:          new(sync.WaitGroup),
		m:           new(sync.Mutex),
		lastTargets: make(map[string]*types.TargetConfig),
		logger:      logging.DiscardLogger(),
	}
}

func TestDockerLoader_SetDefaults(t *testing.T) {
	d := newLoader()
	d.setDefaults()
	if d.cfg.Interval != watchInterval {
		t.Errorf("interval=%v", d.cfg.Interval)
	}
	if d.cfg.Timeout != watchInterval/2 {
		t.Errorf("timeout=%v", d.cfg.Timeout)
	}
	if len(d.cfg.Filters) != 1 {
		t.Errorf("default filters: %+v", d.cfg.Filters)
	}

	// custom values
	d2 := newLoader()
	d2.cfg.Interval = time.Minute
	d2.cfg.Timeout = 10 * time.Second
	d2.setDefaults()
	if d2.cfg.Timeout != 10*time.Second {
		t.Errorf("timeout overridden: %v", d2.cfg.Timeout)
	}
	// timeout >= interval
	d3 := newLoader()
	d3.cfg.Interval = time.Second
	d3.cfg.Timeout = 2 * time.Second
	d3.setDefaults()
	if d3.cfg.Timeout != 500*time.Millisecond {
		t.Errorf("timeout=%v", d3.cfg.Timeout)
	}
}

func TestDockerLoader_GetPortNumber(t *testing.T) {
	cases := []struct {
		name   string
		labels map[string]string
		p      string
		want   uint16
	}{
		{"empty", nil, "", 0},
		{"numeric", nil, "1234", 1234},
		{"label", map[string]string{"port": "5678"}, "label=port", 5678},
		{"label-missing", map[string]string{}, "label=port", 0},
		{"non-numeric", nil, "abc", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getPortNumber(tc.labels, tc.p); got != tc.want {
				t.Errorf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestDockerLoader_String(t *testing.T) {
	d := newLoader()
	d.cfg.Address = "tcp://localhost:2375"
	if got := d.String(); got == "" {
		t.Errorf("String empty")
	}
}

func TestDockerLoader_CreateDockerClient(t *testing.T) {
	d := newLoader()
	d.cfg.Timeout = time.Second
	// unix path - default if address empty (FromEnv)
	c, err := d.createDockerClient()
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer c.Close()

	// custom address
	d2 := newLoader()
	d2.cfg.Address = "tcp://127.0.0.1:9999"
	d2.cfg.Timeout = time.Second
	c2, err := d2.createDockerClient()
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer c2.Close()
}

func TestDockerLoader_ReadVars(t *testing.T) {
	d := newLoader()
	// no vars-file: copy Vars
	d.cfg.Vars = map[string]any{"a": 1}
	if err := d.readVars(context.Background()); err != nil {
		t.Fatalf("readVars: %v", err)
	}
	if d.vars["a"] != 1 {
		t.Errorf("vars=%v", d.vars)
	}

	// vars-file present
	dir := t.TempDir()
	p := filepath.Join(dir, "vars.yaml")
	if err := os.WriteFile(p, []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d2 := newLoader()
	d2.cfg.VarsFile = p
	d2.cfg.Vars = map[string]any{"baz": "qux"}
	if err := d2.readVars(context.Background()); err != nil {
		t.Fatalf("readVars: %v", err)
	}
	if d2.vars["foo"] != "bar" || d2.vars["baz"] != "qux" {
		t.Errorf("vars=%v", d2.vars)
	}

	// missing vars-file
	d3 := newLoader()
	d3.cfg.VarsFile = "/no/such/file"
	if err := d3.readVars(context.Background()); err == nil {
		t.Errorf("expected read error")
	}
}

func TestDockerLoader_InitDecodeError(t *testing.T) {
	d := newLoader()
	if err := d.Init(context.Background(), map[string]any{"interval": "x"}, nil); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestDockerLoader_InitializeAction(t *testing.T) {
	d := newLoader()
	if _, err := d.initializeAction(nil); err == nil {
		t.Errorf("expected missing definition err")
	}
	if _, err := d.initializeAction(map[string]any{"foo": "bar"}); err == nil {
		t.Errorf("expected missing type err")
	}
	if _, err := d.initializeAction(map[string]any{"type": 123}); err == nil {
		t.Errorf("expected unexpected type err")
	}
	if _, err := d.initializeAction(map[string]any{"type": "unknown-action"}); err == nil {
		t.Errorf("expected unknown action err")
	}
}

func TestDockerLoader_DiffEmpty(t *testing.T) {
	d := newLoader()
	d.lastTargets = map[string]*types.TargetConfig{
		"old": {Name: "old"},
	}
	now := map[string]*types.TargetConfig{
		"new": {Name: "new"},
	}
	res := d.diff(now)
	if len(res.Add) != 1 || len(res.Del) != 1 {
		t.Errorf("diff=%+v", res)
	}
	if _, ok := d.lastTargets["new"]; !ok {
		t.Errorf("new not stored")
	}
	if _, ok := d.lastTargets["old"]; ok {
		t.Errorf("old not deleted")
	}
}

func TestDockerLoader_RegisterMetricsDisabled(t *testing.T) {
	d := newLoader()
	d.RegisterMetrics(nil) // disabled, no-op
	d.cfg.EnableMetrics = true
	d.RegisterMetrics(nil) // enabled but nil registry, logs warning
}

func TestDockerLoader_WithActionsAndDefaults(t *testing.T) {
	d := newLoader()
	d.WithActions(map[string]map[string]any{"a1": {"type": "x"}})
	if len(d.actionsConfig) != 1 {
		t.Errorf("actions: %v", d.actionsConfig)
	}
	called := false
	d.WithTargetsDefaults(func(*types.TargetConfig) error {
		called = true
		return nil
	})
	if d.targetConfigFn == nil {
		t.Fatalf("nil")
	}
	_ = d.targetConfigFn(&types.TargetConfig{Name: "n"})
	if !called {
		t.Errorf("not called")
	}
}
