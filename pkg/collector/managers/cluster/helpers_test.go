// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package cluster_manager

import "testing"

func TestTargetsLockPrefix(t *testing.T) {
	if got := targetsLockPrefix("c1"); got != "gnmic/c1/targets" {
		t.Fatalf("targetsLockPrefix: %q", got)
	}
}

func TestTargetLockKey(t *testing.T) {
	if got := targetLockKey("t1", "c1"); got != "gnmic/c1/targets/t1" {
		t.Fatalf("targetLockKey: %q", got)
	}
}

func TestGetAPIScheme(t *testing.T) {
	tests := []struct {
		name   string
		member *Member
		want   string
	}{
		{name: "nil member", member: nil, want: "http"},
		{name: "no labels", member: &Member{Labels: nil}, want: "http"},
		{name: "https label", member: &Member{Labels: []string{"__protocol=https"}}, want: "https"},
		{name: "http label explicit", member: &Member{Labels: []string{"__protocol=http"}}, want: "http"},
		{name: "other labels only", member: &Member{Labels: []string{"env=prod"}}, want: "http"},
		{name: "protocol label without equals", member: &Member{Labels: []string{"__protocol"}}, want: "http"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAPIScheme(tt.member); got != tt.want {
				t.Fatalf("GetAPIScheme() = %q, want %q", got, tt.want)
			}
		})
	}
}
