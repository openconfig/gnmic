// © 2026 Supranett.
//
// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

// A Lease name must be a DNS subdomain, and a label key must be a qualified name.
// The derivation replaced only "/", so an ordinary target name such as
// "Router-A" produced a Lease name containing uppercase characters, which the
// API server rejects.
func TestLeaseKeyNameIsDNS1123Subdomain(t *testing.T) {
	for _, key := range []string{
		"gnmic/test-cluster/targets/Router-A",
		"gnmic/test-cluster/targets/router_a",
		"gnmic/test-cluster/targets/router.a",
		"gnmic/test-cluster/targets/router-1",
		"gnmic/test-cluster/targets/Router.A",
	} {
		t.Run(key, func(t *testing.T) {
			name := leaseNameFor(key)
			if errs := validation.IsDNS1123Subdomain(name); len(errs) != 0 {
				t.Fatalf("Lease name %q derived from key %q is not a valid DNS subdomain: %v",
					name, key, errs)
			}
		})
	}
}

// The derived name is used as a label key too, so it must stay a legal
// qualified name as well.
func TestLeaseKeyNameIsValidLabelKey(t *testing.T) {
	for _, key := range []string{
		"gnmic/test-cluster/targets/Router-A",
		"gnmic/test-cluster/targets/router_a",
		"gnmic/test-cluster/targets/Router.A",
	} {
		t.Run(key, func(t *testing.T) {
			name := leaseNameFor(key)
			if errs := validation.IsQualifiedName(name); len(errs) != 0 {
				t.Fatalf("label key %q derived from key %q is not a valid qualified name: %v",
					name, key, errs)
			}
		})
	}
}

// The derivation must be deterministic, so the same key always maps to the same
// Lease across processes.
func TestLeaseNameIsDeterministic(t *testing.T) {
	key := "gnmic/test-cluster/targets/Router-A"
	first := leaseNameFor(key)
	for i := 0; i < 5; i++ {
		if got := leaseNameFor(key); got != first {
			t.Fatalf("leaseNameFor is not deterministic: %q then %q", first, got)
		}
	}
}

// Distinct keys must not collapse onto one Lease name, otherwise two targets
// would share a lock.
func TestLeaseNameDistinguishesDistinctKeys(t *testing.T) {
	seen := make(map[string]string)
	for _, key := range []string{
		"gnmic/c/targets/Router-A",
		"gnmic/c/targets/Router.B",
		"gnmic/c/targets/router-a",
		"gnmic/c/targets/router_A",
	} {
		name := leaseNameFor(key)
		if prev, ok := seen[name]; ok {
			t.Fatalf("keys %q and %q both derive the Lease name %q", prev, key, name)
		}
		seen[name] = key
	}
}

// The name must stay within the 253 character limit for a DNS subdomain even
// for a long cluster name.
func TestLeaseNameLengthWithinLimit(t *testing.T) {
	key := "gnmic/" + strings.Repeat("long-cluster-name-", 12) + "/targets/t1"
	name := leaseNameFor(key)
	if len(name) > validation.DNS1123SubdomainMaxLength {
		t.Fatalf("Lease name is %d characters, over the %d limit", len(name),
			validation.DNS1123SubdomainMaxLength)
	}
}

// List() matches label keys by prefix, and the label key is the sanitised name
// plus a digest suffix. The sanitised prefix must still match, otherwise
// GET /api/v1/cluster stops reporting locked targets.
func TestListPrefixMatchesDerivedLabelKey(t *testing.T) {
	const prefix = "gnmic/test-cluster/targets"
	for _, key := range []string{
		prefix + "/Router-A",
		prefix + "/router_a",
		prefix + "/router.a",
	} {
		t.Run(key, func(t *testing.T) {
			labelKey := leaseNameFor(key)
			sanitisedPrefix := strings.Trim(
				invalidDNS1123Chars.ReplaceAllString(strings.ToLower(prefix), "-"), "-.")
			if !strings.HasPrefix(labelKey, sanitisedPrefix) {
				t.Fatalf("label key %q does not start with the sanitised prefix %q, "+
					"so List would not report this locked target", labelKey, sanitisedPrefix)
			}
		})
	}
}
