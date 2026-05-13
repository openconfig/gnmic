// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/spf13/cobra"
	zstore "github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func TestCollector_getLocker(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		setup      func(cfg zstore.Store[any]) error
		wantErrSub string
	}{
		{
			name: "no clustering key",
			setup: func(_ zstore.Store[any]) error {
				return nil
			},
		},
		{
			name: "malformed clustering type",
			setup: func(cfg zstore.Store[any]) error {
				_, err := cfg.Set("clustering", "clustering", "not-a-struct")
				return err
			},
			wantErrSub: "malformed clustering config",
		},
		{
			name: "nil clustering pointer",
			setup: func(cfg zstore.Store[any]) error {
				var cl *config.Clustering
				_, err := cfg.Set("clustering", "clustering", cl)
				return err
			},
		},
		{
			name: "clustering missing locker type",
			setup: func(cfg zstore.Store[any]) error {
				cl := &config.Clustering{
					Locker: map[string]any{"address": "127.0.0.1:8500"},
				}
				_, err := cfg.Set("clustering", "clustering", cl)
				return err
			},
			wantErrSub: "missing locker type field",
		},
		{
			name: "unknown locker type",
			setup: func(cfg zstore.Store[any]) error {
				cl := &config.Clustering{
					Locker: map[string]any{"type": "no-such-locker"},
				}
				_, err := cfg.Set("clustering", "clustering", cl)
				return err
			},
			wantErrSub: `unknown locker type "no-such-locker"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := gomap.NewMemStore(zstore.StoreOptions[any]{})
			if err := tt.setup(cfg); err != nil {
				t.Fatalf("setup: %v", err)
			}
			c := New(ctx, cfg)
			c.logger = logging.DiscardLogger()
			err := c.getLocker()
			if tt.wantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("getLocker() err = %v, want substring %q", err, tt.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("getLocker: %v", err)
			}
			if c.locker != nil {
				t.Fatal("expected nil locker")
			}
		})
	}
}

func TestCollector_CollectorPreRunE(t *testing.T) {
	cfg := gomap.NewMemStore(zstore.StoreOptions[any]{})
	c := New(context.Background(), cfg)
	c.logger = logging.DiscardLogger()
	cmd := &cobra.Command{}
	c.InitCollectorFlags(cmd)
	if err := c.CollectorPreRunE(cmd, []string{"extra"}); err == nil {
		t.Fatal("expected error for unexpected args")
	}
	if err := c.CollectorPreRunE(cmd, nil); err != nil {
		t.Fatalf("PreRunE without pyroscope: %v", err)
	}
}

func TestNewCollectorWiresManagers(t *testing.T) {
	cfg := gomap.NewMemStore(zstore.StoreOptions[any]{})
	c := New(context.Background(), cfg)
	if c == nil || c.store == nil || c.apiServer == nil || c.clusterManager == nil ||
		c.targetsManager == nil || c.outputsManager == nil || c.inputsManager == nil || c.pipeline == nil {
		t.Fatal("New() left required fields nil")
	}
}
