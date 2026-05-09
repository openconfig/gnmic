// SPDX-License-Identifier: Apache-2.0

package types

import "testing"

func TestTargetConfig_RedactedDeepCopy(t *testing.T) {
	orig := &TargetConfig{
		Name:     "t1",
		Address:  "127.0.0.1:57400",
		Username: strPtr("admin"),
		Password: strPtr("secret"),
		Token:    strPtr("oauth-token"),
	}
	red := orig.RedactedDeepCopy()
	if red == nil {
		t.Fatal("RedactedDeepCopy returned nil")
	}
	if red.Name != orig.Name || red.Address != orig.Address {
		t.Fatalf("unexpected copy: %+v", red)
	}
	if red.Username == nil || *red.Username != "admin" {
		t.Fatalf("username: got %+v", red.Username)
	}
	if red.Password == nil || *red.Password != "****" {
		t.Fatalf("password: got %+v", red.Password)
	}
	if red.Token == nil || *red.Token != "****" {
		t.Fatalf("token: got %+v", red.Token)
	}
	if orig.Password == nil || *orig.Password != "secret" {
		t.Fatal("original password mutated")
	}
	if orig.Token == nil || *orig.Token != "oauth-token" {
		t.Fatal("original token mutated")
	}
}

func TestTargetConfig_RedactedDeepCopy_nil(t *testing.T) {
	if (*TargetConfig)(nil).RedactedDeepCopy() != nil {
		t.Fatal("expected nil")
	}
}

func strPtr(s string) *string { return &s }
