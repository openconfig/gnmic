// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"testing"

	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func TestNewStore(t *testing.T) {
	cfg := gomap.NewMemStore(store.StoreOptions[any]{})
	s := collstore.NewStore(cfg)
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
	if s.Config != cfg {
		t.Fatal("Config store not wired")
	}
	if s.State == nil {
		t.Fatal("expected in-memory state store")
	}
}
