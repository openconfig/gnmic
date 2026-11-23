package targets_manager

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/cache"
)

func TestRemoveDropsTargetFromCache(t *testing.T) {
	tm := newTargetsTestManager(t)
	c, err := cache.New(&cache.Config{Type: "oc", Expiration: time.Hour})
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	tm.SetCache(c)

	writeLeaf := func(target, leaf, value string) {
		t.Helper()
		c.Write(context.Background(), "sub-name", &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
				Timestamp: time.Now().UnixNano(),
				Prefix:    &gnmi.Path{Target: target},
				Update: []*gnmi.Update{{
					Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}, {Name: leaf}}},
					Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: value}},
				}},
			}},
		})
	}
	writeLeaf("DUT-1", "name", "integration-test")
	writeLeaf("DUT-2", "name", "other")

	path := &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}, {Name: "name"}}}
	if got, err := c.Read("sub-name", "DUT-1", path); err != nil || len(got) == 0 {
		t.Fatalf("seeded DUT-1: n=%d err=%v", len(got), err)
	}

	tm.remove("DUT-1")

	if got, err := c.Read("sub-name", "DUT-1", path); err != nil || len(got) != 0 {
		t.Fatalf("DUT-1 still cached after remove: %+v err=%v", got, err)
	}
	if got, err := c.Read("sub-name", "DUT-2", path); err != nil || len(got) == 0 {
		t.Fatalf("DUT-2 was removed with DUT-1: n=%d err=%v", len(got), err)
	}
}
