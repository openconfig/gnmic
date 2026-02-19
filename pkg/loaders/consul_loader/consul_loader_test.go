// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package consul_loader

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/openconfig/gnmic/pkg/api/utils"
)

// Test the specific bug scenario described in issue #706
// This test reproduces the exact problem: services with extra metadata tags
// were being silently filtered out by the old logic
func TestIssue706_ServicesWithExtraTagsFiltered(t *testing.T) {
	cl := &consulLoader{
		logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		cfg: &cfg{
			Services: []*serviceDef{
				{
					Name: "test-service",
					Tags: []string{"gnmic", "network-device"},
					tags: map[string]struct{}{
						"gnmic":          {},
						"network-device": {},
					},
					Config: map[string]interface{}{
						"name": "test-target",
					},
				},
			},
		},
	}

	err := cl.Init(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Expected Init to succeed, but got error: %v", err)
	}
	// Service with extra metadata tags - this should NOT be filtered out
	serviceEntry := &api.ServiceEntry{
		Service: &api.AgentService{
			ID:      "test-service-1",
			Service: "test-service",
			Tags:    []string{"gnmic", "network-device", "vendor:arista", "environment:production"},
			Address: "192.168.1.100",
			Port:    57400,
		},
		Node: &api.Node{
			Address: "192.168.1.100",
		},
	}

	result, err := cl.serviceEntryToTargetConfig(cl.cfg.Services[0], serviceEntry)

	if err != nil {
		t.Fatalf("Expected service with extra tags to be accepted, but got error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected service with extra tags to be accepted, but got nil result")
	}

	if result.Name != "test-target" {
		t.Errorf("Expected target name 'test-target', got: %s", result.Name)
	}

	if result.Address != "192.168.1.100:57400" {
		t.Errorf("Expected address '192.168.1.100:57400', got: %s", result.Address)
	}
}

// Test case that would demonstrate the old buggy behavior
// This test explicitly documents what the old code was doing wrong
func TestOldBuggyLogicWouldReject(t *testing.T) {
	// Simulate what the OLD buggy logic was doing:
	// for _, t := range se.Service.Tags {
	//     if _, ok := sd.tags[t]; !ok {
	//         goto SRV  // Reject service because of extra tag
	//     }
	// }

	requiredTags := map[string]struct{}{
		"gnmic":          {},
		"network-device": {},
	}

	serviceTags := []string{"gnmic", "network-device", "vendor:arista", "environment:production"}

	// This is what the OLD code was doing (buggy logic)
	oldLogicWouldReject := false
	for _, serviceTag := range serviceTags {
		if _, ok := requiredTags[serviceTag]; !ok {
			oldLogicWouldReject = true
			break
		}
	}

	// The old logic would incorrectly reject this service
	if !oldLogicWouldReject {
		t.Error("This test is invalid - the old buggy logic should have rejected this service")
	}

	// But the NEW logic should accept it (all required tags are present)
	newLogicShouldAccept := true
	for requiredTag := range requiredTags {
		found := false
		for _, serviceTag := range serviceTags {
			if serviceTag == requiredTag {
				found = true
				break
			}
		}
		if !found {
			newLogicShouldAccept = false
			break
		}
	}

	if !newLogicShouldAccept {
		t.Error("The new logic should accept this service since all required tags are present")
	}

	t.Logf("✓ Old logic would incorrectly reject: %v", oldLogicWouldReject)
	t.Logf("✓ New logic correctly accepts: %v", newLogicShouldAccept)
}

func TestRunOnceAppliesServiceFilter(t *testing.T) {
	filterExpr := `Service.Meta.profile == "arista"`
	var filterChecked bool
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/agent/self":
			fmt.Fprint(w, `{"Member":{"Tags":{}}}`)
		case strings.HasPrefix(r.URL.Path, "/v1/health/service/gnmi"):
			if got := r.URL.Query().Get("filter"); got != filterExpr {
				t.Fatalf("expected filter %q, got %q", filterExpr, got)
			}
			filterChecked = true
			fmt.Fprint(w, `[{"Node":{"Address":"10.0.0.1"},"Service":{"ID":"target-1","Service":"gnmi","Address":"10.0.0.1","Port":6030}}]`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer hs.Close()
	addr := strings.TrimPrefix(hs.URL, "http://")
	cl := &consulLoader{
		cfg: &cfg{
			Address:    addr,
			Datacenter: "dc1",
			Services: []*serviceDef{
				{Name: "gnmi", Filter: filterExpr},
			},
		},
		logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		m:      new(sync.Mutex),
	}
	res, err := cl.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if !filterChecked {
		t.Fatalf("expected health query to include filter parameter")
	}
	if _, ok := res["target-1"]; !ok {
		t.Fatalf("expected target-1 in results, got %v", res)
	}
}
