package cluster_manager

import "testing"

func TestPickAssignee_prefersMostFreeQuota(t *testing.T) {
	members := map[string]*Member{
		"m1": {ID: "m1"},
		"m2": {ID: "m2"},
	}
	quotas := map[string]int64{"m1": 10, "m2": 10}
	load := map[string]int64{"m1": 9, "m2": 2}

	pick := pickAssignee("target-a", members, quotas, load)
	if pick == nil || pick.ID != "m2" {
		t.Fatalf("pick = %#v, want m2", pick)
	}
	if load["m2"] != 3 {
		t.Fatalf("load[m2] = %d, want 3", load["m2"])
	}
}

func TestPickAssignee_emptyMembers(t *testing.T) {
	if pick := pickAssignee("t", nil, nil, nil); pick != nil {
		t.Fatalf("pick = %#v, want nil", pick)
	}
}

func TestTieBreak_deterministic(t *testing.T) {
	a := tieBreak("target-1", "member-a", "member-b")
	b := tieBreak("target-1", "member-a", "member-b")
	if a != b {
		t.Fatalf("tieBreak not deterministic: %v vs %v", a, b)
	}
	if !tieBreak("target-1", "member-a", "") {
		t.Fatal("empty memberB should lose to memberA")
	}
}

func TestFnv64_stable(t *testing.T) {
	if fnv64("x") != fnv64("x") {
		t.Fatal("fnv64 not stable")
	}
	if fnv64("x") == fnv64("y") {
		t.Fatal("fnv64 collision on distinct inputs")
	}
}
