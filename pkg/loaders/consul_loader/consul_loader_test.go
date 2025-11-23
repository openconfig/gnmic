// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package consul_loader

import (
	"bytes"
	"context"
	"io"
	"log"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/loaders"
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

	result, err := cl.serviceEntryToTargetConfig(serviceEntry)

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

// TestDuplicateServiceDefinitions ensures duplicate entries log an error and keep the first definition.
func TestDuplicateServiceDefinitions(t *testing.T) {
	logBuf := new(bytes.Buffer)
	cl := &consulLoader{
		logger: log.New(logBuf, loggingPrefix, utils.DefaultLoggingFlags),
		cfg: &cfg{
			Services: []*serviceDef{
				{
					Name: "gnmi",
					Tags: []string{"profile=a"},
					tags: map[string]struct{}{
						"profile=a": {},
					},
				},
				{
					Name: "gnmi",
					Tags: []string{"profile=a"},
					tags: map[string]struct{}{
						"profile=a": {},
					},
				},
			},
		},
	}

	err := cl.Init(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if got := len(cl.cfg.Services); got != 1 {
		t.Fatalf("expected 1 unique service definition, got %d", got)
	}
	if cl.cfg.Services[0].id == "" {
		t.Fatal("expected service identifier to be set")
	}
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "duplicate consul service definition ignored") {
		t.Fatalf("expected duplicate warning log, got %q", logBuf.String())
	}
}

// TestUpdateTargetsUsesServiceID verifies that targets from different tag filters sharing the same service name do not collide.
func TestUpdateTargetsUsesServiceID(t *testing.T) {
	cl := &consulLoader{
		logger:      log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		lastTargets: make(map[string]map[string]*types.TargetConfig),
		m:           new(sync.Mutex),
		cfg: &cfg{
			Services: []*serviceDef{
				{
					Name: "gnmi",
					Tags: []string{"profile=a"},
					tags: map[string]struct{}{
						"profile=a": {},
					},
				},
				{
					Name: "gnmi",
					Tags: []string{"profile=b"},
					tags: map[string]struct{}{
						"profile=b": {},
					},
				},
			},
		},
	}

	if err := cl.Init(context.Background(), nil, nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	ctx := context.Background()
	opChan := make(chan *loaders.TargetOperation, 2)

	serviceIDA := cl.cfg.Services[0].id
	serviceIDB := cl.cfg.Services[1].id

	targetA := &types.TargetConfig{Name: "target-a"}
	targetB := &types.TargetConfig{Name: "target-b"}

	cl.updateTargets(ctx, cl.cfg.Services[0].Name, serviceIDA, map[string]*types.TargetConfig{targetA.Name: targetA}, opChan)
	op := <-opChan
	if len(op.Add) != 1 {
		t.Fatalf("expected 1 addition for service %s, got %d", serviceIDA, len(op.Add))
	}

	cl.updateTargets(ctx, cl.cfg.Services[1].Name, serviceIDB, map[string]*types.TargetConfig{targetB.Name: targetB}, opChan)
	op = <-opChan
	if len(op.Add) != 1 {
		t.Fatalf("expected 1 addition for service %s, got %d", serviceIDB, len(op.Add))
	}

	if len(cl.lastTargets) != 2 {
		t.Fatalf("expected lastTargets to keep 2 services, got %d", len(cl.lastTargets))
	}
	if _, ok := cl.lastTargets[serviceIDA][targetA.Name]; !ok {
		t.Fatalf("service %s missing target %s", serviceIDA, targetA.Name)
	}
	if _, ok := cl.lastTargets[serviceIDB][targetB.Name]; !ok {
		t.Fatalf("service %s missing target %s", serviceIDB, targetB.Name)
	}
}
