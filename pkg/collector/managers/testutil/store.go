package testutil

import (
	"testing"

	"github.com/openconfig/gnmic/pkg/config"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func NewTestStore(t *testing.T) *collstore.Store {
	t.Helper()
	cfgStore := gomap.NewMemStore[any](store.StoreOptions[any]{})
	t.Cleanup(func() { _ = cfgStore.Close() })
	st := collstore.NewStore(cfgStore)
	SeedGlobalFlags(t, st)
	return st
}

func SeedGlobalFlags(t *testing.T, st *collstore.Store) {
	t.Helper()
	if _, err := st.Config.Set("global-flags", "global-flags", config.GlobalFlags{Encoding: "json"}); err != nil {
		t.Fatalf("seed global-flags: %v", err)
	}
}
